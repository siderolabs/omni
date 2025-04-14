// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ConfigPatchCleanupSuite struct {
	OmniSuite
}

func (suite *ConfigPatchCleanupSuite) SetupTest() {
	suite.OmniSuite.ctx, suite.OmniSuite.ctxCancel = context.WithCancel(context.Background()) //nolint:fatcontext
	suite.OmniSuite.disableConnections = true
	suite.OmniSuite.SetupTest()
	suite.startRuntime()
}

func (suite *ConfigPatchCleanupSuite) TestReconcile() {
	suite.Require().NoError(suite.runtime.RegisterController(&omnictrl.ConfigPatchCleanupController{}))

	// create the initial orphan patch

	earlyPatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-early-patch")
	suite.Require().NoError(suite.state.Create(suite.ctx, earlyPatch))

	time.Sleep(15 * 24 * time.Hour) // advance, then create all other resources

	cluster := omni.NewCluster(resources.DefaultNamespace, "test-cluster")
	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "test-machine-set")
	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "test-cluster-machine")
	machine := omni.NewMachine(resources.DefaultNamespace, "test-machine")

	suite.Require().NoError(suite.state.Create(suite.ctx, cluster))
	suite.Require().NoError(suite.state.Create(suite.ctx, machineSet))
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachine))
	suite.Require().NoError(suite.state.Create(suite.ctx, machine))

	clusterPatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-cluster-patch")
	clusterPatch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

	clusterWithNonExistentMachinePatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-cluster-no-machine-patch")
	clusterWithNonExistentMachinePatch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	clusterWithNonExistentMachinePatch.Metadata().Labels().Set(omni.LabelMachine, "non-existent-machine")

	machineSetPatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-machine-set-patch")
	machineSetPatch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	machineSetPatch.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())

	clusterMachinePatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-cluster-machine-patch")
	clusterMachinePatch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	clusterMachinePatch.Metadata().Labels().Set(omni.LabelClusterMachine, clusterMachine.Metadata().ID())

	machinePatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-machine-patch")
	machinePatch.Metadata().Labels().Set(omni.LabelMachine, machine.Metadata().ID())

	nonExistentClusterPatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-cluster-patch-non-existent")
	nonExistentClusterPatch.Metadata().Labels().Set(omni.LabelCluster, "non-existent-cluster")

	nonExistentMachineSetPatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-machine-set-patch-non-existent")
	nonExistentMachineSetPatch.Metadata().Labels().Set(omni.LabelMachineSet, "non-existent-machine-set")

	nonExistentClusterMachinePatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-cluster-machine-patch-non-existent")
	nonExistentClusterMachinePatch.Metadata().Labels().Set(omni.LabelClusterMachine, "non-existent-cluster-machine")

	nonExistentMachinePatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-machine-patch-non-existent")
	nonExistentMachinePatch.Metadata().Labels().Set(omni.LabelMachine, "non-existent-machine")

	unassociatedPatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-unassociated-patch")

	tearingDownPatch := omni.NewConfigPatch(resources.DefaultNamespace, "test-tearing-down-patch")
	tearingDownPatch.Metadata().SetPhase(resource.PhaseTearingDown)

	patchWithOwner := omni.NewConfigPatch(resources.DefaultNamespace, "test-patch-with-owner")

	patchWithOwner.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	suite.Require().NoError(patchWithOwner.Metadata().SetOwner("some-owner"))

	patchWithFinalizer := omni.NewConfigPatch(resources.DefaultNamespace, "test-patch-with-finalizer")
	patchWithFinalizer.Metadata().Finalizers().Add("some-finalizer")

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterPatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, machineSetPatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachinePatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, machinePatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, nonExistentClusterPatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, nonExistentMachineSetPatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, nonExistentClusterMachinePatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, nonExistentMachinePatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, unassociatedPatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, tearingDownPatch))
	suite.Require().NoError(suite.state.Create(suite.ctx, patchWithOwner, state.WithCreateOwner("some-owner")))
	suite.Require().NoError(suite.state.Create(suite.ctx, patchWithFinalizer))
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterWithNonExistentMachinePatch))

	// advance until the early patch is deleted (so we know that we triggered the cleanup, and it worked as expected)

	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, earlyPatch.Metadata().ID())

	// assert that other resources are still there, as none of them reached the deadline yet

	rtestutils.AssertResources[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, []string{
		clusterPatch.Metadata().ID(),
		machineSetPatch.Metadata().ID(),
		clusterMachinePatch.Metadata().ID(),
		machinePatch.Metadata().ID(),
		nonExistentClusterPatch.Metadata().ID(),
		nonExistentMachineSetPatch.Metadata().ID(),
		nonExistentClusterMachinePatch.Metadata().ID(),
		nonExistentMachinePatch.Metadata().ID(),
		unassociatedPatch.Metadata().ID(),
		tearingDownPatch.Metadata().ID(),
		patchWithOwner.Metadata().ID(),
		patchWithFinalizer.Metadata().ID(),
		clusterWithNonExistentMachinePatch.Metadata().ID(),
	}, func(r *omni.ConfigPatch, assertion *assert.Assertions) {
		if r.Metadata().ID() == tearingDownPatch.Metadata().ID() { // assert that there was no phase change
			assertion.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		} else {
			assertion.Equal(resource.PhaseRunning, r.Metadata().Phase())
		}
	})

	// advance until the patch with finalizer gets into the tearing down phase, and other orphans get deleted

	rtestutils.AssertResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, patchWithFinalizer.Metadata().ID(), func(r *omni.ConfigPatch, assertion *assert.Assertions) {
		assertion.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
	})

	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, nonExistentClusterPatch.Metadata().ID())
	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, nonExistentMachineSetPatch.Metadata().ID())
	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, nonExistentClusterMachinePatch.Metadata().ID())
	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, nonExistentMachinePatch.Metadata().ID())
	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, unassociatedPatch.Metadata().ID())
	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, tearingDownPatch.Metadata().ID())

	// remove the finalizer from the patch with the finalizer

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, patchWithFinalizer.Metadata(), func(r *omni.ConfigPatch) error {
		r.Metadata().Finalizers().Remove("some-finalizer")

		return nil
	}, state.WithExpectedPhaseAny())
	suite.Require().NoError(err)

	// advance further until it is deleted as well

	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, patchWithFinalizer.Metadata().ID())

	// assert that the non-orphans were left untouched during the process
	rtestutils.AssertResources[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, []string{
		clusterPatch.Metadata().ID(),
		machineSetPatch.Metadata().ID(),
		clusterMachinePatch.Metadata().ID(),
		machinePatch.Metadata().ID(),
		patchWithOwner.Metadata().ID(),
		clusterWithNonExistentMachinePatch.Metadata().ID(),
	}, func(r *omni.ConfigPatch, assertion *assert.Assertions) {
		assertion.Equal(resource.PhaseRunning, r.Metadata().Phase())
	})
}

func TestConfigPatchCleanupSuite(t *testing.T) {
	t.Parallel()

	synctest.Run(func() {
		suite.Run(t, new(ConfigPatchCleanupSuite))
	})
}
