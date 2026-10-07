// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor

import (
	"context"
	"strings"
	"sync/atomic"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	authpb "github.com/siderolabs/go-api-signature/api/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	resapi "github.com/siderolabs/omni/client/api/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	"github.com/siderolabs/omni/internal/pkg/eula"
)

// EULACheck is a gRPC interceptor that blocks the requests of a signed in user until the EULA is accepted.
type EULACheck struct {
	st       eula.StateGetter
	logger   *zap.Logger
	omniURL  string
	accepted atomic.Bool
}

// NewEULACheck creates a new EULACheck interceptor.
func NewEULACheck(st eula.StateGetter, logger *zap.Logger, omniURL string) *EULACheck {
	return &EULACheck{
		st:      st,
		logger:  logger,
		omniURL: strings.TrimSuffix(omniURL, "/"),
	}
}

// Unary returns a new unary gRPC interceptor.
func (e *EULACheck) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := e.check(ctx, req, info); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// Stream returns a new streaming gRPC interceptor.
func (e *EULACheck) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		if err := e.check(ctx, nil, nil); err != nil {
			return err
		}

		return handler(srv, &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: ctx,
		})
	}
}

// allowedWhileUnaccepted reports whether the request reads or writes the EULA acceptance itself, or
// signs the user out. Those have to work before the EULA is accepted.
func allowedWhileUnaccepted(req any, info *grpc.UnaryServerInfo) bool {
	if info == nil {
		return false
	}

	switch info.FullMethod {
	case authpb.AuthService_RevokePublicKey_FullMethodName:
		return true
	case resapi.ResourceService_Get_FullMethodName:
		getReq, ok := req.(*resapi.GetRequest)

		return ok && getReq.GetType() == authres.EulaAcceptanceType
	case resapi.ResourceService_Create_FullMethodName:
		createReq, ok := req.(*resapi.CreateRequest)

		return ok && createReq.GetResource().GetMetadata().GetType() == authres.EulaAcceptanceType
	default:
		return false
	}
}

func (e *EULACheck) check(ctx context.Context, req any, info *grpc.UnaryServerInfo) error {
	// Internal actors (e.g., controllers, startup code) bypass the EULA check.
	if actor.ContextIsInternalActor(ctx) {
		return nil
	}

	// a request without an identity is on its way to signing in
	if _, ok := ctxstore.Value[auth.IdentityContextKey](ctx); !ok {
		return nil
	}

	if allowedWhileUnaccepted(req, info) {
		return nil
	}

	// Fast path: once accepted it stays accepted.
	if e.accepted.Load() {
		return nil
	}

	internalCtx := actor.MarkContextAsInternalActor(ctx)

	accepted, err := eula.IsAccepted(internalCtx, e.st)
	if err != nil {
		e.logger.Warn("failed to check EULA acceptance", zap.Error(err))

		return status.Error(codes.Internal, "failed to check EULA acceptance status")
	}

	if accepted {
		e.accepted.Store(true)

		return nil
	}

	return status.Errorf(codes.FailedPrecondition, "EULA has not been accepted; please accept the End User License Agreement before using Omni: %s/eula", e.omniURL)
}
