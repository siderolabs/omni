// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access/role"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validations"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestCreateJoinTokenValidation(t *testing.T) {
	server := grpcomni.NewManagementServer(nil, nil, zaptest.NewLogger(t), false, nil, nil)
	ctx := ctxstore.WithValue(context.Background(), auth.EnabledAuthContextKey{Enabled: true})
	ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: role.Admin})

	for _, tt := range []struct {
		request *management.CreateJoinTokenRequest
		name    string
	}{
		{
			name:    "empty name",
			request: &management.CreateJoinTokenRequest{},
		},
		{
			name: "name too long",
			request: &management.CreateJoinTokenRequest{
				Name: strings.Repeat("x", validations.MaxJoinTokenNameLength+1),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.CreateJoinToken(ctx, tt.request)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestCreateSchematicFromRawAuthorization(t *testing.T) {
	server := grpcomni.NewManagementServer(nil, nil, zaptest.NewLogger(t), false, nil, nil)
	request := &management.CreateSchematicFromRawRequest{
		RawSchematic: []byte(":\n"),
	}

	for _, tt := range []struct {
		role    role.Role
		name    string
		allowed bool
	}{
		{
			name: "reader denied",
			role: role.Reader,
		},
		{
			name:    "operator allowed",
			role:    role.Operator,
			allowed: true,
		},
		{
			name:    "infra provider allowed",
			role:    role.InfraProvider,
			allowed: true,
		},
		{
			name:    "admin allowed",
			role:    role.Admin,
			allowed: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ctxstore.WithValue(context.Background(), auth.EnabledAuthContextKey{Enabled: true})
			ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: tt.role})

			_, err := server.CreateSchematicFromRaw(ctx, request)
			require.Error(t, err)

			if !tt.allowed {
				require.Equal(t, codes.PermissionDenied, status.Code(err))

				return
			}

			require.ErrorContains(t, err, "failed to unmarshal raw schematic")
		})
	}
}
