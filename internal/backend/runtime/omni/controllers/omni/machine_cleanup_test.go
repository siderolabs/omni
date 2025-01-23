// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineCleanupSuite struct {
	OmniSuite
}

func (suite *MachineCleanupSuite) TestCleanup() {
	require := suite.Require()

	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.ctx, time.Second*10)

	suite.startRuntime()

	controller := omnictrl.NewMachineCleanupController()

	require.NoError(suite.runtime.RegisterController(controller))

	machineID := "machine-cleanup-test-machine"

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "machine-cleanup-test-machine-set")
	machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, machineID, machineSet)
	machine := omni.NewMachine(resources.DefaultNamespace, machineID)

	machine.Metadata().Finalizers().Add(controller.Name())

	require.NoError(suite.state.Create(suite.ctx, machineSetNode))
	require.NoError(suite.state.Create(suite.ctx, machine))

	_, err := suite.state.Teardown(suite.ctx, machine.Metadata())

	require.NoError(err)

	assertNoResource[*omni.MachineSetNode](&suite.OmniSuite, machineSetNode)

	assertResource[*omni.Machine](&suite.OmniSuite, machine.Metadata(), func(r *omni.Machine, assertion *assert.Assertions) {
		assertion.Empty(r.Metadata().Finalizers())
	})

	require.NoError(suite.state.Destroy(suite.ctx, machine.Metadata()))
}

func (suite *MachineCleanupSuite) TestSkipMachineSetNodeWithOwner() {
	require := suite.Require()

	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.ctx, time.Second*10)

	suite.startRuntime()

	controller := omnictrl.NewMachineCleanupController()

	require.NoError(suite.runtime.RegisterController(controller))

	machineID := "machine-cleanup-skip-test-machine"

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "machine-cleanup-skip-test-machine-set")
	machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, machineID, machineSet)
	machine := omni.NewMachine(resources.DefaultNamespace, machineID)

	machine.Metadata().Finalizers().Add(controller.Name())
	require.NoError(machineSetNode.Metadata().SetOwner("some-owner"))

	require.NoError(suite.state.Create(suite.ctx, machineSetNode, state.WithCreateOwner("some-owner")))
	require.NoError(suite.state.Create(suite.ctx, machine))

	rtestutils.Destroy[*omni.Machine](suite.ctx, suite.T(), suite.state, []string{machine.Metadata().ID()})

	// MachineSetNode should still be around, as it is owned by a controller - CleanupController should skip it
	assertResource[*omni.MachineSetNode](&suite.OmniSuite, machine.Metadata(), func(*omni.MachineSetNode, *assert.Assertions) {})
}

func TestMachineCleanupSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineCleanupSuite))
}
