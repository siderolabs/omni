// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"context"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/siderolabs/grpc-proxy/proxy"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

// Director is a gRPC proxy director.
type Director interface {
	Director(ctx context.Context, fullMethodName string) (proxy.Mode, []proxy.Backend, error)
}

// NewServer creates new gRPC server which routes request either to self or to Talos backend.
func NewServer(router Director, options ...grpc.ServerOption) *grpc.Server {
	opts := append(
		[]grpc.ServerOption{
			grpc.ForceServerCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(
				proxy.TransparentHandler(
					router.Director,
				),
			),
			grpc.SharedWriteBuffer(true),
		},
		options...,
	)

	return grpc.NewServer(opts...)
}

// Interceptors returns gRPC interceptors for router.
func Interceptors(logger *zap.Logger) grpc.ServerOption {
	return grpc.ChainStreamInterceptor(
		grpc_ctxtags.StreamServerInterceptor(),
		grpc_zap.StreamServerInterceptor(logger, grpc_zap.WithMessageProducer(msgProducer)),
		grpcutil.StreamSetUserAgent(),
		grpcutil.StreamSetRealPeerAddress(),
	)
}

func msgProducer(ctx context.Context, msg string, level zapcore.Level, code codes.Code, err error, duration zapcore.Field) {
	if !grpcutil.ShouldLog(ctx) {
		return
	}

	grpc_zap.DefaultMessageProducer(ctx, msg, level, code, err, duration)
}
