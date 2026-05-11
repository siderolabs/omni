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
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
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
				Name: strings.Repeat("x", omniruntime.MaxJoinTokenNameLength+1),
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
