// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/google/uuid"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/siderolink/pkg/wgtunnel/wggrpc"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/logging"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

type testPendingMachineStatusController = qtransform.QController[*siderolinkres.PendingMachine, *siderolinkres.PendingMachineStatus]

func newPendingMachineStatusController(installedCallback func(machine *siderolinkres.PendingMachine) bool) *testPendingMachineStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*siderolinkres.PendingMachine, *siderolinkres.PendingMachineStatus]{
			Name: "PendingMachineStatusController",
			MapMetadataFunc: func(pendingMachine *siderolinkres.PendingMachine) *siderolinkres.PendingMachineStatus {
				return siderolinkres.NewPendingMachineStatus(pendingMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(pendingMachineStatus *siderolinkres.PendingMachineStatus) *siderolinkres.PendingMachine {
				return siderolinkres.NewPendingMachine(pendingMachineStatus.Metadata().ID(), nil)
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, machine *siderolinkres.PendingMachine, status *siderolinkres.PendingMachineStatus) error {
				status.TypedSpec().Value.TalosInstalled = installedCallback(machine)

				return nil
			},
		},
		qtransform.WithConcurrency(32),
	)
}

//nolint:maintidx,gocognit
func TestProvision(t *testing.T) {
	t.Parallel()

	validFingerprint := uuid.NewString()

	validToken, e := jointoken.NewNodeUniqueToken(validFingerprint, "validToken").Encode()
	require.NoError(t, e)

	validFingerprintOnly, e := jointoken.NewNodeUniqueToken(validFingerprint, "randomized").Encode()
	require.NoError(t, e)

	invalidToken, e := jointoken.NewNodeUniqueToken(uuid.NewString(), "invalidToken").Encode()
	require.NoError(t, e)

	genKey := func() string {
		return genPublicKey(t)
	}

	setup := func(ctx context.Context, t *testing.T, mode config.SiderolinkServiceJoinTokensMode) (state.State, *siderolink.ProvisionHandler) {
		return provisionFixture(ctx, t, mode, validToken, &fakeWireguardHandler{peers: map[string]wgtypes.Peer{}}, func(*siderolinkres.PendingMachine) bool {
			return true
		})
	}

	t.Run("full flow", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		request := &pb.ProvisionRequest{
			NodeUuid:      "machine-1",
			NodePublicKey: genKey(),
			TalosVersion:  new("v1.9.0"),
			JoinToken:     new(validToken),
		}

		response, err := provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(
			ctx, t, state, []string{request.NodePublicKey},
			func(r *siderolinkres.PendingMachine, assertion *assert.Assertions) {
				assertion.NotEmpty(r.TypedSpec().Value.NodeSubnet)
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
			},
		)

		require.NotEmpty(t, response.ServerAddress)
		require.NotEmpty(t, response.ServerPublicKey)
		require.NotEmpty(t, response.NodeAddressPrefix)

		request.NodeUniqueToken = new(validToken)

		response, err = provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(
			ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.NotEmpty(r.TypedSpec().Value.NodeSubnet)
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
			},
		)

		rtestutils.AssertResources(
			ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.NodeUniqueToken, assertion *assert.Assertions) {
				require.Equal(t, *request.NodeUniqueToken, r.TypedSpec().Value.Token)
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
			token:      invalidToken,
			shouldFail: true,
		},
	} {
		t.Run(fmt.Sprintf("migration %s", tt.name), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
			defer cancel()

			state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

			request := &pb.ProvisionRequest{
				NodeUuid:      fmt.Sprintf("machine-migration-%s", tt.name),
				NodePublicKey: genKey(),
				TalosVersion:  new("v1.9.0"),
				JoinToken:     new(tt.token),
			}

			link := siderolinkres.NewLink(request.NodeUuid, &specs.SiderolinkSpec{
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

			rtestutils.AssertResources(
				ctx, t, state, []string{request.NodePublicKey},
				func(r *siderolinkres.PendingMachine, assertion *assert.Assertions) {
					assertion.NotEmpty(r.TypedSpec().Value.NodeSubnet)
					assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
				},
			)

			request.NodeUniqueToken = &validToken

			_, err = provisionHandler.Provision(ctx, request)
			require.NoError(t, err)

			rtestutils.AssertResources(
				ctx, t, state, []string{request.NodeUuid},
				func(r *siderolinkres.Link, assertion *assert.Assertions) {
					assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
				},
			)
		})
	}

	t.Run("divergent pending machine does not rewrite the link address", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		linkSubnet := "fdae:41e4:649b:9303::1:1/64"

		request := &pb.ProvisionRequest{
			NodeUuid:        "machine-divergent-pending",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.0"),
			JoinToken:       new(validToken),
			NodeUniqueToken: &validToken,
		}

		link := siderolinkres.NewLink(request.NodeUuid, &specs.SiderolinkSpec{
			NodePublicKey: request.NodePublicKey,
			NodeSubnet:    linkSubnet,
		})

		require.NoError(t, state.Create(ctx, link))

		// a leftover pending machine for the same public key carrying a different
		// address must never overwrite the link's address on re-provision
		pendingMachine := siderolinkres.NewPendingMachine(request.NodePublicKey, &specs.SiderolinkSpec{
			NodePublicKey: request.NodePublicKey,
			NodeSubnet:    "fdae:41e4:649b:9303::2:2/64",
		})

		require.NoError(t, state.Create(ctx, pendingMachine))

		response, err := provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		require.Equal(t, linkSubnet, response.NodeAddressPrefix)

		rtestutils.AssertResources(
			ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.Equal(linkSubnet, r.TypedSpec().Value.NodeSubnet)
			},
		)
	})

	for _, mode := range []struct {
		name string
		mode config.SiderolinkServiceJoinTokensMode
	}{
		{
			name: "legacy",
			mode: config.SiderolinkServiceJoinTokensModeLegacyAllowed,
		},
		{
			name: "normal",
			mode: config.SiderolinkServiceJoinTokensModeStrict,
		},
	} {
		for _, tt := range []struct {
			request           *pb.ProvisionRequest
			linkSpec          *specs.SiderolinkSpec
			errcheck          func(t *testing.T, err error)
			nodeUniqueToken   string
			name              string
			talosNotInstalled bool
		}{
			{
				name: "same fingerprint, valid join token, talos installed",
				request: &pb.ProvisionRequest{
					NodePublicKey:   genKey(),
					NodeUniqueToken: new(validFingerprintOnly),
					TalosVersion:    new("v1.6.0"),
					JoinToken:       new(validToken),
				},
				linkSpec:        &specs.SiderolinkSpec{},
				nodeUniqueToken: validToken,
				errcheck: func(t *testing.T, err error) {
					require.Error(t, err)
					require.Equal(t, codes.PermissionDenied, status.Code(err))
				},
			},
			{
				name: "same fingerprint, valid join token, talos not installed",
				request: &pb.ProvisionRequest{
					NodePublicKey:   genKey(),
					NodeUniqueToken: new(validFingerprintOnly),
					TalosVersion:    new("v1.6.0"),
					JoinToken:       new(validToken),
				},
				talosNotInstalled: true,
				linkSpec:          &specs.SiderolinkSpec{},
				nodeUniqueToken:   validToken,
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "sa %#vme fingerprint, in, provisionContext.requestNodeUniqueTokenvalid join token, talos not installed",
				request: &pb.ProvisionRequest{
					NodePublicKey:   genKey(),
					NodeUniqueToken: new(validFingerprintOnly),
					TalosVersion:    new("v1.6.0"),
				},
				talosNotInstalled: true,
				nodeUniqueToken:   validToken,
				errcheck: func(t *testing.T, err error) {
					require.Error(t, err)
					require.Equal(t, codes.PermissionDenied, status.Code(err))
				},
			},
			{
				name: "no join token, valid node token, has link",
				request: &pb.ProvisionRequest{
					NodePublicKey:   genKey(),
					NodeUniqueToken: new(validToken),
					TalosVersion:    new("v1.6.0"),
				},
				nodeUniqueToken: validToken,
				linkSpec:        &specs.SiderolinkSpec{},
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "no join token, valid node token, no link",
				request: &pb.ProvisionRequest{
					NodePublicKey:   genKey(),
					NodeUniqueToken: new(validToken),
					TalosVersion:    new("v1.6.0"),
				},
				nodeUniqueToken: validToken,
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "valid join token, invalid node token",
				request: &pb.ProvisionRequest{
					NodePublicKey:   genKey(),
					NodeUniqueToken: new(invalidToken),
					TalosVersion:    new("v1.9.0"),
					JoinToken:       new(validToken),
				},
				nodeUniqueToken: validToken,
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "migration",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  new("v1.9.0"),
					JoinToken:     new(validToken),
				},
				linkSpec: &specs.SiderolinkSpec{},
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "initial join",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  new("v1.9.0"),
					JoinToken:     new(validToken),
				},
				errcheck: func(t *testing.T, err error) {
					require.NoError(t, err)
				},
			},
			{
				name: "initial join, no valid token",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  new("v1.9.0"),
				},
				errcheck: func(t *testing.T, err error) {
					require.Equal(t, codes.PermissionDenied, status.Code(err))
				},
			},
			{
				name: "below 1.6",
				request: &pb.ProvisionRequest{
					NodePublicKey: genKey(),
					TalosVersion:  new("v1.4.0"),
					JoinToken:     &validToken,
				},
				errcheck: func(t *testing.T, err error) {
					if mode.mode == config.SiderolinkServiceJoinTokensModeStrict {
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
					TalosVersion:  new("v1.4.0"),
				},
				errcheck: func(t *testing.T, err error) {
					if mode.mode == config.SiderolinkServiceJoinTokensModeLegacyAllowed {
						require.Equal(t, codes.PermissionDenied, status.Code(err))

						return
					}

					require.Equal(t, codes.FailedPrecondition, status.Code(err))
				},
			},
		} {
			t.Run(fmt.Sprintf("access check, mode %s: %s", mode.name, tt.name), func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
				defer cancel()

				state, provisionHandler := setup(ctx, t, mode.mode)

				machine := tt.name

				if tt.nodeUniqueToken != "" {
					nodeUniqueToken := siderolinkres.NewNodeUniqueToken(machine)

					nodeUniqueToken.TypedSpec().Value.Token = tt.nodeUniqueToken

					require.NoError(t, state.Create(ctx, nodeUniqueToken))
				}

				if tt.linkSpec != nil {
					link := siderolinkres.NewLink(machine, tt.linkSpec)
					if !tt.talosNotInstalled {
						link.Metadata().Annotations().Set(siderolinkres.ForceValidNodeUniqueToken, "")
					}

					require.NoError(t, state.Create(ctx, link))
				}

				tt.request.NodeUuid = machine

				_, err := provisionHandler.Provision(ctx, tt.request)
				tt.errcheck(t, err)
			})
		}
	}

	t.Run("allow legacy join", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeLegacyAllowed)

		request := &pb.ProvisionRequest{
			NodeUuid:      "machine-legacy",
			NodePublicKey: genKey(),
			TalosVersion:  new("v1.5.0"),
			JoinToken:     new(validToken),
		}

		link := siderolinkres.NewLink(request.NodeUuid, &specs.SiderolinkSpec{})

		require.NoError(t, state.Create(ctx, link))

		_, err := provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(
			ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
			},
		)

		rtestutils.AssertNoResource[*siderolinkres.PendingMachine](ctx, t, state, request.NodePublicKey)
	})

	t.Run("restrict legacy join", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeLegacyAllowed)

		request := &pb.ProvisionRequest{
			NodeUuid:      "machine-legacy",
			NodePublicKey: genKey(),
			TalosVersion:  new("v1.5.0"),
			JoinToken:     new(validToken),
		}

		link := siderolinkres.NewLink(request.NodeUuid, &specs.SiderolinkSpec{})

		nodeUniqueToken := siderolinkres.NewNodeUniqueToken(request.NodeUuid)
		nodeUniqueToken.TypedSpec().Value.Token = validToken

		require.NoError(t, state.Create(ctx, link))
		require.NoError(t, state.Create(ctx, nodeUniqueToken))

		_, err := provisionHandler.Provision(ctx, request)
		require.Error(t, err)
		require.EqualValues(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("UUID conflict", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		uniqueToken, err := jointoken.NewNodeUniqueToken(uuid.NewString(), uuid.NewString()).Encode()
		require.NoError(t, err)

		request := &pb.ProvisionRequest{
			NodeUuid:        "so-duplicate",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.4"),
			JoinToken:       new(validToken),
			NodeUniqueToken: new(uniqueToken),
		}

		linkResponse, err := provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		uniqueToken, err = jointoken.NewNodeUniqueToken(uuid.NewString(), uuid.NewString()).Encode()
		require.NoError(t, err)

		request2 := &pb.ProvisionRequest{
			NodeUuid:        "so-duplicate",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.4"),
			JoinToken:       new(validToken),
			NodeUniqueToken: new(uniqueToken),
		}

		conflictResponse, err := provisionHandler.Provision(ctx, request2)
		require.NoError(t, err)

		// the conflicting machine must get its own address: sharing the live link's address would
		// put two Wireguard peers on one /128 and take the address away from the connected machine
		require.NotEqual(t, linkResponse.NodeAddressPrefix, conflictResponse.NodeAddressPrefix)

		rtestutils.AssertResources(
			ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
				assertion.Equal(linkResponse.NodeAddressPrefix, r.TypedSpec().Value.NodeSubnet)
			},
		)

		rtestutils.AssertNoResource[*siderolinkres.PendingMachine](ctx, t, state, request.NodePublicKey)
		rtestutils.AssertResource(ctx, t, state, request2.NodePublicKey, func(r *siderolinkres.PendingMachine, assertion *assert.Assertions) {
			_, conflict := r.Metadata().Annotations().Get(siderolinkres.PendingMachineUUIDConflict)

			assertion.True(conflict)
			assertion.NotEqual(linkResponse.NodeAddressPrefix, r.TypedSpec().Value.NodeSubnet)
		})

		// the conflicting machine re-provisions until it is given a new UUID: its address must be
		// stable across those retries, otherwise its peer is torn down and re-created every time
		conflictResponse2, err := provisionHandler.Provision(ctx, request2)
		require.NoError(t, err)
		require.Equal(t, conflictResponse.NodeAddressPrefix, conflictResponse2.NodeAddressPrefix)
	})

	t.Run("v1 default token", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		token, err := jointoken.NewWithExtraData(validToken, jointoken.Version1, map[string]string{
			omni.LabelMachineRequest: "hi",
		})

		require.NoError(t, err)

		encoded, err := token.Encode()
		require.NoError(t, err)

		uniqueToken, err := jointoken.NewNodeUniqueToken("fingerprint", "so-unique").Encode()
		require.NoError(t, err)

		request := &pb.ProvisionRequest{
			NodeUuid:        "machine-from-provider",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: new(uniqueToken),
		}

		_, err = provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(
			ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)

				requestID, ok := r.Metadata().Labels().Get(omni.LabelMachineRequest)
				assertion.True(ok)
				assertion.Equal("hi", requestID)
			},
		)

		rtestutils.AssertNoResource[*siderolinkres.PendingMachine](ctx, t, state, request.NodePublicKey)
	})

	t.Run("provider unique token", func(t *testing.T) {
		t.Parallel()

		providerID := "test"

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		require.NoError(t, state.Create(ctx, infra.NewProvider(providerID)))

		var providerUniqueToken string

		rtestutils.AssertResources(ctx, t, state, []string{providerID}, func(cfg *siderolinkres.ProviderJoinConfig, assert *assert.Assertions) {
			assert.NotEmpty(cfg.TypedSpec().Value.JoinToken)

			providerUniqueToken = cfg.TypedSpec().Value.JoinToken
		})

		token, err := jointoken.NewWithExtraData(providerUniqueToken, jointoken.Version2, map[string]string{
			omni.LabelInfraProviderID: providerID,
		})

		require.NoError(t, err)

		encoded, err := token.Encode()
		require.NoError(t, err)

		uniqueToken, err := jointoken.NewNodeUniqueToken("fingerprint", "so-unique").Encode()
		require.NoError(t, err)

		request := &pb.ProvisionRequest{
			NodeUuid:        "machine-from-provider",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: new(uniqueToken),
		}

		_, err = provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(
			ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)

				providerID, ok := r.Metadata().Labels().Get(omni.LabelInfraProviderID)
				assertion.True(ok)
				assertion.Equal("test", providerID)
			},
		)

		rtestutils.AssertNoResource[*siderolinkres.PendingMachine](ctx, t, state, request.NodePublicKey)
	})

	t.Run("provider unique token invalid", func(t *testing.T) {
		t.Parallel()

		providerID := "test"

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		require.NoError(t, state.Create(ctx, infra.NewProvider(providerID)))

		rtestutils.AssertResources(ctx, t, state, []string{providerID}, func(cfg *siderolinkres.ProviderJoinConfig, assert *assert.Assertions) {
			assert.NotEmpty(cfg.TypedSpec().Value.JoinToken)
		})

		token, err := jointoken.NewWithExtraData("meow", jointoken.Version2, map[string]string{
			omni.LabelInfraProviderID: providerID,
		})

		require.NoError(t, err)

		encoded, err := token.Encode()
		require.NoError(t, err)

		uniqueToken, err := jointoken.NewNodeUniqueToken("fingerprint", "so-unique").Encode()
		require.NoError(t, err)

		request := &pb.ProvisionRequest{
			NodeUuid:        "machine-from-provider",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: new(uniqueToken),
		}

		_, err = provisionHandler.Provision(ctx, request)
		require.Equal(t, codes.PermissionDenied, status.Code(err))

		token, err = jointoken.NewWithExtraData(validToken, jointoken.Version2, map[string]string{
			omni.LabelInfraProviderID: "nonexistent",
		})

		require.NoError(t, err)

		encoded, err = token.Encode()
		require.NoError(t, err)

		request = &pb.ProvisionRequest{
			NodeUuid:        "machine-from-provider",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: new(uniqueToken),
		}

		_, err = provisionHandler.Provision(ctx, request)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("registration limit blocks new machines", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		t.Cleanup(cancel)

		st, _ := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		// Override the handler with one that has a limit of 2.
		provisionHandler := siderolink.NewProvisionHandler(zaptest.NewLogger(t), st, config.SiderolinkServiceJoinTokensModeStrict, false, 2)

		// Create 2 links to reach the limit.
		for _, id := range []string{"m1", "m2"} {
			require.NoError(t, st.Create(ctx, siderolinkres.NewLink(id, &specs.SiderolinkSpec{})))
		}

		uniqueToken, tokenErr := jointoken.NewNodeUniqueToken(uuid.NewString(), uuid.NewString()).Encode()
		require.NoError(t, tokenErr)

		_, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
			NodeUuid:        "m3",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.0"),
			JoinToken:       new(validToken),
			NodeUniqueToken: new(uniqueToken),
		})
		require.Error(t, err)
		require.Equal(t, codes.ResourceExhausted, status.Code(err))
		require.Contains(t, err.Error(), "2/2 machines registered")
	})

	t.Run("registration limit allows when under", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		t.Cleanup(cancel)

		st, _ := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		provisionHandler := siderolink.NewProvisionHandler(zaptest.NewLogger(t), st, config.SiderolinkServiceJoinTokensModeStrict, false, 5)

		require.NoError(t, st.Create(ctx, siderolinkres.NewLink("m1", &specs.SiderolinkSpec{})))

		uniqueToken, tokenErr := jointoken.NewNodeUniqueToken(uuid.NewString(), uuid.NewString()).Encode()
		require.NoError(t, tokenErr)

		_, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
			NodeUuid:        "m2",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.0"),
			JoinToken:       new(validToken),
			NodeUniqueToken: new(uniqueToken),
		})
		require.NoError(t, err)
	})

	t.Run("registration limit unlimited when zero", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		t.Cleanup(cancel)

		st, _ := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		provisionHandler := siderolink.NewProvisionHandler(zaptest.NewLogger(t), st, config.SiderolinkServiceJoinTokensModeStrict, false, 0)

		for _, id := range []string{"m1", "m2", "m3"} {
			require.NoError(t, st.Create(ctx, siderolinkres.NewLink(id, &specs.SiderolinkSpec{})))
		}

		uniqueToken, tokenErr := jointoken.NewNodeUniqueToken(uuid.NewString(), uuid.NewString()).Encode()
		require.NoError(t, tokenErr)

		_, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
			NodeUuid:        "m4",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.0"),
			JoinToken:       new(validToken),
			NodeUniqueToken: new(uniqueToken),
		})
		require.NoError(t, err)
	})

	t.Run("all provision logs have the machine id", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		t.Cleanup(cancel)

		st, _ := setup(ctx, t, config.SiderolinkServiceJoinTokensModeStrict)

		observedCore, observedLogs := observer.New(zapcore.DebugLevel)

		provisionHandler := siderolink.NewProvisionHandler(zap.New(observedCore), st, config.SiderolinkServiceJoinTokensModeStrict, false, 0)

		uniqueToken, tokenErr := jointoken.NewNodeUniqueToken(uuid.NewString(), uuid.NewString()).Encode()
		require.NoError(t, tokenErr)

		// rejected join: the warning about the join token should still be attributed to the machine
		_, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
			NodeUuid:        "rejected-machine",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.0"),
			JoinToken:       new("not-a-valid-token"),
			NodeUniqueToken: new(uniqueToken),
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))

		// successful join
		_, err = provisionHandler.Provision(ctx, &pb.ProvisionRequest{
			NodeUuid:        "accepted-machine",
			NodePublicKey:   genKey(),
			TalosVersion:    new("v1.9.0"),
			JoinToken:       new(validToken),
			NodeUniqueToken: new(uniqueToken),
		})
		require.NoError(t, err)

		entries := observedLogs.All()

		require.NotEmpty(t, entries)

		for _, entry := range entries {
			machine, ok := entry.ContextMap()["machine"]

			assert.True(t, ok, "log entry %q has no machine id", entry.Message)
			assert.NotEmpty(t, machine, "log entry %q has an empty machine id", entry.Message)
		}

		assert.NotEmpty(t, observedLogs.FilterField(zap.String("machine", "rejected-machine")).All())
		assert.NotEmpty(t, observedLogs.FilterField(zap.String("machine", "accepted-machine")).All())
	})
}

func genPublicKey(t *testing.T) string {
	t.Helper()

	privateKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	return privateKey.PublicKey().String()
}

// provisionFixture wires the state, the controller runtime and a provision handler the way the
// backend does: both LinkStatus controllers share a single peers pool, so a Link and a
// PendingMachine can hand a device peer over to each other.
func provisionFixture(
	ctx context.Context,
	t *testing.T,
	mode config.SiderolinkServiceJoinTokensMode,
	joinToken string,
	device siderolink.WireguardHandler,
	talosInstalled func(machine *siderolinkres.PendingMachine) bool,
) (state.State, *siderolink.ProvisionHandler) {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)

	ctx, cancel := context.WithCancel(ctx)

	rt, err := runtime.NewRuntime(st, logging.IncreaseLevel(logger, zap.InfoLevel), omniruntime.RuntimeCacheOptions()...)
	require.NoError(t, err)

	peers := siderolink.NewPeersPool(logger, device)

	require.NoError(t, rt.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.PendingMachine](peers)))
	require.NoError(t, rt.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.Link](peers)))
	require.NoError(t, rt.RegisterQController(omnictrl.NewProviderJoinConfigController()))
	require.NoError(t, rt.RegisterQController(newPendingMachineStatusController(talosInstalled)))

	siderolinkAPIconfig := siderolinkres.NewAPIConfig()
	siderolinkAPIconfig.TypedSpec().Value.EventsPort = 8081
	siderolinkAPIconfig.TypedSpec().Value.LogsPort = 8082
	siderolinkAPIconfig.TypedSpec().Value.MachineApiAdvertisedUrl = "grpc://127.0.0.1:8090"

	require.NoError(t, st.Create(ctx, siderolinkAPIconfig))

	var eg errgroup.Group

	eg.Go(func() error {
		return rt.Run(ctx)
	})

	t.Cleanup(func() {
		cancel()

		require.NoError(t, eg.Wait())
	})

	provisionHandler := siderolink.NewProvisionHandler(logger, st, mode, false, 0)

	cfg := siderolinkres.NewConfig()
	cfg.TypedSpec().Value.ServerAddress = "127.0.0.1"
	cfg.TypedSpec().Value.PublicKey = genPublicKey(t)
	cfg.TypedSpec().Value.InitialJoinToken = joinToken
	cfg.TypedSpec().Value.Subnet = wireguard.NetworkPrefix("").String()

	require.NoError(t, st.Create(ctx, cfg))

	tokenRes := siderolinkres.NewJoinToken(joinToken)
	tokenRes.TypedSpec().Value.Name = "default"

	require.NoError(t, st.Create(ctx, tokenRes))

	tokenStatusRes := siderolinkres.NewJoinTokenStatus(joinToken)
	tokenStatusRes.TypedSpec().Value.Name = "default"
	tokenStatusRes.TypedSpec().Value.IsDefault = true
	tokenStatusRes.TypedSpec().Value.State = specs.JoinTokenStatusSpec_ACTIVE
	tokenStatusRes.Metadata().Labels().Set(siderolinkres.LabelJoinTokenFingerprint, jointoken.Fingerprint(joinToken))

	require.NoError(t, st.Create(ctx, tokenStatusRes))

	defaultToken := siderolinkres.NewDefaultJoinToken()
	defaultToken.TypedSpec().Value.TokenId = joinToken

	require.NoError(t, st.Create(ctx, defaultToken))

	return st, provisionHandler
}

// testDeviceHandler models the Wireguard device closely enough for the peer lifecycle tests.
//
// Peers are identified by public key alone: an add upserts the peer and a removal deletes it
// wholesale. On top of that it models the allowed-IPs trie, where each node address is owned by
// exactly one peer -- configuring a peer for an address another peer already holds moves the
// address and leaves the previous owner with no allowed IPs, and therefore unreachable. It also
// drives wggrpc.AllowedPeers the same way the manager's peer handler does.
//
// Unlike fakeWireguardHandler it requires every spec to carry a parseable node address.
type testDeviceHandler struct {
	ownerOf   map[netip.Addr]string
	addrOf    map[string]netip.Addr
	allowed   *wggrpc.AllowedPeers
	evictions []string
	mu        sync.Mutex
}

func newTestDeviceHandler() *testDeviceHandler {
	return &testDeviceHandler{
		ownerOf: map[netip.Addr]string{},
		addrOf:  map[string]netip.Addr{},
		allowed: wggrpc.NewAllowedPeers(),
	}
}

func (h *testDeviceHandler) SetupDevice(wireguard.DeviceConfig) error { return nil }

func (h *testDeviceHandler) Shutdown() error { return nil }

func (h *testDeviceHandler) Run(ctx context.Context, _ *zap.Logger) error {
	<-ctx.Done()

	return nil
}

func (h *testDeviceHandler) Peers() ([]wgtypes.Peer, error) { return nil, nil }

func (h *testDeviceHandler) PeerEvent(_ context.Context, spec *specs.SiderolinkSpec, deleted bool) error {
	pubKey, err := wgtypes.ParseKey(spec.NodePublicKey)
	if err != nil {
		return err
	}

	prefix, err := netip.ParsePrefix(spec.NodeSubnet)
	if err != nil {
		return err
	}

	addr := prefix.Addr()

	h.mu.Lock()
	defer h.mu.Unlock()

	if deleted {
		h.allowed.RemoveToken(pubKey)

		if h.ownerOf[addr] == spec.NodePublicKey {
			delete(h.ownerOf, addr)
		}

		delete(h.addrOf, spec.NodePublicKey)

		return nil
	}

	// ReplaceAllowedIPs on a /128 that another peer already holds moves it: the previous owner is
	// left with no allowed IPs at all.
	if prev, ok := h.ownerOf[addr]; ok && prev != spec.NodePublicKey {
		delete(h.addrOf, prev)

		h.evictions = append(h.evictions, fmt.Sprintf("%s lost %s to %s", prev, addr, spec.NodePublicKey))
	}

	h.ownerOf[addr] = spec.NodePublicKey
	h.addrOf[spec.NodePublicKey] = addr

	if spec.VirtualAddrport != "" {
		addrPort, parseErr := netip.ParseAddrPort(spec.VirtualAddrport)
		if parseErr != nil {
			return parseErr
		}

		h.allowed.AddToken(pubKey, addrPort.Addr().String())
	}

	return nil
}

// owner returns the public key that currently owns the address, or "" if nobody does.
func (h *testDeviceHandler) owner(addr netip.Addr) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.ownerOf[addr]
}

// address returns the address the peer currently has an allowed-IP for.
func (h *testDeviceHandler) address(pubKey string) (netip.Addr, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	addr, ok := h.addrOf[pubKey]

	return addr, ok
}

func (h *testDeviceHandler) evictionLog() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]string(nil), h.evictions...)
}

func retryDestroy(ctx context.Context, st state.State, md *resource.Metadata) error {
	for {
		err := st.Destroy(ctx, md)
		if err == nil || state.IsNotFoundError(err) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// Regression test for the duplicate-UUID address collision.
//
// A Link is keyed by the machine UUID, and generateLinkSpec reuses the address of whatever
// record already exists for that UUID. So when a second machine joined with a UUID that was
// already taken, Omni detected the conflict and correctly parked the newcomer as a
// PendingMachine instead of overwriting the live Link -- but it still handed that pending
// machine the live machine's SideroLink address, because the conflict verdict is applied as
// an annotation only after the spec has been generated, and never reached the code picking
// the address.
//
// Both records then got a WireGuard peer for the same /128, configured with
// ReplaceAllowedIPs. WireGuard's allowed-IPs are a single cryptokey-routing trie, so the
// second peer took the address away from the first and the live, already-joined machine
// became unroutable in both directions while still reporting as connected.
//
// The test asserts the invariant: a pending machine created for a UUID conflict gets its own
// address, and no peer ever takes an allowed-IP away from another peer.
func TestUUIDConflictDoesNotStealLinkAddress(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	// Distinct fingerprints, so updateNodeUniqueToken reaches errUUIDConflict rather than
	// the same-fingerprint PermissionDenied: two different physical machines that happen to
	// report the same UUID.
	joinToken, err := jointoken.NewNodeUniqueToken(uuid.NewString(), "joinToken").Encode()
	require.NoError(t, err)

	victimToken, err := jointoken.NewNodeUniqueToken(uuid.NewString(), "victim").Encode()
	require.NoError(t, err)

	intruderToken, err := jointoken.NewNodeUniqueToken(uuid.NewString(), "intruder").Encode()
	require.NoError(t, err)

	device := newTestDeviceHandler()

	st, provisionHandler := provisionFixture(ctx, t, config.SiderolinkServiceJoinTokensModeStrict, joinToken, device, func(*siderolinkres.PendingMachine) bool {
		// both machines run installed Talos, which closes the same-fingerprint overwrite path
		return true
	})

	const dupUUID = "so-duplicate"

	victimKey := genPublicKey(t)
	intruderKey := genPublicKey(t)

	// Step 1: the victim joins normally and gets its SideroLink address. Provision blocks on
	// the LinkStatus, so the peer has already been installed when it returns.
	victimResp, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
		NodeUuid:        dupUUID,
		NodePublicKey:   victimKey,
		TalosVersion:    new("v1.9.4"),
		JoinToken:       new(joinToken),
		NodeUniqueToken: new(victimToken),
	})
	require.NoError(t, err)

	victimPrefix, err := netip.ParsePrefix(victimResp.NodeAddressPrefix)
	require.NoError(t, err)

	victimAddr := victimPrefix.Addr()

	require.Equal(t, victimKey, device.owner(victimAddr), "the victim must own its address after joining")

	// Step 2: a second, unrelated machine reporting the same UUID joins. This is not an error
	// to the caller: Omni parks it as a pending machine and resolves the conflict later.
	_, err = provisionHandler.Provision(ctx, &pb.ProvisionRequest{
		NodeUuid:        dupUUID,
		NodePublicKey:   intruderKey,
		TalosVersion:    new("v1.9.4"),
		JoinToken:       new(joinToken),
		NodeUniqueToken: new(intruderToken),
	})
	require.NoError(t, err)

	// Step 3: confirm we are on the intended conflict path, so a change that reroutes this
	// scenario makes the test fail loudly instead of passing for the wrong reason.
	victimLink, err := safe.StateGetByID[*siderolinkres.Link](ctx, st, dupUUID)
	require.NoError(t, err)
	require.Equal(t, victimKey, victimLink.TypedSpec().Value.NodePublicKey, "the victim's link must keep its own public key")

	pending, err := safe.StateGetByID[*siderolinkres.PendingMachine](ctx, st, intruderKey)
	require.NoError(t, err)

	_, conflict := pending.Metadata().Annotations().Get(siderolinkres.PendingMachineUUIDConflict)
	require.True(t, conflict, "the intruder must be marked as a UUID conflict")

	machineUUID, _ := pending.Metadata().Labels().Get(omni.MachineUUID)
	require.Equal(t, dupUUID, machineUUID)

	// Step 4: the pending machine must have its own address, so its peer cannot take the
	// victim's allowed-IP away.
	assert.NotEqual(t, victimLink.TypedSpec().Value.NodeSubnet, pending.TypedSpec().Value.NodeSubnet,
		"a pending machine created for a UUID conflict must not inherit the live link's address")
	assert.Empty(t, device.evictionLog(),
		"no peer may take an allowed-IP away from another peer")
	assert.Equal(t, victimKey, device.owner(victimAddr),
		"the victim must keep ownership of its address")

	gotAddr, ok := device.address(victimKey)
	assert.True(t, ok, "the victim's peer must still have an allowed-IP")
	assert.Equal(t, victimAddr, gotAddr, "the victim's peer must still hold its original address")

	// Step 5: the conflict does not heal itself once the pending machine is reaped. The
	// victim's Link never changed, so needsPeerUpdate never fires and its allowed-IP is
	// never restored -- the address is simply left owned by nobody.
	pendingMD := siderolinkres.NewPendingMachine(intruderKey, nil).Metadata()

	_, err = st.Teardown(ctx, pendingMD)
	require.NoError(t, err)

	require.NoError(t, retryDestroy(ctx, st, pendingMD))

	pendingLinkStatusID := siderolinkres.NewLinkStatus(siderolinkres.NewPendingMachine(intruderKey, nil)).Metadata().ID()
	rtestutils.AssertNoResource[*siderolinkres.LinkStatus](ctx, t, st, pendingLinkStatusID)

	assert.Equal(t, victimKey, device.owner(victimAddr),
		"the victim must still own its address after the pending machine is cleaned up")

	// Step 6: and none of this is visible. Link.Connected is only ever cleared by
	// pollWireguardPeers, which looks peers up by public key -- the victim's key still has a
	// peer, so it keeps reporting as connected while no traffic can reach it.
	victimLink, err = safe.StateGetByID[*siderolinkres.Link](ctx, st, dupUUID)
	require.NoError(t, err)
	assert.True(t, victimLink.TypedSpec().Value.Connected,
		"the victim reports as connected throughout, which is why this is silent in the UI")

	if evictions := device.evictionLog(); len(evictions) > 0 {
		t.Logf("allowed-IP evictions:\n%v", evictions)
	}
}

// registerJoinToken adds an extra join token the way JoinTokenStatusController would, including the
// fingerprint label the v3 provision flow resolves the token by.
func registerJoinToken(ctx context.Context, t *testing.T, st state.State, id, name string, tokenState specs.JoinTokenStatusSpec_State) {
	t.Helper()

	token := siderolinkres.NewJoinToken(id)
	token.TypedSpec().Value.Name = name

	require.NoError(t, st.Create(ctx, token))

	tokenStatus := siderolinkres.NewJoinTokenStatus(id)
	tokenStatus.TypedSpec().Value.Name = name
	tokenStatus.TypedSpec().Value.State = tokenState
	tokenStatus.Metadata().Labels().Set(siderolinkres.LabelJoinTokenFingerprint, jointoken.Fingerprint(id))

	require.NoError(t, st.Create(ctx, tokenStatus))
}

// TestProvisionWithLabels covers the v3 join tokens: the user labels they carry, and resolving the
// join token which signed them by its fingerprint rather than by the secret.
// labelsTestDefaultToken is the default join token the v3 fixtures below are built on.
const labelsTestDefaultToken = "labels-test-default-token"

func labelsTestSetup(ctx context.Context, t *testing.T) (state.State, *siderolink.ProvisionHandler) {
	t.Helper()

	return provisionFixture(
		ctx, t, config.SiderolinkServiceJoinTokensModeStrict, labelsTestDefaultToken,
		&fakeWireguardHandler{peers: map[string]wgtypes.Peer{}},
		func(*siderolinkres.PendingMachine) bool { return true },
	)
}

// labelsTestRequest builds the request a machine makes when it boots with a v3 join config.
func labelsTestRequest(t *testing.T, uuid string, token jointoken.JoinToken) *pb.ProvisionRequest {
	t.Helper()

	encoded, encodeErr := token.Encode()
	require.NoError(t, encodeErr)

	uniqueToken, tokenErr := jointoken.NewNodeUniqueToken("fingerprint", uuid).Encode()
	require.NoError(t, tokenErr)

	return &pb.ProvisionRequest{
		NodeUuid:        uuid,
		NodePublicKey:   genPublicKey(t),
		TalosVersion:    new("v1.9.4"),
		JoinToken:       &encoded,
		NodeUniqueToken: new(uniqueToken),
	}
}

// TestProvisionWithLabels covers the user labels a v3 join token carries.
func TestProvisionWithLabels(t *testing.T) {
	t.Parallel()

	// the point of the fingerprint: a token which is not the default one can carry labels
	t.Run("labels on a non-default token", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		st, provisionHandler := labelsTestSetup(ctx, t)

		otherToken := "other-join-token"
		registerJoinToken(ctx, t, st, otherToken, "other", specs.JoinTokenStatusSpec_ACTIVE)

		token, tokenErr := jointoken.NewWithLabels(otherToken, map[string]string{"env": "prod", "rack": "a12"}, nil)
		require.NoError(t, tokenErr)

		req := labelsTestRequest(t, "labeled-machine", token)

		_, err := provisionHandler.Provision(ctx, req)
		require.NoError(t, err)

		// the labels are seeded onto the user owned resource, not onto the link
		rtestutils.AssertResources(
			ctx, t, st, []string{req.NodeUuid},
			func(r *omni.MachineLabels, assertion *assert.Assertions) {
				env, ok := r.Metadata().Labels().Get("env")
				assertion.True(ok)
				assertion.Equal("prod", env)

				rack, ok := r.Metadata().Labels().Get("rack")
				assertion.True(ok)
				assertion.Equal("a12", rack)

				assertion.Empty(r.Metadata().Owner(), "must be ownerless, so the user can edit it")
			},
		)

		rtestutils.AssertResources(
			ctx, t, st, []string{req.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				_, ok := r.Metadata().Labels().Get("env")
				assertion.False(ok, "the link must not be cluttered with the user labels")
			},
		)

		// the usage has to point at the join token itself, not at the encoded blob, otherwise the
		// machine never counts towards that token's use count
		rtestutils.AssertResources(
			ctx, t, st, []string{req.NodeUuid},
			func(r *siderolinkres.JoinTokenUsage, assertion *assert.Assertions) {
				assertion.Equal(otherToken, r.TypedSpec().Value.TokenId)
			},
		)
	})

	// the token labels are a one shot seed: once the machine has them they are the user's to change
	t.Run("existing machine labels are left alone", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		st, provisionHandler := labelsTestSetup(ctx, t)

		machineID := "already-labeled-machine"

		// the user has already curated the labels of this machine
		existing := omni.NewMachineLabels(machineID)
		existing.Metadata().Labels().Set("env", "staging")

		require.NoError(t, st.Create(ctx, existing))

		token, tokenErr := jointoken.NewWithLabels(labelsTestDefaultToken, map[string]string{"env": "prod", "rack": "a12"}, nil)
		require.NoError(t, tokenErr)

		_, err := provisionHandler.Provision(ctx, labelsTestRequest(t, machineID, token))
		require.NoError(t, err)

		rtestutils.AssertResources(
			ctx, t, st, []string{machineID},
			func(r *omni.MachineLabels, assertion *assert.Assertions) {
				env, ok := r.Metadata().Labels().Get("env")
				assertion.True(ok)
				assertion.Equal("staging", env, "the user value must win over the join token")

				_, ok = r.Metadata().Labels().Get("rack")
				assertion.False(ok, "no token label may be added once the machine has its labels")
			},
		)
	})
}

// TestProvisionTokenFingerprint covers resolving which join token signed a v3 token, by the
// non-secret fingerprint it carries.
func TestProvisionTokenFingerprint(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		tokenState specs.JoinTokenStatusSpec_State
	}{
		{name: "revoked", tokenState: specs.JoinTokenStatusSpec_REVOKED},
		{name: "expired", tokenState: specs.JoinTokenStatusSpec_EXPIRED},
	} {
		t.Run("rejects a "+tt.name+" token", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
			defer cancel()

			st, provisionHandler := labelsTestSetup(ctx, t)

			inactive := "inactive-" + tt.name
			registerJoinToken(ctx, t, st, inactive, tt.name, tt.tokenState)

			token, tokenErr := jointoken.NewWithLabels(inactive, map[string]string{"env": "prod"}, nil)
			require.NoError(t, tokenErr)

			_, err := provisionHandler.Provision(ctx, labelsTestRequest(t, "machine-"+tt.name, token))
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}

	// a provider signs with its own per provider secret, which is not the token the machine is
	// handed, so a v3 provider token carries no fingerprint and still resolves by the provider ID
	t.Run("provider tokens work on v3", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		st, provisionHandler := labelsTestSetup(ctx, t)

		providerID := "test-provider"

		require.NoError(t, st.Create(ctx, infra.NewProvider(providerID)))

		var providerSecret string

		rtestutils.AssertResources(ctx, t, st, []string{providerID}, func(cfg *siderolinkres.ProviderJoinConfig, assert *assert.Assertions) {
			assert.NotEmpty(cfg.TypedSpec().Value.JoinToken)

			providerSecret = cfg.TypedSpec().Value.JoinToken
		})

		token, tokenErr := jointoken.NewWithLabels(
			providerSecret,
			map[string]string{"env": "prod"},
			map[string]string{omni.LabelInfraProviderID: providerID},
		)
		require.NoError(t, tokenErr)

		require.Equal(t, jointoken.Version3, token.Version)
		require.Empty(t, token.TokenFingerprint, "a provider token must not leak a fingerprint of the provider secret")

		req := labelsTestRequest(t, "provider-machine", token)

		_, err := provisionHandler.Provision(ctx, req)
		require.NoError(t, err)

		// the provider label still lands on the link, the way it does on v2
		rtestutils.AssertResources(
			ctx, t, st, []string{req.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				gotProvider, ok := r.Metadata().Labels().Get(omni.LabelInfraProviderID)
				assertion.True(ok)
				assertion.Equal(providerID, gotProvider)
			},
		)

		// and the user labels the v3 token carried are seeded for the user
		rtestutils.AssertResources(
			ctx, t, st, []string{req.NodeUuid},
			func(r *omni.MachineLabels, assertion *assert.Assertions) {
				env, ok := r.Metadata().Labels().Get("env")
				assertion.True(ok)
				assertion.Equal("prod", env)
			},
		)
	})

	t.Run("rejects an unknown fingerprint", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		_, provisionHandler := labelsTestSetup(ctx, t)

		token, tokenErr := jointoken.NewWithLabels("never-registered", map[string]string{"env": "prod"}, nil)
		require.NoError(t, tokenErr)

		_, err := provisionHandler.Provision(ctx, labelsTestRequest(t, "unknown-machine", token))
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	// the fingerprint is truncated, so two tokens can share one: the signature is what decides
	t.Run("a fingerprint collision resolves by the signature", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		st, provisionHandler := labelsTestSetup(ctx, t)

		realToken := "colliding-real"
		decoyToken := "colliding-decoy"

		registerJoinToken(ctx, t, st, realToken, "real", specs.JoinTokenStatusSpec_ACTIVE)
		registerJoinToken(ctx, t, st, decoyToken, "decoy", specs.JoinTokenStatusSpec_ACTIVE)

		// force both onto the same fingerprint, which is what a truncation collision looks like
		fingerprint := jointoken.Fingerprint(realToken)

		_, err := safe.StateUpdateWithConflicts(
			ctx, st, siderolinkres.NewJoinTokenStatus(decoyToken).Metadata(),
			func(res *siderolinkres.JoinTokenStatus) error {
				res.Metadata().Labels().Set(siderolinkres.LabelJoinTokenFingerprint, fingerprint)

				return nil
			},
		)
		require.NoError(t, err)

		token, tokenErr := jointoken.NewWithLabels(realToken, map[string]string{"env": "prod"}, nil)
		require.NoError(t, tokenErr)

		req := labelsTestRequest(t, "colliding-machine", token)

		_, err = provisionHandler.Provision(ctx, req)
		require.NoError(t, err)

		rtestutils.AssertResources(
			ctx, t, st, []string{req.NodeUuid},
			func(r *siderolinkres.JoinTokenUsage, assertion *assert.Assertions) {
				assertion.Equal(realToken, r.TypedSpec().Value.TokenId)
			},
		)
	})
}
