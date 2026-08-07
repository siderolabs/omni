// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Regression test for the WireGuard-over-gRPC pending-to-link handoff.
//
// The pending machine and the link it hands off to share a public key, and the WireGuard device
// tracks peers by that key alone. If the link were to get its own virtual address-port, the peers
// pool would track two entries over one device peer, and the pending machine cleanup would delete
// the live link's device peer and revoke its tunnel token. The test drives the full handoff with
// the real controllers and asserts that the peer and the token survive the cleanup.
package siderolink_test

import (
	"context"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
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
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/logging"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// tunnelDeviceHandler models the real WireGuard device: peers are identified
// by public key alone, an add upserts the peer and a removal deletes it
// wholesale. It drives wggrpc.AllowedPeers the same way the manager's peer
// handler does.
type tunnelDeviceHandler struct {
	peers   map[string]*specs.SiderolinkSpec
	allowed *wggrpc.AllowedPeers
	log     []string
	mu      sync.Mutex
}

func (h *tunnelDeviceHandler) SetupDevice(wireguard.DeviceConfig) error { return nil }

func (h *tunnelDeviceHandler) Shutdown() error { return nil }

func (h *tunnelDeviceHandler) Run(ctx context.Context, _ *zap.Logger) error {
	<-ctx.Done()

	return nil
}

func (h *tunnelDeviceHandler) PeerEvent(_ context.Context, spec *specs.SiderolinkSpec, deleted bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	pubKey, err := wgtypes.ParseKey(spec.NodePublicKey)
	if err != nil {
		return err
	}

	if deleted {
		delete(h.peers, spec.NodePublicKey)
		h.allowed.RemoveToken(pubKey)

		h.log = append(h.log, "remove "+spec.NodePublicKey+" va="+spec.VirtualAddrport)

		return nil
	}

	h.peers[spec.NodePublicKey] = spec

	if spec.VirtualAddrport != "" {
		addrPort, err := netip.ParseAddrPort(spec.VirtualAddrport)
		if err != nil {
			return err
		}

		h.allowed.AddToken(pubKey, addrPort.Addr().String())
	}

	h.log = append(h.log, "add "+spec.NodePublicKey+" va="+spec.VirtualAddrport)

	return nil
}

func (h *tunnelDeviceHandler) Peers() ([]wgtypes.Peer, error) {
	return nil, nil
}

func (h *tunnelDeviceHandler) hasPeer(pubKey string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, ok := h.peers[pubKey]

	return ok
}

func (h *tunnelDeviceHandler) eventLog() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]string(nil), h.log...)
}

// countEvents counts device events of the given kind ("add" or "remove") for the given public key.
func (h *tunnelDeviceHandler) countEvents(kind, pubKey string) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	count := 0

	for _, line := range h.log {
		if strings.HasPrefix(line, kind+" "+pubKey+" ") {
			count++
		}
	}

	return count
}

func tokenOf(t *testing.T, virtualAddrPort string) string {
	t.Helper()

	addrPort, err := netip.ParseAddrPort(virtualAddrPort)
	require.NoError(t, err)

	return addrPort.Addr().String()
}

func TestGrpcTunnelPendingHandoffKeepsLinkPeer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	validFingerprint := uuid.NewString()

	validToken, err := jointoken.NewNodeUniqueToken(validFingerprint, "validToken").Encode()
	require.NoError(t, err)

	genKey := func() string {
		privateKey, keyErr := wgtypes.GeneratePrivateKey()
		require.NoError(t, keyErr)

		return privateKey.PublicKey().String()
	}

	device := &tunnelDeviceHandler{
		peers:   map[string]*specs.SiderolinkSpec{},
		allowed: wggrpc.NewAllowedPeers(),
	}

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)

	rt, err := runtime.NewRuntime(st, logging.IncreaseLevel(logger, zap.InfoLevel), omniruntime.RuntimeCacheOptions()...)
	require.NoError(t, err)

	peers := siderolink.NewPeersPool(logger, device)

	require.NoError(t, rt.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.PendingMachine](peers)))
	require.NoError(t, rt.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.Link](peers)))
	require.NoError(t, rt.RegisterQController(newPendingMachineStatusController(func(*siderolinkres.PendingMachine) bool {
		return true
	})))

	siderolinkAPIconfig := siderolinkres.NewAPIConfig()
	siderolinkAPIconfig.TypedSpec().Value.EventsPort = 8081
	siderolinkAPIconfig.TypedSpec().Value.LogsPort = 8082
	siderolinkAPIconfig.TypedSpec().Value.MachineApiAdvertisedUrl = "grpc://127.0.0.1:8090"

	require.NoError(t, st.Create(ctx, siderolinkAPIconfig))

	runCtx, runCancel := context.WithCancel(ctx)

	var eg errgroup.Group

	eg.Go(func() error {
		return rt.Run(runCtx)
	})

	t.Cleanup(func() {
		runCancel()

		require.NoError(t, eg.Wait())
	})

	provisionHandler := siderolink.NewProvisionHandler(logger, st, config.SiderolinkServiceJoinTokensModeStrict, false, 0)

	cfg := siderolinkres.NewConfig()
	cfg.TypedSpec().Value.ServerAddress = "127.0.0.1"
	cfg.TypedSpec().Value.PublicKey = genKey()
	cfg.TypedSpec().Value.InitialJoinToken = validToken
	cfg.TypedSpec().Value.Subnet = wireguard.NetworkPrefix("").String()

	require.NoError(t, st.Create(ctx, cfg))

	tokenRes := siderolinkres.NewJoinToken(validToken)
	tokenRes.TypedSpec().Value.Name = "default"
	require.NoError(t, st.Create(ctx, tokenRes))

	tokenStatusRes := siderolinkres.NewJoinTokenStatus(validToken)
	tokenStatusRes.TypedSpec().Value.Name = "default"
	tokenStatusRes.TypedSpec().Value.IsDefault = true
	tokenStatusRes.TypedSpec().Value.State = specs.JoinTokenStatusSpec_ACTIVE
	require.NoError(t, st.Create(ctx, tokenStatusRes))

	defaultToken := siderolinkres.NewDefaultJoinToken()
	defaultToken.TypedSpec().Value.TokenId = validToken
	require.NoError(t, st.Create(ctx, defaultToken))

	nodeUUID := "tunnel-handoff-machine"
	pubKey := genKey()

	// Step 1: the machine boots from tunnel-mode boot media and provisions with
	// no node unique token yet. This creates a PendingMachine and installs its
	// peer with a fresh virtual address-port.
	resp1, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
		NodeUuid:          nodeUUID,
		NodePublicKey:     pubKey,
		TalosVersion:      new("v1.9.0"),
		JoinToken:         new(validToken),
		WireguardOverGrpc: new(true),
	})
	require.NoError(t, err)

	pendingVA := resp1.GrpcPeerAddrPort
	require.NotEmpty(t, pendingVA)

	require.True(t, device.hasPeer(pubKey), "the pending machine's device peer must exist after the first provision")
	require.True(t, device.allowed.CheckToken(tokenOf(t, pendingVA)), "the pending tunnel token must be allowed")

	// Step 2: the unique token is delivered to the machine and it re-provisions
	// carrying it. This creates the Link: same public key, inherited node
	// subnet, and the SAME virtual address-port, so the pool keeps a single
	// peer with two owners.
	resp2, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
		NodeUuid:          nodeUUID,
		NodePublicKey:     pubKey,
		TalosVersion:      new("v1.9.0"),
		JoinToken:         new(validToken),
		NodeUniqueToken:   new(validToken),
		WireguardOverGrpc: new(true),
	})
	require.NoError(t, err)

	linkVA := resp2.GrpcPeerAddrPort
	require.Equal(t, pendingVA, linkVA, "the link must keep the pending machine's virtual address-port")
	require.Equal(t, resp1.NodeAddressPrefix, resp2.NodeAddressPrefix, "the link must have inherited the pending machine's node address")

	// Wait for the link's LinkStatus: at that point the link's peer reference is registered.
	linkStatusID := siderolinkres.NewLinkStatus(siderolinkres.NewLink(nodeUUID, nil)).Metadata().ID()
	rtestutils.AssertResources(
		ctx, t, st, []resource.ID{linkStatusID},
		func(r *siderolinkres.LinkStatus, assertion *assert.Assertions) {
			assertion.Equal(linkVA, r.TypedSpec().Value.VirtualAddrport)
		},
	)

	require.True(t, device.hasPeer(pubKey))
	require.True(t, device.allowed.CheckToken(tokenOf(t, linkVA)), "the link tunnel token must be allowed")

	// Step 3: the pending machine's grace period expires and the provision
	// handler cleans it up, exactly like removePendingMachine does: teardown,
	// wait for the finalizers, destroy.
	pendingMD := siderolinkres.NewPendingMachine(pubKey, nil).Metadata()

	_, err = st.Teardown(ctx, pendingMD)
	require.NoError(t, err)

	require.NoError(t, retryDestroy(ctx, st, pendingMD))

	// The pending machine's LinkStatus is gone: its peer reference was dropped.
	pendingLinkStatusID := siderolinkres.NewLinkStatus(siderolinkres.NewPendingMachine(pubKey, nil)).Metadata().ID()
	rtestutils.AssertNoResource[*siderolinkres.LinkStatus](ctx, t, st, pendingLinkStatusID)

	// The link is untouched, and so are its device peer and tunnel token.
	rtestutils.AssertResources(
		ctx, t, st, []resource.ID{linkStatusID},
		func(r *siderolinkres.LinkStatus, assertion *assert.Assertions) {
			assertion.Equal(linkVA, r.TypedSpec().Value.VirtualAddrport)
		},
	)

	assert.True(t, device.hasPeer(pubKey), "the live link's device peer must survive the pending machine cleanup")
	assert.True(t, device.allowed.CheckToken(tokenOf(t, linkVA)), "the link's tunnel token must stay allowed")

	// Step 4: a later re-provision with the same key keeps the virtual
	// address-port, so the peer is not torn down and re-created.
	resp3, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
		NodeUuid:          nodeUUID,
		NodePublicKey:     pubKey,
		TalosVersion:      new("v1.9.0"),
		JoinToken:         new(validToken),
		NodeUniqueToken:   new(validToken),
		WireguardOverGrpc: new(true),
	})
	require.NoError(t, err)
	require.Equal(t, linkVA, resp3.GrpcPeerAddrPort, "a re-provision with the same key must keep the virtual address-port")

	assert.True(t, device.hasPeer(pubKey))
	assert.True(t, device.allowed.CheckToken(tokenOf(t, linkVA)))

	// Across handoff, cleanup, and same-key re-provision the device must have
	// seen exactly ONE add and NO removal for the key: the peer is never torn
	// down and re-created. The settle delay lets any in-flight reconcile land
	// before the negative assertion.
	time.Sleep(250 * time.Millisecond)

	assert.Equal(t, 1, device.countEvents("add", pubKey), "exactly one device add for the key, ever")
	assert.Zero(t, device.countEvents("remove", pubKey), "no device removal for the key through handoff, cleanup and re-provision")

	// Step 5: the machine reboots. Talos generates a fresh WireGuard key on
	// every boot, so the re-provision carries a new key: it must get a fresh
	// virtual address-port, the peer must be re-created under the new key, the
	// old token revoked and the new one allowed.
	rotatedKey := genKey()

	resp4, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
		NodeUuid:          nodeUUID,
		NodePublicKey:     rotatedKey,
		TalosVersion:      new("v1.9.0"),
		JoinToken:         new(validToken),
		NodeUniqueToken:   new(validToken),
		WireguardOverGrpc: new(true),
	})
	require.NoError(t, err)

	rotatedVA := resp4.GrpcPeerAddrPort
	require.NotEmpty(t, rotatedVA)
	require.NotEqual(t, linkVA, rotatedVA, "a key rotation must produce a fresh virtual address-port")

	rtestutils.AssertResources(
		ctx, t, st, []resource.ID{linkStatusID},
		func(r *siderolinkres.LinkStatus, assertion *assert.Assertions) {
			assertion.Equal(rotatedVA, r.TypedSpec().Value.VirtualAddrport)
			assertion.Equal(rotatedKey, r.TypedSpec().Value.NodePublicKey)
		},
	)

	assert.True(t, device.hasPeer(rotatedKey), "the rotated key's device peer must exist")
	assert.False(t, device.hasPeer(pubKey), "the old key's device peer must be gone")
	assert.True(t, device.allowed.CheckToken(tokenOf(t, rotatedVA)), "the rotated key's tunnel token must be allowed")
	assert.False(t, device.allowed.CheckToken(tokenOf(t, linkVA)), "the old tunnel token must be revoked")

	// Step 6: tunnel mode switched off: the virtual address-port is cleared,
	// not kept.
	resp5, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
		NodeUuid:        nodeUUID,
		NodePublicKey:   rotatedKey,
		TalosVersion:    new("v1.9.0"),
		JoinToken:       new(validToken),
		NodeUniqueToken: new(validToken),
	})
	require.NoError(t, err)
	require.Empty(t, resp5.GrpcPeerAddrPort, "disabling the tunnel must clear the virtual address-port")

	// Step 7: tunnel mode switched back on: a FRESH virtual address-port is
	// generated, the old one is not resurrected.
	resp6, err := provisionHandler.Provision(ctx, &pb.ProvisionRequest{
		NodeUuid:          nodeUUID,
		NodePublicKey:     rotatedKey,
		TalosVersion:      new("v1.9.0"),
		JoinToken:         new(validToken),
		NodeUniqueToken:   new(validToken),
		WireguardOverGrpc: new(true),
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp6.GrpcPeerAddrPort)
	require.NotEqual(t, rotatedVA, resp6.GrpcPeerAddrPort, "re-enabling the tunnel must generate a fresh virtual address-port")

	t.Logf("device event log:\n%s", eventLogString(device))
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

func eventLogString(device *tunnelDeviceHandler) string {
	return strings.Join(device.eventLog(), "\n")
}
