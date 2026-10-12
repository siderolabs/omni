// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpcutil_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
	grpc_ctxtags "github.com/siderolabs/omni/internal/pkg/grpcutil/grpctags"
)

func TestAuditClientAddress(t *testing.T) {
	for _, tc := range []struct {
		name     string
		md       metadata.MD
		expected string
	}{
		{name: "forwarded for", md: metadata.Pairs("x-forwarded-for", "203.0.113.9"), expected: "203.0.113.9"},
		{name: "no address", md: metadata.MD{}, expected: ""},                                     // SetRealPeerAddress sets <unknown_ip>
		{name: "invalid address", md: metadata.Pairs("x-forwarded-for", "garbage"), expected: ""}, // SetRealPeerAddress sets <invalid_ip>
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := grpc_ctxtags.SetInContext(metadata.NewIncomingContext(t.Context(), tc.md), grpc_ctxtags.NewTags())

			var session auditlog.Session

			handler := func(ctx context.Context, _ any) (any, error) {
				data, ok := ctxstore.Value[*auditlog.Data](ctx)
				require.True(t, ok)

				session = data.Session

				return nil, nil //nolint:nilnil
			}

			// the same order as in the API server
			_, err := grpcutil.SetRealPeerAddress()(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
				return grpcutil.SetAuditData()(ctx, req, &grpc.UnaryServerInfo{}, handler)
			})
			require.NoError(t, err)

			assert.Equal(t, tc.expected, session.ClientAddress)
		})
	}
}
