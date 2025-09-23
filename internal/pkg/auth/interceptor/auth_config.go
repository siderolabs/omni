// Copyright (c) 2025 Sidero Labs, Inc.
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

	resapi "github.com/siderolabs/omni/client/api/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
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
		isGetAuthConfigRequest := false

		if req != nil && info != nil && info.FullMethod == resapi.ResourceService_Get_FullMethodName {
			if getReq, getReqOk := req.(*resapi.GetRequest); getReqOk && getReq.Type == authres.AuthConfigType {
				isGetAuthConfigRequest = true
			}
		}

		ctx = c.intercept(ctx, isGetAuthConfigRequest, info.FullMethod)

		return handler(ctx, req)
	}
}

// Stream returns a new streaming GRPC interceptor.
func (c *AuthConfig) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := c.intercept(ss.Context(), false, info.FullMethod)

		return handler(srv, &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: ctx,
		})
	}
}

func (c *AuthConfig) intercept(ctx context.Context, isGetAuthConfigRequest bool, method string) context.Context {
	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: c.enabled})

	if !c.enabled {
		return ctx
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	msg := message.NewGRPC(md, method, message.WithSignatureRequiredCheck(func() (bool, error) {
		if actor.ContextIsInternalActor(ctx) {
			return false, nil
		}

		return !isGetAuthConfigRequest, nil
	}))

	auditData, ok := ctxstore.Value[*audit.Data](ctx)
	if ok {
		sig, err := msg.Signature()
		if err == nil {
			auditData.Session.Fingerprint = sig.KeyFingerprint
			auditData.Session.Email = sig.Identity
		}
	}

	return ctxstore.WithValue(ctx, auth.GRPCMessageContextKey{Message: msg})
}
