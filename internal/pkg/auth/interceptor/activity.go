// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/containers"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

const (
	activityDebounceInterval = time.Minute
	activityWriteTimeout     = 5 * time.Second
	sweepInterval            = 10 * time.Minute
)

// Activity is a gRPC interceptor that tracks the last activity time of authenticated users.
type Activity struct {
	state      state.State
	logger     *zap.Logger
	lastUpdate containers.ConcurrentMap[string, time.Time]
	lastSweep  atomic.Int64
}

// NewActivity returns a new activity tracking interceptor.
func NewActivity(state state.State, logger *zap.Logger) *Activity {
	return &Activity{
		state:  state,
		logger: logger,
	}
}

// Unary returns a new unary activity tracking interceptor.
func (a *Activity) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		a.trackActivity(ctx)

		return handler(ctx, req)
	}
}

// Stream returns a new stream activity tracking interceptor.
func (a *Activity) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		a.trackActivity(ss.Context())

		return handler(srv, ss)
	}
}

func (a *Activity) trackActivity(ctx context.Context) {
	identity := identityFromContext(ctx)
	if identity == "" {
		return
	}

	now := time.Now()

	if last, ok := a.lastUpdate.Get(identity); ok && now.Sub(last) < activityDebounceInterval {
		return
	}

	a.lastUpdate.Set(identity, now)

	// Periodically sweep stale debounce entries to prevent unbounded memory growth from deleted users/service accounts whose entries would otherwise linger forever.
	if now.Unix()-a.lastSweep.Load() > int64(sweepInterval.Seconds()) {
		a.lastSweep.Store(now.Unix())

		a.lastUpdate.FilterInPlace(func(_ string, t time.Time) bool {
			return now.Sub(t) <= activityDebounceInterval
		})
	}

	// Write asynchronously with a detached context to avoid blocking the RPC and to prevent client disconnects from canceling the write.
	panichandler.Go(func() { //nolint:contextcheck
		writeCtx, cancel := context.WithTimeout(context.Background(), activityWriteTimeout)
		defer cancel()

		writeCtx = actor.MarkContextAsInternalActor(writeCtx)

		if _, err := safe.StateUpdateWithConflicts(writeCtx, a.state, authres.NewIdentityLastActive(identity).Metadata(),
			func(r *authres.IdentityLastActive) error {
				r.TypedSpec().Value.LastActive = timestamppb.Now()

				return nil
			},
		); err != nil && !state.IsNotFoundError(err) {
			a.logger.Warn("failed to update identity last active", zap.String("identity", identity), zap.Error(err))
		}
	}, a.logger)
}

func identityFromContext(ctx context.Context) string {
	if val, ok := ctxstore.Value[auth.IdentityContextKey](ctx); ok && val.Identity != "" {
		return val.Identity
	}

	if val, ok := ctxstore.Value[auth.VerifiedEmailContextKey](ctx); ok && val.Email != "" {
		return val.Email
	}

	return ""
}
