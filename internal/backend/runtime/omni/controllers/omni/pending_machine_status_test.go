// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
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

	for name, machineService := range machineServices {
		metaKeys := machineService.getMetaKeys()

		token, ok := metaKeys[meta.UniqueMachineToken]

		suite.Assert().True(ok, "no unique token, machine %s", name)
		suite.Assert().NotEmpty(token, "empty unique token, machine %s", name)
	}

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

func TestPendingMachineStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(PendingMachineStatusSuite))
}
