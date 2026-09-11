// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

const joinConfigTestToken = "test-join-token"

func newJoinConfigTestState(t *testing.T) state.State {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	ctx := actor.MarkContextAsInternalActor(t.Context())

	apiConfig := siderolinkres.NewAPIConfig()
	apiConfig.TypedSpec().Value.EventsPort = 8091
	apiConfig.TypedSpec().Value.LogsPort = 8092
	apiConfig.TypedSpec().Value.MachineApiAdvertisedUrl = "grpc://127.0.0.1:8090"

	require.NoError(t, st.Create(ctx, apiConfig))

	defaultToken := siderolinkres.NewDefaultJoinToken()
	defaultToken.TypedSpec().Value.TokenId = joinConfigTestToken

	require.NoError(t, st.Create(ctx, defaultToken))

	return st
}

// tokenFromKernelArgs pulls the join token back out of the rendered siderolink kernel argument.
func tokenFromKernelArgs(t *testing.T, args []string) jointoken.JoinToken {
	t.Helper()

	for _, arg := range args {
		value, ok := strings.CutPrefix(arg, "siderolink.api=")
		if !ok {
			continue
		}

		u, err := url.Parse(value)
		require.NoError(t, err)

		token, err := jointoken.Parse(u.Query().Get("jointoken"))
		require.NoError(t, err)

		return token
	}

	t.Fatal("no siderolink.api kernel argument was rendered")

	return jointoken.JoinToken{}
}

func TestGetMachineJoinConfigMachineLabels(t *testing.T) {
	server := grpcomni.NewManagementServer(newJoinConfigTestState(t), nil, zaptest.NewLogger(t), false, nil, nil)
	ctx := managementPowerTestContext(t.Context(), "user@example.com", role.Reader)

	t.Run("labels are signed into the token", func(t *testing.T) {
		resp, err := server.GetMachineJoinConfig(ctx, &management.GetMachineJoinConfigRequest{
			MachineLabels: map[string]string{"env": "prod", "rack": "a12"},
		})
		require.NoError(t, err)

		token := tokenFromKernelArgs(t, resp.KernelArgs)

		require.Equal(t, jointoken.Version3, token.Version)
		require.Equal(t, map[string]string{"env": "prod", "rack": "a12"}, token.Labels)
		require.True(t, token.IsValid(joinConfigTestToken))

		// the machine config has to embed the very same token as the kernel args, so a machine
		// gets the same labels whichever of the two it is given. Both carry it URL escaped.
		_, encoded, ok := strings.Cut(resp.KernelArgs[0], "jointoken=")
		require.True(t, ok, "the kernel args carry no join token")
		require.Contains(t, encoded, "v3%3A")
		require.Contains(t, resp.Config, encoded)
	})

	t.Run("no labels keeps the token plain", func(t *testing.T) {
		resp, err := server.GetMachineJoinConfig(ctx, &management.GetMachineJoinConfigRequest{})
		require.NoError(t, err)

		token := tokenFromKernelArgs(t, resp.KernelArgs)

		require.Equal(t, jointoken.VersionPlain, token.Version)
	})

	t.Run("system labels are rejected", func(t *testing.T) {
		_, err := server.GetMachineJoinConfig(ctx, &management.GetMachineJoinConfigRequest{
			MachineLabels: map[string]string{omnires.LabelCluster: "c1"},
		})

		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))
	})

	t.Run("oversized labels are rejected", func(t *testing.T) {
		_, err := server.GetMachineJoinConfig(ctx, &management.GetMachineJoinConfigRequest{
			MachineLabels: map[string]string{"env": strings.Repeat("a", jointoken.MaxEncodedTokenLen)},
		})

		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))
	})
}
