// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor

import (
	"context"
	"errors"
	"fmt"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/siderolabs/go-api-signature/pkg/jwt"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/auth0"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

var errGRPCInvalidJWT = status.Error(codes.Unauthenticated, "invalid jwt")

// JWT is a GRPC interceptor that verifies JWT tokens.
type JWT struct {
	jwtVerifier jwt.Verifier
	logger      *zap.Logger
}

// NewJWT returns a new JWT interceptor.
func NewJWT(jwtVerifier jwt.Verifier, logger *zap.Logger) *JWT {
	return &JWT{
		jwtVerifier: jwtVerifier,
		logger:      logger,
	}
}

// Unary returns a new unary JWT interceptor.
func (i *JWT) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, err := i.intercept(ctx)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// Stream returns a new stream JWT interceptor.
func (i *JWT) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := i.intercept(ss.Context())
		if err != nil {
			return err
		}

		return handler(srv, &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: ctx,
		})
	}
}

func (i *JWT) intercept(ctx context.Context) (context.Context, error) {
	msgVal, ok := ctxstore.Value[auth.GRPCMessageContextKey](ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "missing or invalid message in context")
	}

	claims, err := msgVal.Message.VerifyJWT(ctx, i.jwtVerifier)
	if errors.Is(err, message.ErrNotFound) { // missing jwt, pass it through
		return ctx, nil
	}

	if err != nil {
		i.logger.Info("invalid jwt", zap.Error(err))

		var errEmailNotVerified *auth0.EmailNotVerifiedError

		if errors.As(err, &errEmailNotVerified) {
			return nil, status.Error(codes.Unauthenticated,
				fmt.Sprintf(`Email address %q is not verified. Please check your email for a message to verify it, then click "Log In" again.`,
					errEmailNotVerified.Email))
		}

		return nil, errGRPCInvalidJWT
	}

	ctx = ctxstore.WithValue(ctx, auth.VerifiedEmailContextKey{Email: claims.VerifiedEmail})

	return ctx, nil
}
