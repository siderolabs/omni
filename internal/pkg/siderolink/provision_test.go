// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

//nolint:maintidx
func TestProvision(t *testing.T) {
	t.Parallel()

	validToken := "validToken"

	genKey := func() string {
		privateKey, err := wgtypes.GeneratePrivateKey()
		require.NoError(t, err)

		return privateKey.PublicKey().String()
	}

	setup := func(ctx context.Context, t *testing.T, disableLegacyJoinToken bool) (state.State, *siderolink.ProvisionHandler) {
		state := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)

		ctx, cancel := context.WithCancel(ctx)

		runtime, err := runtime.NewRuntime(state, logger)
		require.NoError(t, err)

		peers := siderolink.NewPeersPool(logger, &fakeWireguardHandler{peers: map[string]wgtypes.Peer{}})

		require.NoError(t, runtime.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.PendingMachine](peers)))
		require.NoError(t, runtime.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.Link](peers)))

		var eg errgroup.Group

		eg.Go(func() error {
			return runtime.Run(ctx)
		})

		t.Cleanup(func() {
			cancel()

			require.NoError(t, eg.Wait())
		})

		provisionHandler := siderolink.NewProvisionHandler(logger, state, disableLegacyJoinToken)

		config := siderolinkres.NewConfig(resources.DefaultNamespace)
		config.TypedSpec().Value.ServerAddress = "127.0.0.1"
		config.TypedSpec().Value.PublicKey = genKey()
		config.TypedSpec().Value.JoinToken = validToken
		config.TypedSpec().Value.Subnet = wireguard.NetworkPrefix("").String()

		require.NoError(t, state.Create(ctx, config))

		return state, provisionHandler
	}

	t.Run("full flow", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, true)

		request := &pb.ProvisionRequest{
			NodeUuid:      "machine-1",
			NodePublicKey: genKey(),
			TalosVersion:  pointer.To("v1.9.0"),
			JoinToken:     pointer.To(validToken),
		}

		response, err := provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, state, []string{request.NodePublicKey},
			func(r *siderolinkres.PendingMachine, assertion *assert.Assertions) {
				assertion.NotEmpty(r.TypedSpec().Value.NodeSubnet)
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
			},
		)

		require.NotEmpty(t, response.ServerAddress)
		require.NotEmpty(t, response.ServerPublicKey)
		require.NotEmpty(t, response.NodeAddressPrefix)

		request.NodeUniqueToken = pointer.To("token")

		response, err = provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.NotEmpty(r.TypedSpec().Value.NodeSubnet)
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
				require.Equal(t, *request.NodeUniqueToken, r.TypedSpec().Value.NodeUniqueToken)
			},
		)

		require.NotEmpty(t, response.ServerAddress)
		require.NotEmpty(t, response.ServerPublicKey)
		require.NotEmpty(t, response.NodeAddressPrefix)
	})

	for _, tt := range []struct {
		name       string
		token      string
		shouldFail bool
	}{
		{
			name:  "valid token",
			token: validToken,
		},
		{
			name:       "invalid token",
			token:      "nope",
			shouldFail: true,
		},
	} {
		t.Run(fmt.Sprintf("migration %s", tt.name), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
			defer cancel()

			state, provisionHandler := setup(ctx, t, true)

			request := &pb.ProvisionRequest{
				NodeUuid:      fmt.Sprintf("machine-migration-%s", tt.name),
				NodePublicKey: genKey(),
				TalosVersion:  pointer.To("v1.9.0"),
				JoinToken:     pointer.To(tt.token),
			}

			link := siderolinkres.NewLink(resources.DefaultNamespace, request.NodeUuid, &specs.SiderolinkSpec{
				NodePublicKey: "",
				NodeSubnet:    "asdf",
			})

			require.NoError(t, state.Create(ctx, link))

			_, err := provisionHandler.Provision(ctx, request)
			if tt.shouldFail {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			rtestutils.AssertResources(ctx, t, state, []string{request.NodePublicKey},
				func(r *siderolinkres.PendingMachine, assertion *assert.Assertions) {
					assertion.NotEmpty(r.TypedSpec().Value.NodeSubnet)
					assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
				},
			)

			rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
				func(r *siderolinkres.Link, assertion *assert.Assertions) {
					assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
				},
			)
		})
	}

	for _, mode := range []struct {
		name           string
		legacyDisabled bool
	}{
		{
			name: "legacy",
		},
		{
			name:           "normal",
			legacyDisabled: true,
		},
	} {
		for _, tt := range []struct {
			request  *pb.ProvisionRequest
			linkSpec *specs.SiderolinkSpec
			errcheck func(t *testing.T, err error)
			name     string
		}{
			{
				name: "no join token, valid node token",
				request: &pb.ProvisionRequest{
					NodePublicKey:   genKey(),
					NodeUniqueToken: pointer.To("letmein"),
					TalosVersion:    pointer.To("v1.6.0"),
				},
				linkSpec: &specs.SiderolinkSpec{
					NodeUniqueToken: "letmein",
				},
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "valid join token, invalid node token",
				request: &pb.ProvisionRequest{
					NodePublicKey:   genKey(),
					NodeUniqueToken: pointer.To("letmein"),
					TalosVersion:    pointer.To("v1.9.0"),
				},
				linkSpec: &specs.SiderolinkSpec{
					NodeUniqueToken: "youshallnotpass",
				},
				errcheck: func(t *testing.T, err error) {
					require.Equal(t, codes.PermissionDenied, status.Code(err))
				},
			},
			{
				name: "migration",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  pointer.To("v1.9.0"),
					JoinToken:     pointer.To(validToken),
				},
				linkSpec: &specs.SiderolinkSpec{
					NodeUniqueToken: "",
				},
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "initial join",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  pointer.To("v1.9.0"),
					JoinToken:     pointer.To(validToken),
				},
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "initial join, no valid token",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  pointer.To("v1.9.0"),
				},
				errcheck: func(t *testing.T, err error) {
					require.Equal(t, codes.PermissionDenied, status.Code(err))
				},
			},
			{
				name: "below 1.6",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  pointer.To("v1.4.0"),
					JoinToken:     &validToken,
				},
				errcheck: func(t *testing.T, err error) {
					if mode.legacyDisabled {
						require.Equal(t, codes.FailedPrecondition, status.Code(err))

						return
					}

					require.NoError(t, err)
				},
			},
			{
				name: "below 1.6, no token",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  pointer.To("v1.4.0"),
				},
				errcheck: func(t *testing.T, err error) {
					require.Equal(t, codes.PermissionDenied, status.Code(err))
				},
			},
		} {
			t.Run(fmt.Sprintf("access check, mode %s: %s", mode.name, tt.name), func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
				defer cancel()

				state, provisionHandler := setup(ctx, t, mode.legacyDisabled)

				if tt.linkSpec != nil {
					require.NoError(t, state.Create(ctx, siderolinkres.NewLink(resources.DefaultNamespace, "machine", tt.linkSpec)))
				}

				tt.request.NodeUuid = "machine"

				_, err := provisionHandler.Provision(ctx, tt.request)
				tt.errcheck(t, err)
			})
		}
	}

	t.Run("allow legacy join", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, false)

		request := &pb.ProvisionRequest{
			NodeUuid:      "machine-legacy",
			NodePublicKey: genKey(),
			TalosVersion:  pointer.To("v1.5.0"),
			JoinToken:     pointer.To(validToken),
		}

		link := siderolinkres.NewLink(resources.DefaultNamespace, request.NodeUuid, &specs.SiderolinkSpec{})

		require.NoError(t, state.Create(ctx, link))

		_, err := provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
			},
		)

		rtestutils.AssertNoResource[*siderolinkres.PendingMachine](ctx, t, state, request.NodePublicKey)
	})

	t.Run("provider token", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, true)

		token, err := jointoken.NewWithExtraData(validToken, map[string]string{
			omni.LabelInfraProviderID: "test",
		})

		require.NoError(t, err)

		encoded, err := token.Encode()
		require.NoError(t, err)

		request := &pb.ProvisionRequest{
			NodeUuid:        "machine-from-provider",
			NodePublicKey:   genKey(),
			TalosVersion:    pointer.To("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: pointer.To("none"),
		}

		_, err = provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)

				providerID, ok := r.Metadata().Annotations().Get(omni.LabelInfraProviderID)
				assertion.True(ok)
				assertion.Equal("test", providerID)
			},
		)

		rtestutils.AssertNoResource[*siderolinkres.PendingMachine](ctx, t, state, request.NodePublicKey)
	})
}
