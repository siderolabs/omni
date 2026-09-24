// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// TestCallerAccess checks that the requests made on behalf of a caller are refused before any client is built, unless
// the caller may read the cluster.
func TestCallerAccess(t *testing.T) {
	t.Parallel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	r := kubernetes.New(st, zaptest.NewLogger(t), "", "omni", "")

	callerContext := func(ctx context.Context, r role.Role) context.Context {
		ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: true})
		ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: "user@example.org"})

		return ctxstore.WithValue(ctx, auth.RoleContextKey{Role: r})
	}

	for _, tt := range []struct {
		ctx      context.Context //nolint:containedctx
		name     string
		cluster  string
		wantCode codes.Code
	}{
		{name: "caller without a role", ctx: callerContext(t.Context(), role.None), cluster: "alpha", wantCode: codes.PermissionDenied},
		{name: "unsigned caller", ctx: ctxstore.WithValue(t.Context(), auth.EnabledAuthContextKey{Enabled: true}), cluster: "alpha", wantCode: codes.Unauthenticated},
		{name: "no cluster", ctx: callerContext(t.Context(), role.Admin), wantCode: codes.InvalidArgument},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := []runtime.QueryOption{runtime.WithContext(tt.cluster), runtime.WithResource("pods"), runtime.WithNamespace("default")}

			_, err := r.Get(tt.ctx, append(opts, runtime.WithName("pod"))...)
			require.Error(t, err)
			assert.Equal(t, tt.wantCode, status.Code(err), "get: %v", err)

			_, err = r.List(tt.ctx, opts...)
			require.Error(t, err)
			assert.Equal(t, tt.wantCode, status.Code(err), "list: %v", err)

			err = r.Watch(tt.ctx, nil, opts...)
			require.Error(t, err)
			assert.Equal(t, tt.wantCode, status.Code(err), "watch: %v", err)
		})
	}
}
