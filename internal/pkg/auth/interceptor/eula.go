// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor

import (
	"context"
	"sync/atomic"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	resapi "github.com/siderolabs/omni/client/api/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/eula"
)

// EULACheck is a gRPC interceptor that blocks all requests until the EULA has been accepted.
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
		omniURL: omniURL,
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

		// Allow watch on EulaAcceptance so the frontend can react when EULA is accepted
		if info != nil && info.FullMethod == resapi.ResourceService_Watch_FullMethodName {
			return handler(srv, ss)
		}

		if err := e.check(ctx, nil, nil); err != nil {
			return err
		}

		return handler(srv, &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: ctx,
		})
	}
}

func (e *EULACheck) check(ctx context.Context, req any, info *grpc.UnaryServerInfo) error {
	// Internal actors (e.g., controllers, startup code) bypass the EULA check.
	if actor.ContextIsInternalActor(ctx) {
		return nil
	}

	// Allow read access to public resource types (no auth required by design),
	// plus Create on EulaAcceptance so unauthenticated users can accept.
	if req != nil && info != nil {
		switch info.FullMethod {
		case resapi.ResourceService_Get_FullMethodName:
			if getReq, ok := req.(*resapi.GetRequest); ok {
				if _, public := omni.PublicResourceTypes[getReq.Type]; public {
					return nil
				}
			}
		case resapi.ResourceService_List_FullMethodName:
			if listReq, ok := req.(*resapi.ListRequest); ok {
				if _, public := omni.PublicResourceTypes[listReq.Type]; public {
					return nil
				}
			}
		case resapi.ResourceService_Create_FullMethodName:
			if createReq, ok := req.(*resapi.CreateRequest); ok && createReq.Resource != nil &&
				createReq.Resource.GetMetadata().GetType() == authres.EulaAcceptanceType {
				return nil
			}
		}
	}

	// Fast path: once accepted it stays accepted.
	if e.accepted.Load() {
		return nil
	}

	internalCtx := actor.MarkContextAsInternalActor(ctx)

	accepted, err := eula.IsAccepted(internalCtx, e.st)
	if err != nil {
		e.logger.Warn("failed to check EULA acceptance", zap.Error(err))

		// Fail open only on read-only requests; for writes, fail closed.
		return status.Error(codes.Internal, "failed to check EULA acceptance status")
	}

	if accepted {
		e.accepted.Store(true)

		return nil
	}

	return status.Errorf(codes.FailedPrecondition, "EULA has not been accepted; please accept the End User License Agreement before using Omni: %s/eula", e.omniURL)
}
