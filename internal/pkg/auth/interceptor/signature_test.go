// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/plain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/interceptor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

type testServer struct {
	resources.UnimplementedResourceServiceServer
}

type registerServer func(server *grpc.Server)

func (s testServer) Get(context.Context, *resources.GetRequest) (*resources.GetResponse, error) {
	return &resources.GetResponse{}, nil
}

func initServer(ctx context.Context, t *testing.T, services []registerServer, opts ...grpc.ServerOption) string {
	eg, ctx := errgroup.WithContext(ctx)

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "localhost:0")

	require.NoError(t, err)

	server := grpc.NewServer(opts...)

	t.Cleanup(func() {
		server.Stop()

		listener.Close() //nolint:errcheck

		require.NoError(t, eg.Wait())
	})

	addr, ok := listener.Addr().(*net.TCPAddr)

	require.True(t, ok)

	for _, s := range services {
		s(server)
	}

	eg.Go(func() error {
		return server.Serve(listener)
	})

	return fmt.Sprintf("localhost:%d", addr.Port)
}

func setup(ctx context.Context, t *testing.T, key message.SignatureVerifier) resources.ResourceServiceClient {
	authenticatorFunc := func(context.Context, string) (*auth.Authenticator, error) {
		return &auth.Authenticator{
			Verifier: key,
			Identity: "user@example.com",
			UserID:   "user-id",
			Role:     role.Operator,
		}, nil
	}

	logger := zaptest.NewLogger(t)

	authConfigInterceptor := interceptor.NewAuthConfig(true, logger)

	signatureInterceptor := interceptor.NewSignature(authenticatorFunc, logger)

	target := initServer(
		ctx,
		t,
		[]registerServer{
			func(server *grpc.Server) {
				resources.RegisterResourceServiceServer(server, testServer{})
			},
		},
		grpc.ChainUnaryInterceptor(
			authConfigInterceptor.Unary(),
			signatureInterceptor.Unary(),
		),
		grpc.ChainStreamInterceptor(
			authConfigInterceptor.Stream(),
			signatureInterceptor.Stream(),
		),
	)

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	clientConn, err := grpc.NewClient(target, dialOptions...)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, clientConn.Close())
	})

	return resources.NewResourceServiceClient(clientConn)
}

func TestPGP(t *testing.T) {
	key, err := pgp.GenerateKey("", "", "test@example.org", time.Minute)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)

	t.Cleanup(cancel)

	client := setup(ctx, t, key)

	t.Run("missing signature passthrough", func(t *testing.T) {
		_, err = client.Get(ctx, &resources.GetRequest{
			Type: authres.AuthConfigType,
		})

		require.NoError(t, err)
	})

	t.Run("invalid signature version", func(t *testing.T) {
		getCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(
			message.SignatureHeaderKey, "invalid",
		))

		_, err := client.Get(getCtx, &resources.GetRequest{})

		assert.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err), "error code should be codes.Unauthenticated")
		assert.ErrorContains(t, err, "invalid signature")
	})

	t.Run("missing timestamp", func(t *testing.T) {
		payload := base64.StdEncoding.EncodeToString([]byte("payload"))

		getCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(
			message.SignatureHeaderKey, fmt.Sprintf("%s test@example.org signer-1 %s", message.SignatureVersionV1, payload),
		))

		_, err := client.Get(getCtx, &resources.GetRequest{})

		assert.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err), "error code should be codes.Unauthenticated")
		assert.ErrorContains(t, err, "invalid signature")
	})

	t.Run("valid signature", func(t *testing.T) {
		epochTimestamp := strconv.FormatInt(time.Now().Unix(), 10)

		payload := message.GRPCPayload{
			Headers: map[string][]string{
				message.TimestampHeaderKey: {epochTimestamp},
			},
			Method: resources.ResourceService_Get_FullMethodName,
		}

		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		signature, err := key.Sign(payloadJSON)
		require.NoError(t, err)

		signatureBase64 := base64.StdEncoding.EncodeToString(signature)

		getCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(
			message.SignatureHeaderKey, fmt.Sprintf(
				"%s test@example.org %s %s",
				message.SignatureVersionV1,
				key.Fingerprint(),
				signatureBase64,
			),
			message.TimestampHeaderKey, epochTimestamp,
			message.PayloadHeaderKey, string(payloadJSON),
		))

		_, err = client.Get(getCtx, &resources.GetRequest{})

		assert.NoError(t, err)
	})
}

func encodeRFC4754(curve elliptic.Curve, r, s *big.Int) []byte {
	bitSize := curve.Params().BitSize
	byteLen := (bitSize + 7) / 8
	rb := r.Bytes()
	sb := s.Bytes()

	rp := make([]byte, 0, byteLen-len(rb))
	rp = append(rp, rb...)
	sp := make([]byte, 0, byteLen-len(sb))
	sp = append(sp, sb...)

	return append(rp, sp...)
}

func TestPlainSignature(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	verifier, err := plain.NewEcdsaKey(&key.PublicKey)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)

	t.Cleanup(cancel)

	client := setup(ctx, t, verifier)

	epochTimestamp := strconv.FormatInt(time.Now().Unix(), 10)

	payload := message.GRPCPayload{
		Headers: map[string][]string{
			message.TimestampHeaderKey: {epochTimestamp},
		},
		Method: resources.ResourceService_Get_FullMethodName,
	}

	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	hasher := sha256.New()

	_, err = hasher.Write(payloadJSON)
	require.NoError(t, err)

	r, s, err := ecdsa.Sign(rand.Reader, key, hasher.Sum(nil))
	require.NoError(t, err)

	signature := encodeRFC4754(elliptic.P256(), r, s)

	signatureBase64 := base64.StdEncoding.EncodeToString(signature)

	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		message.SignatureHeaderKey, fmt.Sprintf(
			"%s test@example.org %s %s",
			message.SignatureVersionV1,
			verifier.ID(),
			signatureBase64,
		),
		message.TimestampHeaderKey, epochTimestamp,
		message.PayloadHeaderKey, string(payloadJSON),
	))

	_, err = client.Get(ctx, &resources.GetRequest{})

	assert.NoError(t, err)
}
