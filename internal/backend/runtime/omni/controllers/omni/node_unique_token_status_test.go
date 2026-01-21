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
	"github.com/siderolabs/omni/client/pkg/jointoken"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type NodeUniqueTokenStatusControllerSuite struct {
	OmniSuite
}

func (suite *MachineStatusSnapshotControllerSuite) TestNodeUniqueTokenStatus() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	// Register the controller and start the runtime
	controller := omni.NewNodeUniqueTokenStatusController()

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	var machineCount int

	createMachine := func(connected, withToken bool, talosVersion string) string {
		machineStatus := system.NewResourceLabels[*omnires.MachineStatus](fmt.Sprintf("m%d", machineCount))

		machine := omnires.NewMachine(machineStatus.Metadata().ID())

		if connected {
			machineStatus.Metadata().Labels().Set(omnires.MachineStatusLabelConnected, "")
		} else {
			machineStatus.Metadata().Labels().Delete(omnires.MachineStatusLabelConnected)
		}

		machineStatus.Metadata().Labels().Set(omnires.MachineStatusLabelTalosVersion, talosVersion)

		if withToken {
			token := siderolink.NewNodeUniqueToken(machineStatus.Metadata().ID())

			var err error

			token.TypedSpec().Value.Token, err = jointoken.NewNodeUniqueToken("aaa", machineStatus.Metadata().ID()).Encode()

			suite.Require().NoError(err)

			suite.Require().NoError(suite.state.Create(ctx, token))
		}

		machineCount++

		suite.Require().NoError(suite.state.Create(ctx, machine))
		suite.Require().NoError(suite.state.Create(ctx, machineStatus))

		return machineStatus.Metadata().ID()
	}

	id := createMachine(true, true, "v1.9.0")

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{id}, func(status *siderolink.NodeUniqueTokenStatus, assert *assert.Assertions) {
		assert.Equal(specs.NodeUniqueTokenStatusSpec_EPHEMERAL, status.TypedSpec().Value.State)
	})

	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, system.NewResourceLabels[*omnires.MachineStatus](id).Metadata(),
		func(res *system.ResourceLabels[*omnires.MachineStatus]) error {
			res.Metadata().Labels().Set(omnires.MachineStatusLabelInstalled, "")

			return nil
		},
	)

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{id}, func(status *siderolink.NodeUniqueTokenStatus, assert *assert.Assertions) {
		assert.Equal(specs.NodeUniqueTokenStatusSpec_PERSISTENT, status.TypedSpec().Value.State)
	})

	id = createMachine(true, true, "v1.5.0")

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{id}, func(status *siderolink.NodeUniqueTokenStatus, assert *assert.Assertions) {
		assert.Equal(specs.NodeUniqueTokenStatusSpec_UNSUPPORTED, status.TypedSpec().Value.State)
	})

	id = createMachine(false, false, "v1.9.0")

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{id}, func(status *siderolink.NodeUniqueTokenStatus, assert *assert.Assertions) {
		assert.Equal(specs.NodeUniqueTokenStatusSpec_NONE, status.TypedSpec().Value.State)
	})

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, system.NewResourceLabels[*omnires.MachineStatus](id).Metadata(),
		func(res *system.ResourceLabels[*omnires.MachineStatus]) error {
			res.Metadata().Labels().Set(omnires.MachineStatusLabelConnected, "")

			return nil
		},
	)

	suite.Require().NoError(err)

	token := siderolink.NewNodeUniqueToken(id)

	token.TypedSpec().Value.Token, err = jointoken.NewNodeUniqueToken("aaa", id).Encode()

	suite.Require().NoError(err)

	suite.Require().NoError(suite.state.Create(ctx, token))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{id}, func(status *siderolink.NodeUniqueTokenStatus, assert *assert.Assertions) {
		assert.Equal(specs.NodeUniqueTokenStatusSpec_EPHEMERAL, status.TypedSpec().Value.State)
	})
}

func TestNodeUniqueTokenStatusControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(NodeUniqueTokenStatusControllerSuite))
}
