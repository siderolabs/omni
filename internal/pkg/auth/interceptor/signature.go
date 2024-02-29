// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor

import (
	"context"
	"errors"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/pkg/auth"
)

var errGRPCInvalidSignature = status.Error(codes.Unauthenticated, "invalid signature")

// Signature represents a signature interceptor.
type Signature struct {
	authenticatorFunc auth.AuthenticatorFunc
	logger            *zap.Logger
}

// NewSignature returns a new signature interceptor.
func NewSignature(authenticatorFunc auth.AuthenticatorFunc, logger *zap.Logger) *Signature {
	return &Signature{
		authenticatorFunc: authenticatorFunc,
		logger:            logger,
	}
}

// Unary returns a new unary signature interceptor.
func (i *Signature) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, err := i.intercept(ctx)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// Stream returns a new stream signature interceptor.
func (i *Signature) Stream() grpc.StreamServerInterceptor {
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

func (i *Signature) intercept(ctx context.Context) (context.Context, error) {
	msg, ok := ctx.Value(auth.GRPCMessageContextKey{}).(*message.GRPC)
	if !ok {
		return nil, status.Error(codes.Internal, "missing or invalid message in context")
	}

	signature, err := msg.Signature()
	if errors.Is(err, message.ErrNotFound) { // missing signature, pass it through
		grpc_ctxtags.Extract(ctx).
			Set("authenticator.user_id", "").
			Set("authenticator.identity", "")

		return ctx, nil
	}

	if err != nil {
		i.logger.Warn("failed to get signature", zap.Error(err))

		return nil, errGRPCInvalidSignature
	}

	// set the initial identity based on the signature header, it can't be trusted yet, but it gives
	// a better context if the auth fails
	grpc_ctxtags.Extract(ctx).
		Set("authenticator.identity", signature.Identity)

	authenticator, err := i.authenticatorFunc(ctx, signature.KeyFingerprint)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil, status.Error(codes.Canceled, "context canceled while doing authentication")
		}

		i.logger.Warn("failed to get authenticator", zap.Error(err))

		return nil, errGRPCInvalidSignature
	}

	err = msg.VerifySignature(authenticator.Verifier)
	if err != nil {
		i.logger.Info("failed to verify message", zap.Error(err))

		return nil, errGRPCInvalidSignature
	}

	grpc_ctxtags.Extract(ctx).
		Set("authenticator.user_id", authenticator.UserID).
		Set("authenticator.identity", authenticator.Identity).
		Set("authenticator.role", string(authenticator.Role))

	ctx = context.WithValue(ctx, auth.UserIDContextKey{}, authenticator.UserID)
	ctx = context.WithValue(ctx, auth.IdentityContextKey{}, authenticator.Identity)
	ctx = context.WithValue(ctx, auth.RoleContextKey{}, authenticator.Role)

	return ctx, nil
}
