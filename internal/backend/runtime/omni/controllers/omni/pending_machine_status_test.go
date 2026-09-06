// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/pkg/config"
	siderolinkpkg "github.com/siderolabs/omni/internal/pkg/siderolink"
)

type PendingMachineStatusSuite struct {
	OmniSuite
}

func (suite *PendingMachineStatusSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)

	defer cancel()

	machineServices := map[string]*machineService{}

	knownUUIDs := map[string]struct{}{}

	createPendingMachine := func(name, uuid string) *siderolink.PendingMachine {
		ms, err := suite.newServer(name)
		suite.Require().NoError(err)

		_, conflict := knownUUIDs[uuid]

		knownUUIDs[uuid] = struct{}{}

		machineServices[name] = ms

		pendingMachine := siderolink.NewPendingMachine(name, &specs.SiderolinkSpec{})
		pendingMachine.Metadata().Labels().Set(omni.MachineUUID, uuid)

		if conflict {
			pendingMachine.Metadata().Annotations().Set(siderolink.PendingMachineUUIDConflict, "")
		}

		pendingMachine.TypedSpec().Value.NodeSubnet = unixSocket + suite.socketPath + name

		suite.Require().NoError(suite.state.Create(ctx, pendingMachine))

		return pendingMachine
	}

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))

	defaultUUID := "uuid"

	type machine struct {
		link            *specs.SiderolinkSpec
		nodeUniqueToken string
		uuid            string
	}

	machines := []machine{
		{
			uuid: defaultUUID,
		},
		{
			uuid: defaultUUID,
		},
		{
			uuid: defaultUUID,
		},
		{
			uuid: defaultUUID,
		},
		{
			uuid: "non-conflict",
			link: &specs.SiderolinkSpec{},
		},
		{
			uuid:            "conflict-link",
			link:            &specs.SiderolinkSpec{},
			nodeUniqueToken: "abcdefg",
		},
	}

	awaitMachines := make([]string, 0, len(machines))

	links := make([]*siderolink.LinkStatus, 0, len(machines))

	for index, m := range machines {
		if m.nodeUniqueToken != "" {
			nodeUniqueToken := siderolink.NewNodeUniqueToken(m.uuid)
			nodeUniqueToken.TypedSpec().Value.Token = m.nodeUniqueToken

			suite.Require().NoError(suite.state.Create(ctx, nodeUniqueToken))
		}

		pm := createPendingMachine(fmt.Sprintf("p%d", index), m.uuid)

		awaitMachines = append(awaitMachines, pm.Metadata().ID())

		links = append(links, siderolink.NewLinkStatus(pm))

		if m.link != nil {
			suite.Require().NoError(suite.state.Create(ctx, siderolink.NewLink(m.uuid, m.link)))
		}
	}

	for _, link := range links {
		suite.Require().NoError(suite.state.Create(ctx, link))
	}

	uuidCounts := map[string]map[string]struct{}{}

	rtestutils.AssertResources(
		suite.ctx, suite.T(), suite.state, awaitMachines,
		func(ms *siderolink.PendingMachineStatus, assert *assert.Assertions) {
			assert.NotEmpty(ms.TypedSpec().Value.Token)

			uuid, ok := ms.Metadata().Annotations().Get(omni.MachineUUID)
			assert.True(ok)

			if uuidCounts[uuid] == nil {
				uuidCounts[uuid] = map[string]struct{}{}
			}

			uuidCounts[uuid][ms.Metadata().ID()] = struct{}{}
		},
	)

	for id, c := range uuidCounts {
		suite.Require().Equal(1, len(c), "uuid %s is duplicate", id)
	}

	tokens := map[string]struct{}{}

	for name, machineService := range machineServices {
		metaKeys := machineService.getMetaKeys()

		token, ok := metaKeys[meta.UniqueMachineToken]

		suite.Assert().True(ok, "no unique token, machine %s", name)
		suite.Assert().NotEmpty(token, "empty unique token, machine %s", name)

		tokens[token] = struct{}{}
	}

	// the machines with the same UUID are different physical machines, so they must not share a token
	suite.Assert().Len(tokens, len(machineServices))

	metaKeys := machineServices["p5"].getMetaKeys()
	suite.Assert().NotEqual("conflict-link", metaKeys[meta.UUIDOverride])

	metaKeys = machineServices["p4"].getMetaKeys()
	suite.Assert().NotContains(metaKeys, meta.UUIDOverride)
}

// TestUUIDConflictGeneratedOnce ensures the override UUID is generated exactly once for a conflicting
// pending machine, even if the controller reconciles many times while the conflict annotation is set.
//
// Regenerating the override on every reconcile makes the machine re-join under multiple UUIDs, which
// produces duplicate Link resources (same public key and node subnet) for a single physical machine.
func (suite *PendingMachineStatusSuite) TestUUIDConflictGeneratedOnce() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))

	const (
		name        = "conflict-once"
		machineUUID = "dup-uuid"
	)

	ms, err := suite.newServer(name)
	suite.Require().NoError(err)

	pendingMachine := siderolink.NewPendingMachine(name, &specs.SiderolinkSpec{})
	pendingMachine.Metadata().Labels().Set(omni.MachineUUID, machineUUID)
	pendingMachine.Metadata().Annotations().Set(siderolink.PendingMachineUUIDConflict, "")
	pendingMachine.TypedSpec().Value.NodeSubnet = unixSocket + suite.socketPath + name

	suite.Require().NoError(suite.state.Create(ctx, pendingMachine))
	suite.Require().NoError(suite.state.Create(ctx, siderolink.NewLinkStatus(pendingMachine)))

	// wait until the controller injects a freshly generated override UUID
	var overrideUUID string

	rtestutils.AssertResource(
		ctx, suite.T(), suite.state, name,
		func(pms *siderolink.PendingMachineStatus, assert *assert.Assertions) {
			id, ok := pms.Metadata().Annotations().Get(omni.MachineUUID)
			assert.True(ok)
			assert.NotEqual(machineUUID, id)

			overrideUUID = id
		},
	)

	suite.Require().NotEmpty(overrideUUID)
	suite.Require().Equal(overrideUUID, ms.getMetaKeys()[meta.UUIDOverride])

	// force repeated reconciles of the same pending machine, mirroring the per-provision touches that
	// happen in production while the conflict annotation is still present
	for i := range 10 {
		_, err = safe.StateUpdateWithConflicts(ctx, suite.state, pendingMachine.Metadata(), func(res *siderolink.PendingMachine) error {
			res.Metadata().Annotations().Set("reconcile-trigger", fmt.Sprintf("%d", i))

			return nil
		})
		suite.Require().NoError(err)
	}

	// the override UUID must be written exactly once and never change
	suite.Assert().Never(func() bool {
		return ms.getMetaWriteCount(meta.UUIDOverride) != 1
	}, time.Second*2, time.Millisecond*50)

	suite.Assert().Equal(overrideUUID, ms.getMetaKeys()[meta.UUIDOverride])
}

// TestTokenReusedByLaterPendingMachine checks that a pending machine created for a machine which already holds a token reuses it.
// The pending machine status is ephemeral, so the token must be recovered from the machine itself.
func (suite *PendingMachineStatusSuite) TestTokenReusedByLaterPendingMachine() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	ms, err := suite.newServer("token-reuse")
	require.NoError(t, err)

	key, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pubkey := key.PublicKey().String()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{LogLevel: new(zapcore.ErrorLevel)},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			createPending := func(id string) string {
				pending := siderolink.NewPendingMachine(id, &specs.SiderolinkSpec{NodeSubnet: unixSocket + ms.address})
				pending.Metadata().Labels().Set(omni.MachineUUID, "machine")
				require.NoError(t, tc.State.Create(ctx, pending))
				require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(pending)))

				var token string

				rtestutils.AssertResource(ctx, t, tc.State, id, func(res *siderolink.PendingMachineStatus, a *assert.Assertions) {
					a.NotEmpty(res.TypedSpec().Value.Token)

					token = res.TypedSpec().Value.Token
				})

				return token
			}

			first := createPending("first-key")
			require.Equal(t, first, ms.getMetaKeys()[meta.UniqueMachineToken])

			accepted := siderolink.NewNodeUniqueToken("machine")
			accepted.TypedSpec().Value.Token = first
			require.NoError(t, tc.State.Create(ctx, accepted))

			link := siderolink.NewLink("machine", &specs.SiderolinkSpec{NodePublicKey: pubkey, NodeSubnet: "fdae:41e4:649b:9303::2/128"})
			link.Metadata().Annotations().Set(siderolink.ForceValidNodeUniqueToken, "")
			require.NoError(t, tc.State.Create(ctx, link))

			// the machine re-joins with a new key, the token on the machine must stay the same
			second := createPending(pubkey)
			require.Equal(t, first, second)
			require.Equal(t, first, ms.getMetaKeys()[meta.UniqueMachineToken])
			require.Equal(t, 2, ms.getMetaWriteCount(meta.UniqueMachineToken))

			createProvisionPrerequisites(ctx, t, tc.State)
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(link)))

			handler := siderolinkpkg.NewProvisionHandler(tc.Logger, tc.State, config.SiderolinkServiceJoinTokensModeStrict, false, 0)
			_, err = handler.Provision(ctx, &pb.ProvisionRequest{
				NodeUuid: "machine", NodePublicKey: pubkey, NodeUniqueToken: new(second), JoinToken: new("join"), TalosVersion: new("v1.12.0"),
			})
			require.NoError(t, err)

			stored, err := safe.ReaderGetByID[*siderolink.NodeUniqueToken](ctx, tc.State, "machine")
			require.NoError(t, err)
			require.Equal(t, first, stored.TypedSpec().Value.Token)
		})
}

// TestUUIDConflictKeepsExistingToken checks the UUID conflict resolution of an installed machine which already holds a token.
//
// Talos re-joins with the new UUID and its existing token as soon as the UUID override is written, so the token must not change after that.
// Otherwise the existing token gets registered under the new UUID and the machine is rejected with the token Omni wrote later.
func (suite *PendingMachineStatusSuite) TestUUIDConflictKeepsExistingToken() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	ms, err := suite.newServer("token-uuid")
	require.NoError(t, err)

	ms.systemDisk = true

	key, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pubkey := key.PublicKey().String()

	existing, err := jointoken.NewNodeUniqueToken(fmt.Sprintf("%x", sha256.Sum256(nil)), "already-on-machine").Encode()
	require.NoError(t, err)

	ms.metaKeys = map[uint32]string{meta.UniqueMachineToken: existing}

	metaKey := talosruntime.NewMetaKey(talosruntime.NamespaceName, talosruntime.MetaKeyTagToID(meta.UniqueMachineToken))
	metaKey.TypedSpec().Value = existing
	require.NoError(t, ms.state.Create(ctx, metaKey))

	uuidWritten := make(chan string, 1)
	watchStarted := make(chan struct{})

	ms.metaWriteHook = func(req *machine.MetaWriteRequest) error {
		switch req.Key {
		case meta.UUIDOverride:
			uuidWritten <- string(req.Value)
		case meta.UniqueMachineToken:
			// Talos has already re-joined with the new UUID and is waiting for the pending machine status.
			select {
			case <-watchStarted:
			case <-ctx.Done():
			}
		}

		return nil
	}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{LogLevel: new(zapcore.ErrorLevel)},
		func(ctx context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))

			createProvisionPrerequisites(ctx, t, tc.State)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			pending := siderolink.NewPendingMachine(pubkey, &specs.SiderolinkSpec{NodeSubnet: unixSocket + ms.address})
			pending.Metadata().Labels().Set(omni.MachineUUID, "conflicting-uuid")
			pending.Metadata().Annotations().Set(siderolink.PendingMachineUUIDConflict, "")
			require.NoError(t, tc.State.Create(ctx, pending))
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(pending)))

			var newUUID string

			select {
			case newUUID = <-uuidWritten:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(siderolink.NewLink(newUUID, nil))))

			handler := siderolinkpkg.NewProvisionHandler(tc.Logger, &statusWatchState{State: tc.State, watchStarted: watchStarted}, config.SiderolinkServiceJoinTokensModeStrict, false, 0)
			request := &pb.ProvisionRequest{
				NodeUuid: newUUID, NodePublicKey: pubkey, NodeUniqueToken: new(existing), JoinToken: new("join"), TalosVersion: new("v1.12.0"),
			}

			_, err := handler.Provision(ctx, request)
			require.NoError(t, err)

			pendingStatus, err := safe.ReaderGetByID[*siderolink.PendingMachineStatus](ctx, tc.State, pubkey)
			require.NoError(t, err)
			require.Equal(t, existing, pendingStatus.TypedSpec().Value.Token)
			require.Equal(t, existing, ms.getMetaKeys()[meta.UniqueMachineToken])

			accepted, err := safe.ReaderGetByID[*siderolink.NodeUniqueToken](ctx, tc.State, newUUID)
			require.NoError(t, err)
			require.Equal(t, existing, accepted.TypedSpec().Value.Token)

			link, err := safe.ReaderGetByID[*siderolink.Link](ctx, tc.State, newUUID)
			require.NoError(t, err)

			_, enforced := link.Metadata().Annotations().Get(siderolink.ForceValidNodeUniqueToken)
			require.True(t, enforced)

			// the machine re-joins with the token from META, which must be accepted without a warning
			logCore, observedLogs := observer.New(zap.WarnLevel)
			handler = siderolinkpkg.NewProvisionHandler(zap.New(logCore), tc.State, config.SiderolinkServiceJoinTokensModeStrict, false, 0)

			request.NodeUniqueToken = new(ms.getMetaKeys()[meta.UniqueMachineToken])

			_, err = handler.Provision(ctx, request)
			require.NoError(t, err)
			require.Zero(t, observedLogs.Len())

			require.Equal(t, 1, ms.getMetaWriteCount(meta.UUIDOverride))
			require.Equal(t, 1, ms.getMetaWriteCount(meta.UniqueMachineToken))
		})
}

// TestUUIDConflictOverrideKeptAfterFailedTokenWrite checks that a failed reconcile does not make the machine re-join under yet another UUID.
//
// The UUID override is written before the token, and the pending machine status is not saved when the token write fails,
// so the next reconcile must reuse the override UUID already written to the machine.
func (suite *PendingMachineStatusSuite) TestUUIDConflictOverrideKeptAfterFailedTokenWrite() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	ms, err := suite.newServer("uuid-retry")
	require.NoError(t, err)

	var failed atomic.Bool

	ms.metaWriteHook = func(req *machine.MetaWriteRequest) error {
		if req.Key == meta.UniqueMachineToken && failed.CompareAndSwap(false, true) {
			return status.Error(codes.Unavailable, "reply lost")
		}

		return nil
	}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{LogLevel: new(zapcore.ErrorLevel)},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			pending := siderolink.NewPendingMachine("uuid-retry", &specs.SiderolinkSpec{NodeSubnet: unixSocket + ms.address})
			pending.Metadata().Labels().Set(omni.MachineUUID, "conflicting-uuid")
			pending.Metadata().Annotations().Set(siderolink.PendingMachineUUIDConflict, "")
			require.NoError(t, tc.State.Create(ctx, pending))
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(pending)))

			rtestutils.AssertResource(ctx, t, tc.State, "uuid-retry", func(res *siderolink.PendingMachineStatus, a *assert.Assertions) {
				a.NotEmpty(res.TypedSpec().Value.Token)
				a.Equal(ms.getMetaKeys()[meta.UniqueMachineToken], res.TypedSpec().Value.Token)

				id, _ := res.Metadata().Annotations().Get(omni.MachineUUID)
				a.Equal(ms.getMetaKeys()[meta.UUIDOverride], id)
			})

			require.Equal(t, 1, ms.getMetaWriteCount(meta.UUIDOverride))
			require.Equal(t, 2, ms.getMetaWriteCount(meta.UniqueMachineToken))
		})
}

// TestUUIDConflictStaleOverrideReplaced checks that an override UUID left on the machine by an earlier conflict is not reused
// when the machine is in conflict under that very UUID, as it cannot resolve the conflict.
func (suite *PendingMachineStatusSuite) TestUUIDConflictStaleOverrideReplaced() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	ms, err := suite.newServer("stale-override")
	require.NoError(t, err)

	const staleOverride = "stale-override-uuid"

	ms.metaKeys = map[uint32]string{meta.UUIDOverride: staleOverride}

	metaKey := talosruntime.NewMetaKey(talosruntime.NamespaceName, talosruntime.MetaKeyTagToID(meta.UUIDOverride))
	metaKey.TypedSpec().Value = staleOverride
	require.NoError(t, ms.state.Create(ctx, metaKey))

	testutils.WithRuntime(ctx, t, testutils.TestOptions{LogLevel: new(zapcore.ErrorLevel)},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			pending := siderolink.NewPendingMachine("stale-override", &specs.SiderolinkSpec{NodeSubnet: unixSocket + ms.address})
			pending.Metadata().Labels().Set(omni.MachineUUID, staleOverride)
			pending.Metadata().Annotations().Set(siderolink.PendingMachineUUIDConflict, "")
			require.NoError(t, tc.State.Create(ctx, pending))
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(pending)))

			rtestutils.AssertResource(ctx, t, tc.State, "stale-override", func(res *siderolink.PendingMachineStatus, a *assert.Assertions) {
				id, _ := res.Metadata().Annotations().Get(omni.MachineUUID)
				a.NotEmpty(id)
				a.NotEqual(staleOverride, id)
				a.Equal(ms.getMetaKeys()[meta.UUIDOverride], id)
			})

			require.Equal(t, 1, ms.getMetaWriteCount(meta.UUIDOverride))
		})
}

// TestFirstJoinWaitsForToken checks the normal first join of an installed machine: the token-bearing join waits for the pending machine status,
// and the token is written exactly once.
func (suite *PendingMachineStatusSuite) TestFirstJoinWaitsForToken() {
	suite.firstJoin(false)
}

// TestFirstJoinLostReplyKeepsToken checks the first join when Talos applies the token but the META write call fails.
// Talos already re-joins with the applied token, so the retry must write the same token again instead of generating a new one.
func (suite *PendingMachineStatusSuite) TestFirstJoinLostReplyKeepsToken() {
	suite.firstJoin(true)
}

func (suite *PendingMachineStatusSuite) firstJoin(failFirst bool) {
	t := suite.T()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	ms, err := suite.newServer("first-join")
	require.NoError(t, err)

	ms.systemDisk = true

	key, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pubkey := key.PublicKey().String()
	watchStarted := make(chan struct{})
	writes := make(chan string, 8)

	var first atomic.Bool

	first.Store(true)

	ms.metaWriteHook = func(req *machine.MetaWriteRequest) error {
		if req.Key != meta.UniqueMachineToken {
			return nil
		}

		writes <- string(req.Value)

		if !first.CompareAndSwap(true, false) {
			return nil
		}

		// Talos re-joins with the applied token before the write call returns.
		select {
		case <-watchStarted:
		case <-ctx.Done():
			return ctx.Err()
		}

		if failFirst {
			return status.Error(codes.Unavailable, "reply lost")
		}

		return nil
	}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{LogLevel: new(zapcore.ErrorLevel)},
		func(ctx context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))

			createProvisionPrerequisites(ctx, t, tc.State)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			pending := siderolink.NewPendingMachine(pubkey, &specs.SiderolinkSpec{NodeSubnet: unixSocket + ms.address})
			pending.Metadata().Labels().Set(omni.MachineUUID, "first-join")
			require.NoError(t, tc.State.Create(ctx, pending))
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(pending)))
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(siderolink.NewLink("first-join", nil))))

			var token string

			select {
			case token = <-writes:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			handler := siderolinkpkg.NewProvisionHandler(tc.Logger, &statusWatchState{State: tc.State, watchStarted: watchStarted}, config.SiderolinkServiceJoinTokensModeStrict, false, 0)
			_, err := handler.Provision(ctx, &pb.ProvisionRequest{
				NodeUuid: "first-join", NodePublicKey: pubkey, NodeUniqueToken: new(token), JoinToken: new("join"), TalosVersion: new("v1.12.0"),
			})
			require.NoError(t, err)

			stored, err := safe.ReaderGetByID[*siderolink.NodeUniqueToken](ctx, tc.State, "first-join")
			require.NoError(t, err)
			require.Equal(t, token, stored.TypedSpec().Value.Token)
			require.Equal(t, token, ms.getMetaKeys()[meta.UniqueMachineToken])

			if !failFirst {
				require.Equal(t, 1, ms.getMetaWriteCount(meta.UniqueMachineToken))

				return
			}

			select {
			case replayed := <-writes:
				require.Equal(t, token, replayed)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			require.Equal(t, 2, ms.getMetaWriteCount(meta.UniqueMachineToken))
		})
}

// TestMigrationAndNewBootShareToken checks a registered machine without a token which reboots with a new WireGuard key.
//
// The new boot joins without a token and creates a pending machine under the new key, while the legacy token migration
// creates another pending machine under the key of the existing link. Both point to the same machine and reconcile concurrently,
// so they must agree on a single token.
func (suite *PendingMachineStatusSuite) TestMigrationAndNewBootShareToken() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	ms, err := suite.newServer("migration-reboot")
	require.NoError(t, err)

	ms.systemDisk = true

	oldKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	newKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	oldPublicKey, newPublicKey := oldKey.PublicKey().String(), newKey.PublicKey().String()

	candidates := make(chan string, 2)
	written := make(chan string, 2)
	allowWrites := make(chan struct{})
	reconciles := make(chan struct{}, 8)

	var writeCount atomic.Int32

	// every reconcile lists the disks before it gets to the token, so two calls mean both pending machines are being reconciled
	ms.disksHook = func() {
		reconciles <- struct{}{}
	}

	// hold the first token write until the second pending machine is being reconciled, so that the token operations overlap
	ms.metaWriteBeforeHook = func(req *machine.MetaWriteRequest) error {
		if req.Key != meta.UniqueMachineToken {
			return nil
		}

		if writeCount.Add(1) > 2 {
			return fmt.Errorf("unexpected token write")
		}

		candidates <- string(req.Value)

		select {
		case <-allowWrites:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	ms.metaWriteHook = func(req *machine.MetaWriteRequest) error {
		if req.Key == meta.UniqueMachineToken {
			written <- string(req.Value)
		}

		return nil
	}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{LogLevel: new(zapcore.ErrorLevel)},
		func(ctx context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))
			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewNodeUniqueTokenStatusController()))

			// a registered machine without a token: the legacy token migration must create a pending machine for it
			link := siderolink.NewLink("same-machine", &specs.SiderolinkSpec{NodePublicKey: oldPublicKey, NodeSubnet: unixSocket + ms.address})
			require.NoError(t, tc.State.Create(ctx, link))
			require.NoError(t, tc.State.Create(ctx, omni.NewMachine("same-machine")))

			labels := system.NewResourceLabels[*omni.MachineStatus]("same-machine")
			labels.Metadata().Labels().Set(omni.MachineStatusLabelTalosVersion, "v1.12.0")
			labels.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
			require.NoError(t, tc.State.Create(ctx, labels))

			// the WireGuard peers of the existing link and of both pending machines
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(link)))
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(siderolink.NewPendingMachine(oldPublicKey, nil))))
			require.NoError(t, tc.State.Create(ctx, siderolink.NewLinkStatus(siderolink.NewPendingMachine(newPublicKey, nil))))

			createProvisionPrerequisites(ctx, t, tc.State)

			// the new boot joins without a token
			handler := siderolinkpkg.NewProvisionHandler(tc.Logger, tc.State, config.SiderolinkServiceJoinTokensModeStrict, false, 0)
			_, err = handler.Provision(ctx, &pb.ProvisionRequest{NodeUuid: "same-machine", NodePublicKey: newPublicKey, JoinToken: new("join"), TalosVersion: new("v1.12.0")})
			require.NoError(t, err)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			var first string

			select {
			case first = <-candidates:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			rtestutils.AssertResource(ctx, t, tc.State, oldPublicKey, func(res *siderolink.PendingMachine, a *assert.Assertions) {
				a.Equal(unixSocket+ms.address, res.TypedSpec().Value.NodeSubnet)
			})

			for range 2 {
				select {
				case <-reconciles:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}

			close(allowWrites)

			select {
			case value := <-written:
				require.Equal(t, first, value)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			rtestutils.AssertResources(ctx, t, tc.State, []string{oldPublicKey, newPublicKey}, func(res *siderolink.PendingMachineStatus, a *assert.Assertions) {
				a.Equal(first, res.TypedSpec().Value.Token)
			})
			require.Equal(t, first, ms.getMetaKeys()[meta.UniqueMachineToken])

			// the machine re-joins with the token from META, which must be accepted without a warning
			logCore, observedLogs := observer.New(zap.WarnLevel)
			handler := siderolinkpkg.NewProvisionHandler(zap.New(logCore), tc.State, config.SiderolinkServiceJoinTokensModeStrict, false, 0)

			_, err = handler.Provision(ctx, &pb.ProvisionRequest{
				NodeUuid: "same-machine", NodePublicKey: newPublicKey, NodeUniqueToken: new(first), JoinToken: new("join"), TalosVersion: new("v1.12.0"),
			})
			require.NoError(t, err)
			require.Zero(t, observedLogs.Len())
		})
}

// createProvisionPrerequisites creates the resources the provision handler needs to accept a join with the join token "join".
func createProvisionPrerequisites(ctx context.Context, t *testing.T, st state.State) {
	cfg := siderolink.NewConfig()
	cfg.TypedSpec().Value.Subnet = "fdae:41e4:649b:9303::/64"
	require.NoError(t, st.Create(ctx, cfg))

	joinStatus := siderolink.NewJoinTokenStatus("join")
	joinStatus.TypedSpec().Value.State = specs.JoinTokenStatusSpec_ACTIVE
	require.NoError(t, st.Create(ctx, joinStatus))
}

// statusWatchState signals when the provision handler starts waiting for the pending machine status,
// which is the moment a join request sits between the META write and the saved status.
type statusWatchState struct {
	state.State

	watchStarted chan struct{}
}

func (st *statusWatchState) WatchFor(ctx context.Context, ptr resource.Pointer, opts ...state.WatchForConditionFunc) (resource.Resource, error) {
	if ptr.Type() == siderolink.PendingMachineStatusType {
		close(st.watchStarted)
	}

	return st.State.WatchFor(ctx, ptr, opts...)
}

func TestPendingMachineStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(PendingMachineStatusSuite))
}
