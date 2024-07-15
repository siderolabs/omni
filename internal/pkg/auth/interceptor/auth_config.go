// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor

import (
	"context"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// AuthConfig represents the configuration for the auth config interceptor.
type AuthConfig struct {
	logger  *zap.Logger
	enabled bool
}

// NewAuthConfig returns a new auth config interceptor.
func NewAuthConfig(enabled bool, logger *zap.Logger) *AuthConfig {
	return &AuthConfig{
		enabled: enabled,
		logger:  logger,
	}
}

// Unary returns a new unary GRPC interceptor.
func (c *AuthConfig) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = c.intercept(ctx, info.FullMethod)

		return handler(ctx, req)
	}
}

// Stream returns a new streaming GRPC interceptor.
func (c *AuthConfig) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := c.intercept(ss.Context(), info.FullMethod)

		return handler(srv, &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: ctx,
		})
	}
}

func (c *AuthConfig) intercept(ctx context.Context, method string) context.Context {
	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: c.enabled})

	if !c.enabled {
		return ctx
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	return ctxstore.WithValue(ctx, auth.GRPCMessageContextKey{Message: message.NewGRPC(md, method)})
}
