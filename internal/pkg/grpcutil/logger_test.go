// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpcutil_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

func TestPayloadUnaryServerInterceptor(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	defer runNoErr(listener.Close, "failed to close listener")

	var dst bytes.Buffer
	logger := newLogger(&dst)

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpc_ctxtags.UnaryServerInterceptor(),
		grpc_zap.UnaryServerInterceptor(logger,
			grpc_zap.WithTimestampFormat(""),
			grpc_zap.WithDurationField(func(_ time.Duration) zap.Field {
				return zap.Float64("grpc.duration", 1)
			}),
		),
		grpcutil.SetUserAgent(),
		grpcutil.SetRealPeerAddress(),
		grpcutil.InterceptBodyToTags(grpcutil.NewHook(
			grpcutil.NewRewriter(func(req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleRequest, bool) {
				req.Payload.Body = nil

				return req, true
			}),
			grpcutil.NewRewriter(func(res *grpc_testing.SimpleResponse) (*grpc_testing.SimpleResponse, bool) {
				res.Username = "REDACTED"

				return res, true
			}),
		), 1024),
	))

	grpc_testing.RegisterTestServiceServer(server, &testService{})

	errCh := make(chan error, 1)

	go func() {
		errCh <- server.Serve(listener)
	}()

	dial, err := grpc.Dial(listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		panic(err)
	}

	defer runNoErr(dial.Close, "failed to close dial")

	client := grpc_testing.NewTestServiceClient(dial)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("grpcgateway-user-agent", "test", "x-forwarded-for", "10.10.10.10"))

	res, err := client.UnaryCall(ctx, &grpc_testing.SimpleRequest{
		Payload: &grpc_testing.Payload{
			Body: []byte("Hello World"),
		},
		FillUsername:   true,
		FillOauthScope: true,
	})
	if err != nil {
		panic(err)
	}

	if res.Username != "MyUserName" {
		panic("username should be MyUserName")
	}

	server.Stop()

	if err := <-errCh; err != nil {
		panic(err)
	}

	var actualOutput map[string]any

	require.NoError(t, json.Unmarshal(dst.Bytes(), &actualOutput))
	require.Equal(t, expectedOutput, actualOutput)
}

func newLogger(dst io.Writer) *zap.Logger {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(dst), zap.DebugLevel)

	return zap.New(core)
}

func runNoErr(c func() error, s string) {
	if err := c(); err != nil {
		if errors.Is(err, net.ErrClosed) {
			return
		}

		panic(fmt.Errorf("%s: %w", s, err))
	}
}

type testService struct {
	grpc_testing.UnimplementedTestServiceServer
}

func (t *testService) UnaryCall(_ context.Context, req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
	if req.Payload.Body == nil || string(req.Payload.Body) != "Hello World" {
		return nil, status.Errorf(codes.InvalidArgument, "payload body is incorrect")
	}

	return &grpc_testing.SimpleResponse{
		Payload:    nil,
		Username:   "MyUserName",
		OauthScope: "Scope",
	}, nil
}

//nolint:lll
var expectedOutput = func() map[string]any {
	const line = `{"level":"info","msg":"finished unary call with code OK","grpc.start_time":"","system":"grpc","span.kind":"server","grpc.service":"grpc.testing.TestService","grpc.method":"UnaryCall","grpc.request.content":{"msg":{"payload":{},"fillUsername":true,"fillOauthScope":true}},"grpc.response.content":{"msg":{"username":"REDACTED","oauthScope":"Scope"}},"peer.address":"10.10.10.10","user_agent":"test","grpc.code":"OK","grpc.duration":1}`

	dst := map[string]any{}
	if err := json.Unmarshal([]byte(line), &dst); err != nil {
		panic(err)
	}

	return dst
}()
