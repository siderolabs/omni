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

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/google/uuid"
	"github.com/siderolabs/go-pointer"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
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

//nolint:maintidx
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
		privateKey, err := wgtypes.GeneratePrivateKey()
		require.NoError(t, err)

		return privateKey.PublicKey().String()
	}

	setup := func(ctx context.Context, t *testing.T, mode string) (state.State, *siderolink.ProvisionHandler) {
		state := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)

		ctx, cancel := context.WithCancel(ctx)

		runtime, err := runtime.NewRuntime(state, logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel)))
		require.NoError(t, err)

		peers := siderolink.NewPeersPool(logger, &fakeWireguardHandler{peers: map[string]wgtypes.Peer{}})

		require.NoError(t, runtime.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.PendingMachine](peers)))
		require.NoError(t, runtime.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.Link](peers)))
		require.NoError(t, runtime.RegisterQController(omnictrl.NewProviderJoinConfigController()))
		require.NoError(t, runtime.RegisterQController(newPendingMachineStatusController(func(*siderolinkres.PendingMachine) bool {
			return true
		})))

		siderolinkAPIconfig := siderolinkres.NewAPIConfig()
		siderolinkAPIconfig.TypedSpec().Value.EventsPort = 8081
		siderolinkAPIconfig.TypedSpec().Value.LogsPort = 8082
		siderolinkAPIconfig.TypedSpec().Value.MachineApiAdvertisedUrl = "grpc://127.0.0.1:8090"

		require.NoError(t, state.Create(ctx, siderolinkAPIconfig))

		var eg errgroup.Group

		eg.Go(func() error {
			return runtime.Run(ctx)
		})

		t.Cleanup(func() {
			cancel()

			require.NoError(t, eg.Wait())
		})

		provisionHandler := siderolink.NewProvisionHandler(logger, state, mode, false)

		config := siderolinkres.NewConfig()
		config.TypedSpec().Value.ServerAddress = "127.0.0.1"
		config.TypedSpec().Value.PublicKey = genKey()
		config.TypedSpec().Value.InitialJoinToken = validToken
		config.TypedSpec().Value.Subnet = wireguard.NetworkPrefix("").String()

		require.NoError(t, state.Create(ctx, config))

		tokenRes := siderolinkres.NewJoinToken(resources.DefaultNamespace, validToken)

		tokenRes.TypedSpec().Value.Name = "default"

		require.NoError(t, state.Create(ctx, tokenRes))

		tokenStatusRes := siderolinkres.NewJoinTokenStatus(resources.DefaultNamespace, validToken)

		tokenStatusRes.TypedSpec().Value.Name = "default"
		tokenStatusRes.TypedSpec().Value.IsDefault = true
		tokenStatusRes.TypedSpec().Value.State = specs.JoinTokenStatusSpec_ACTIVE

		require.NoError(t, state.Create(ctx, tokenStatusRes))

		defaultToken := siderolinkres.NewDefaultJoinToken()
		defaultToken.TypedSpec().Value.TokenId = validToken

		require.NoError(t, state.Create(ctx, defaultToken))

		return state, provisionHandler
	}

	t.Run("full flow", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.JoinTokensModeStrict)

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

		request.NodeUniqueToken = pointer.To(validToken)

		response, err = provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.NotEmpty(r.TypedSpec().Value.NodeSubnet)
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
			},
		)

		rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
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

			state, provisionHandler := setup(ctx, t, config.JoinTokensModeStrict)

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

			request.NodeUniqueToken = &validToken

			_, err = provisionHandler.Provision(ctx, request)
			require.NoError(t, err)

			rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
				func(r *siderolinkres.Link, assertion *assert.Assertions) {
					assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
				},
			)
		})
	}

	for _, mode := range []struct {
		name string
		mode string
	}{
		{
			name: "legacy",
			mode: config.JoinTokensModeLegacyAllowed,
		},
		{
			name: "normal",
			mode: config.JoinTokensModeStrict,
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
					NodeUniqueToken: pointer.To(validFingerprintOnly),
					TalosVersion:    pointer.To("v1.6.0"),
					JoinToken:       pointer.To(validToken),
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
					NodeUniqueToken: pointer.To(validFingerprintOnly),
					TalosVersion:    pointer.To("v1.6.0"),
					JoinToken:       pointer.To(validToken),
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
					NodeUniqueToken: pointer.To(validFingerprintOnly),
					TalosVersion:    pointer.To("v1.6.0"),
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
					NodeUniqueToken: pointer.To(validToken),
					TalosVersion:    pointer.To("v1.6.0"),
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
					NodeUniqueToken: pointer.To(validToken),
					TalosVersion:    pointer.To("v1.6.0"),
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
					NodeUniqueToken: pointer.To(invalidToken),
					TalosVersion:    pointer.To("v1.9.0"),
					JoinToken:       pointer.To(validToken),
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
					TalosVersion:  pointer.To("v1.9.0"),
					JoinToken:     pointer.To(validToken),
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
					if mode.mode == config.JoinTokensModeStrict {
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
					if mode.mode == config.JoinTokensModeLegacyAllowed {
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
					link := siderolinkres.NewLink(resources.DefaultNamespace, machine, tt.linkSpec)
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

		state, provisionHandler := setup(ctx, t, config.JoinTokensModeLegacyAllowed)

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

	t.Run("restrict legacy join", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.JoinTokensModeLegacyAllowed)

		request := &pb.ProvisionRequest{
			NodeUuid:      "machine-legacy",
			NodePublicKey: genKey(),
			TalosVersion:  pointer.To("v1.5.0"),
			JoinToken:     pointer.To(validToken),
		}

		link := siderolinkres.NewLink(resources.DefaultNamespace, request.NodeUuid, &specs.SiderolinkSpec{})

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

		state, provisionHandler := setup(ctx, t, config.JoinTokensModeStrict)

		uniqueToken, err := jointoken.NewNodeUniqueToken(uuid.NewString(), uuid.NewString()).Encode()
		require.NoError(t, err)

		request := &pb.ProvisionRequest{
			NodeUuid:        "so-duplicate",
			NodePublicKey:   genKey(),
			TalosVersion:    pointer.To("v1.9.4"),
			JoinToken:       pointer.To(validToken),
			NodeUniqueToken: pointer.To(uniqueToken),
		}

		_, err = provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		uniqueToken, err = jointoken.NewNodeUniqueToken(uuid.NewString(), uuid.NewString()).Encode()
		require.NoError(t, err)

		request2 := &pb.ProvisionRequest{
			NodeUuid:        "so-duplicate",
			NodePublicKey:   genKey(),
			TalosVersion:    pointer.To("v1.9.4"),
			JoinToken:       pointer.To(validToken),
			NodeUniqueToken: pointer.To(uniqueToken),
		}

		_, err = provisionHandler.Provision(ctx, request2)
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
			func(r *siderolinkres.Link, assertion *assert.Assertions) {
				assertion.Equal(r.TypedSpec().Value.NodePublicKey, request.NodePublicKey)
			},
		)

		rtestutils.AssertNoResource[*siderolinkres.PendingMachine](ctx, t, state, request.NodePublicKey)
		rtestutils.AssertResource(ctx, t, state, request2.NodePublicKey, func(r *siderolinkres.PendingMachine, assertion *assert.Assertions) {
			_, conflict := r.Metadata().Annotations().Get(siderolinkres.PendingMachineUUIDConflict)

			assertion.True(conflict)
		})
	})

	t.Run("v1 default token", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.JoinTokensModeStrict)

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
			TalosVersion:    pointer.To("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: pointer.To(uniqueToken),
		}

		_, err = provisionHandler.Provision(ctx, request)
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, state, []string{request.NodeUuid},
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

		state, provisionHandler := setup(ctx, t, config.JoinTokensModeStrict)

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
			TalosVersion:    pointer.To("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: pointer.To(uniqueToken),
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

	t.Run("provider unique token invalid", func(t *testing.T) {
		t.Parallel()

		providerID := "test"

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		defer cancel()

		state, provisionHandler := setup(ctx, t, config.JoinTokensModeStrict)

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
			TalosVersion:    pointer.To("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: pointer.To(uniqueToken),
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
			TalosVersion:    pointer.To("v1.9.4"),
			JoinToken:       &encoded,
			NodeUniqueToken: pointer.To(uniqueToken),
		}

		_, err = provisionHandler.Provision(ctx, request)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}
