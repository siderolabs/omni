// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/destroy"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

const msnHolderFinalizer = "test-msn-finalizer-holder"

// msnFinalizerHolder mimics the downstream finalizer that MachineSetStatusController holds on
// every MachineSetNode: it is added while the node is running, and released once the node starts
// tearing down (standing in for the downstream cleanup that gates destruction).
type msnFinalizerHolder struct{}

func (msnFinalizerHolder) Name() string { return "MSNFinalizerHolderController" }

func (msnFinalizerHolder) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineSetNodeType,
				Kind:      controller.InputQPrimary,
			},
		},
		Concurrency: optional.Some(uint(4)),
	}
}

func (msnFinalizerHolder) MapInput(
	context.Context, *zap.Logger, controller.QRuntime, controller.ReducedResourceMetadata,
) ([]resource.Pointer, error) {
	return nil, nil
}

func (msnFinalizerHolder) Reconcile(ctx context.Context, _ *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	msn, err := safe.ReaderGet[*omni.MachineSetNode](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if msn.Metadata().Phase() == resource.PhaseRunning {
		if !msn.Metadata().Finalizers().Has(msnHolderFinalizer) {
			return r.AddFinalizer(ctx, msn.Metadata(), msnHolderFinalizer)
		}

		return nil
	}

	// tearing down: simulate downstream cleanup completing and release the finalizer
	if msn.Metadata().Finalizers().Has(msnHolderFinalizer) {
		return r.RemoveFinalizer(ctx, msn.Metadata(), msnHolderFinalizer)
	}

	return nil
}

// cascadeTestSetup registers the controllers (including the generic destroy controller for
// MachineSetNode, as in production) and creates a static machine set with `count` matching
// machines, each node carrying a downstream finalizer. It returns the machine set and node ids.
func (suite *MachineSetNodeSuite) cascadeTestSetup(ctx context.Context, clusterName, machineSetName string, count int) (*omni.MachineSet, []string) {
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineSetNodeController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewLabelsExtractorController[*omni.MachineStatus]()))
	suite.Require().NoError(suite.runtime.RegisterQController(msnFinalizerHolder{}))
	suite.Require().NoError(suite.runtime.RegisterQController(destroy.NewController[*omni.MachineSetNode](optional.Some[uint](4))))
	// as in production, every user-managed type gets a destroy controller
	suite.Require().NoError(suite.runtime.RegisterQController(destroy.NewController[*omni.MachineSet](optional.Some[uint](4))))

	labelSets := make([]map[string]string, count)
	for i := range labelSets {
		labelSets[i] = map[string]string{
			omni.MachineStatusLabelArch:            "amd64",
			omni.MachineStatusLabelAvailable:       "",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReadyToUse:      "",
			omni.MachineStatusLabelReportingEvents: "",
		}
	}

	machines := suite.createMachines(labelSets...)

	cluster := omni.NewCluster(clusterName)
	cluster.TypedSpec().Value.TalosVersion = "1.6.0"
	suite.Require().NoError(suite.state.Create(ctx, cluster))

	machineClass := newMachineClass(fmt.Sprintf("%s==amd64", omni.MachineStatusLabelArch))
	suite.Require().NoError(suite.state.Create(ctx, machineClass))

	machineSet := omni.NewMachineSet(machineSetName)
	machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	machineSet.Metadata().Labels().Set(omni.LabelWorkerRole, "")
	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name:         machineClass.Metadata().ID(),
		MachineCount: uint32(count), //nolint:gosec
	}
	suite.Require().NoError(suite.state.Create(ctx, machineSet))

	ids := xslices.Map(machines, func(m *omni.MachineStatus) string { return m.Metadata().ID() })

	// all MSNs created and carrying the downstream finalizer
	rtestutils.AssertResources(ctx, suite.T(), suite.state, ids, func(n *omni.MachineSetNode, assert *assert.Assertions) {
		assert.True(n.Metadata().Finalizers().Has(msnHolderFinalizer))
	})

	return machineSet, ids
}

// TestScaleDownReleasesAllNodes verifies that scaling a machine set down tears down and destroys
// every node, with the destroy controller doing the destruction. Before the finalizer handoff fix
// the cascade stalled after the first node, because its destroy-ready wake-up was lost when the
// destroy controller destroyed it before the controller could map it back to its machine set.
func (suite *MachineSetNodeSuite) TestScaleDownReleasesAllNodes() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	machineSet, ids := suite.cascadeTestSetup(ctx, "cluster-scaledown", "set-scaledown", 5)

	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineAllocation.MachineCount = 0

		return nil
	})
	suite.Require().NoError(err)

	for _, id := range ids {
		rtestutils.AssertNoResource[*omni.MachineSetNode](ctx, suite.T(), suite.state, id)
	}
}

// TestMachineSetTeardownReleasesAllNodes verifies that tearing the whole machine set down tears
// down and destroys every node and then releases the machine set itself.
func (suite *MachineSetNodeSuite) TestMachineSetTeardownReleasesAllNodes() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	machineSet, ids := suite.cascadeTestSetup(ctx, "cluster-teardown", "set-teardown", 5)

	_, err := suite.state.Teardown(ctx, machineSet.Metadata())
	suite.Require().NoError(err)

	for _, id := range ids {
		rtestutils.AssertNoResource[*omni.MachineSetNode](ctx, suite.T(), suite.state, id)
	}

	rtestutils.AssertNoResource[*omni.MachineSet](ctx, suite.T(), suite.state, machineSet.Metadata().ID())
}

// TestTeardownAdoptsPreexistingNodes covers the upgrade case where a machine set is torn down with
// nodes that were created before this controller started holding its finalizer. The controller must
// adopt them so the destroy controller cannot delete them before they are handed off, otherwise the
// machine set finalizer would never be released.
func (suite *MachineSetNodeSuite) TestTeardownAdoptsPreexistingNodes() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	const staleFinalizer = "stale-downstream"

	// destroy controllers are registered first; the machine set node controller is registered only
	// after the stuck pre-upgrade state is in place, so it never gets a chance to finalize the nodes
	suite.Require().NoError(suite.runtime.RegisterQController(destroy.NewController[*omni.MachineSetNode](optional.Some[uint](4))))
	suite.Require().NoError(suite.runtime.RegisterQController(destroy.NewController[*omni.MachineSet](optional.Some[uint](4))))

	machineSet := omni.NewMachineSet("set-adopt")
	machineSet.Metadata().Labels().Set(omni.LabelCluster, "cluster-adopt")
	machineSet.Metadata().Labels().Set(omni.LabelWorkerRole, "")
	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name:         "some-machine-class",
		MachineCount: 2,
	}
	suite.Require().NoError(suite.state.Create(ctx, machineSet))
	suite.Require().NoError(suite.state.AddFinalizer(ctx, machineSet.Metadata(), omnictrl.MachineSetNodeControllerName))
	_, err := suite.state.Teardown(ctx, machineSet.Metadata())
	suite.Require().NoError(err)

	ids := []string{"adopt-machine-0", "adopt-machine-1"}

	for _, id := range ids {
		msn := omni.NewMachineSetNode(id, machineSet)
		msn.Metadata().Labels().Set(omni.LabelManagedByMachineSetNodeController, "")

		suite.Require().NoError(suite.state.Create(ctx, msn))
		suite.Require().NoError(suite.state.AddFinalizer(ctx, msn.Metadata(), staleFinalizer))

		_, err = suite.state.Teardown(ctx, msn.Metadata())
		suite.Require().NoError(err)
	}

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineSetNodeController()))

	// the controller adopts the pre-existing nodes, so they stay around (now also carrying our
	// finalizer) until their downstream cleanup finishes
	rtestutils.AssertResources(ctx, suite.T(), suite.state, ids, func(n *omni.MachineSetNode, assert *assert.Assertions) {
		assert.True(n.Metadata().Finalizers().Has(omnictrl.MachineSetNodeControllerName))
	})

	// finish the downstream cleanup; the nodes and then the machine set must be destroyed
	for _, id := range ids {
		suite.Require().NoError(suite.state.RemoveFinalizer(ctx, omni.NewMachineSetNode(id, machineSet).Metadata(), staleFinalizer))
	}

	for _, id := range ids {
		rtestutils.AssertNoResource[*omni.MachineSetNode](ctx, suite.T(), suite.state, id)
	}

	rtestutils.AssertNoResource[*omni.MachineSet](ctx, suite.T(), suite.state, machineSet.Metadata().ID())
}
