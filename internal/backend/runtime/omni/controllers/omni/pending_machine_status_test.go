// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
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

	createPendingMachine := func(name, uuid string) *siderolink.PendingMachine {
		ms, err := suite.newServer(name)
		suite.Require().NoError(err)

		machineServices[name] = ms

		pendingMachine := siderolink.NewPendingMachine(name, &specs.SiderolinkSpec{})
		pendingMachine.Metadata().Labels().Set(omni.MachineUUID, uuid)

		pendingMachine.TypedSpec().Value.NodeSubnet = unixSocket + suite.socketPath + name

		suite.Require().NoError(suite.state.Create(ctx, pendingMachine))

		return pendingMachine
	}

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewPendingMachineStatusController()))

	defaultUUID := "uuid"

	type machine struct {
		link *specs.SiderolinkSpec
		uuid string
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
			uuid: "conflict-link",
			link: &specs.SiderolinkSpec{
				NodeUniqueToken: "abcdefg",
			},
		},
	}

	awaitMachines := make([]string, 0, len(machines))

	links := make([]*siderolink.LinkStatus, 0, len(machines))

	for index, m := range machines {
		pm := createPendingMachine(fmt.Sprintf("p%d", index), m.uuid)

		awaitMachines = append(awaitMachines, pm.Metadata().ID())

		links = append(links, siderolink.NewLinkStatus(pm))

		if m.link != nil {
			suite.Require().NoError(suite.state.Create(ctx, siderolink.NewLink(resources.DefaultNamespace, m.uuid, m.link)))
		}
	}

	for _, link := range links {
		suite.Require().NoError(suite.state.Create(ctx, link))
	}

	uuidCounts := map[string]map[string]struct{}{}

	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, awaitMachines,
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

func TestPendingMachineStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(PendingMachineStatusSuite))
}
