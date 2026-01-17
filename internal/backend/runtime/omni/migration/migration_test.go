// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/clustermachine"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/migration"
)

type MigrationSuite struct {
	suite.Suite

	state   state.State
	manager *migration.Manager
	logger  *zap.Logger
}

func (suite *MigrationSuite) SetupTest() {
	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	suite.logger = zaptest.NewLogger(suite.T())

	suite.manager = migration.NewManager(suite.state, suite.logger.WithOptions(zap.IncreaseLevel(zapcore.WarnLevel)))
}

func (suite *MigrationSuite) TestMoveClusterTaintFromResourceToLabel() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	clusterID := "cluster1"
	deletedClusterID := "cluster2"

	taint := omni.NewClusterTaint(clusterID)
	suite.Require().NoError(suite.state.Create(ctx, taint))

	danglingTaint := omni.NewClusterTaint(deletedClusterID)
	suite.Require().NoError(suite.state.Create(ctx, danglingTaint))

	clusterStatus := omni.NewClusterStatus(clusterID)
	suite.Require().NoError(suite.state.Create(ctx, clusterStatus, state.WithCreateOwner(omnictrl.ClusterStatusControllerName)))

	_, taintBreakGlass := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByBreakGlass)
	suite.Require().False(taintBreakGlass)

	_, err := suite.manager.Run(ctx, migration.WithFilter(filterWith("moveClusterTaintFromResourceToLabel")))
	suite.Require().NoError(err)

	clusterStatusMigrated, err := safe.ReaderGetByID[*omni.ClusterStatus](ctx, suite.state, clusterID)
	suite.Require().NoError(err)

	_, taintBreakGlass = clusterStatusMigrated.Metadata().Labels().Get(omni.LabelClusterTaintedByBreakGlass)
	suite.Require().True(taintBreakGlass)

	taintDeleted, err := safe.ReaderGetByID[*omni.ClusterTaint](ctx, suite.state, clusterID)
	suite.Require().Error(err)
	suite.Require().True(state.IsNotFoundError(err), "ClusterTaint resource should not exist after migration")
	suite.Require().Nil(taintDeleted, "ClusterTaint resource should not exist after migration")

	danglingTaintDeleted, err := safe.ReaderGetByID[*omni.ClusterTaint](ctx, suite.state, deletedClusterID)
	suite.Require().Error(err)
	suite.Require().True(state.IsNotFoundError(err), "ClusterTaint resource should not exist after migration")
	suite.Require().Nil(danglingTaintDeleted, "ClusterTaint resource should not exist after migration")
}

func (suite *MigrationSuite) TestDropExtraInputFinalizers() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	const resourceID = "cms1"

	cms := omni.NewClusterMachineStatus(resourceID)
	cms.Metadata().Finalizers().Add("MachineSetDestroyStatusController")
	cms.Metadata().Finalizers().Add("MachineStatusController")
	cms.Metadata().Finalizers().Add("SomeOtherFinalizer")
	suite.Require().NoError(suite.state.Create(ctx, cms))

	_, err := suite.manager.Run(ctx, migration.WithFilter(filterWith("dropExtraInputFinalizers")))
	suite.Require().NoError(err)

	cmdMigrated, err := safe.ReaderGetByID[*omni.ClusterMachineStatus](ctx, suite.state, resourceID)
	suite.Require().NoError(err)

	suite.Require().False(cmdMigrated.Metadata().Finalizers().Has("MachineSetDestroyStatusController"))
	suite.Require().False(cmdMigrated.Metadata().Finalizers().Has("MachineStatusController"))
	suite.Require().True(cmdMigrated.Metadata().Finalizers().Has("SomeOtherFinalizer"))
}

func (suite *MigrationSuite) TestMoveInfraProviderAnnotationsToLabels() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	link1 := siderolink.NewLink("link1", &specs.SiderolinkSpec{})
	link1.Metadata().Annotations().Set(omni.LabelInfraProviderID, "test-id-1")

	link2 := siderolink.NewLink("link2", &specs.SiderolinkSpec{})
	link2.Metadata().Annotations().Set(omni.LabelInfraProviderID, "test-id-2")
	link2.Metadata().SetPhase(resource.PhaseTearingDown)

	link3 := siderolink.NewLink("link3", &specs.SiderolinkSpec{})

	machine1 := omni.NewMachine("machine1")
	machine1.Metadata().Annotations().Set(omni.LabelInfraProviderID, "test-id-3")

	suite.Require().NoError(suite.state.Create(ctx, link1))
	suite.Require().NoError(suite.state.Create(ctx, link2))
	suite.Require().NoError(suite.state.Create(ctx, link3))
	suite.Require().NoError(suite.state.Create(ctx, machine1))

	link3VersionBefore := link3.Metadata().Version()

	_, err := suite.manager.Run(ctx, migration.WithFilter(filterWith("moveInfraProviderAnnotationsToLabels")))
	suite.Require().NoError(err)

	link1Migrated, err := suite.state.Get(ctx, link1.Metadata())
	suite.Require().NoError(err)

	link1Label, _ := link1Migrated.Metadata().Labels().Get(omni.LabelInfraProviderID)
	_, link1AnnotationOk := link1Migrated.Metadata().Annotations().Get(omni.LabelInfraProviderID)

	suite.Equal("test-id-1", link1Label)
	suite.False(link1AnnotationOk)

	link2Migrated, err := suite.state.Get(ctx, link2.Metadata())
	suite.Require().NoError(err)

	link2Label, _ := link2Migrated.Metadata().Labels().Get(omni.LabelInfraProviderID)
	_, link2AnnotationOk := link2Migrated.Metadata().Annotations().Get(omni.LabelInfraProviderID)

	suite.Equal("test-id-2", link2Label)
	suite.False(link2AnnotationOk)

	link3Migrated, err := suite.state.Get(ctx, link3.Metadata())
	suite.Require().NoError(err)

	suite.Equal(link3VersionBefore, link3Migrated.Metadata().Version(), "expected link3 to be left untouched")

	machine1Migrated, err := suite.state.Get(ctx, machine1.Metadata())
	suite.Require().NoError(err)

	machine1Label, _ := machine1Migrated.Metadata().Labels().Get(omni.LabelInfraProviderID)
	_, machine1AnnotationOk := machine1Migrated.Metadata().Annotations().Get(omni.LabelInfraProviderID)

	suite.Equal("test-id-3", machine1Label)
	suite.False(machine1AnnotationOk)
}

//nolint:dupl
func (suite *MigrationSuite) TestDropSchematicConfigFinalizerFromClusterMachines() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	cm1 := omni.NewClusterMachine("cm1")
	cm1.Metadata().Finalizers().Add(omnictrl.SchematicConfigurationControllerName)
	cm1.Metadata().SetPhase(resource.PhaseTearingDown)
	suite.Require().NoError(suite.state.Create(ctx, cm1))

	cm2 := omni.NewClusterMachine("cm2")
	cm2.Metadata().Finalizers().Add(omnictrl.SchematicConfigurationControllerName)
	suite.Require().NoError(cm2.Metadata().SetOwner("some-owner"))
	suite.Require().NoError(suite.state.Create(ctx, cm2, state.WithCreateOwner("some-owner")))

	cm3 := omni.NewClusterMachine("cm3")
	suite.Require().NoError(suite.state.Create(ctx, cm3))

	cm3VersionBefore := cm3.Metadata().Version()

	_, err := suite.manager.Run(ctx, migration.WithFilter(filterWith("dropSchematicConfigFinalizerFromClusterMachines")))
	suite.Require().NoError(err)

	cm1Migrated, err := suite.state.Get(ctx, cm1.Metadata())
	suite.Require().NoError(err)

	cm2Migrated, err := suite.state.Get(ctx, cm2.Metadata())
	suite.Require().NoError(err)

	cm3Migrated, err := suite.state.Get(ctx, cm3.Metadata())
	suite.Require().NoError(err)

	suite.False(cm1Migrated.Metadata().Finalizers().Has(omnictrl.SchematicConfigurationControllerName))
	suite.False(cm2Migrated.Metadata().Finalizers().Has(omnictrl.SchematicConfigurationControllerName))
	suite.True(cm3VersionBefore.Equal(cm3Migrated.Metadata().Version()), "expected cm3 to be left untouched")
}

//nolint:dupl
func (suite *MigrationSuite) TestDropTalosUpgradeStatusFinalizersFromSchematicConfigs() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	sc1 := omni.NewSchematicConfiguration("sc1")
	sc1.Metadata().Finalizers().Add(omnictrl.TalosUpgradeStatusControllerName)
	sc1.Metadata().SetPhase(resource.PhaseTearingDown)
	suite.Require().NoError(suite.state.Create(ctx, sc1))

	sc2 := omni.NewSchematicConfiguration("sc2")
	sc2.Metadata().Finalizers().Add(omnictrl.TalosUpgradeStatusControllerName)
	suite.Require().NoError(sc2.Metadata().SetOwner("some-owner"))
	suite.Require().NoError(suite.state.Create(ctx, sc2, state.WithCreateOwner("some-owner")))

	sc3 := omni.NewSchematicConfiguration("sc3")
	suite.Require().NoError(suite.state.Create(ctx, sc3))

	sc3VersionBefore := sc3.Metadata().Version()

	_, err := suite.manager.Run(ctx, migration.WithFilter(filterWith("dropTalosUpgradeStatusFinalizersFromSchematicConfigs")))
	suite.Require().NoError(err)

	sc1Migrated, err := suite.state.Get(ctx, sc1.Metadata())
	suite.Require().NoError(err)

	sc2Migrated, err := suite.state.Get(ctx, sc2.Metadata())
	suite.Require().NoError(err)

	sc3Migrated, err := suite.state.Get(ctx, sc3.Metadata())
	suite.Require().NoError(err)

	suite.False(sc1Migrated.Metadata().Finalizers().Has(omnictrl.TalosUpgradeStatusControllerName))
	suite.False(sc2Migrated.Metadata().Finalizers().Has(omnictrl.TalosUpgradeStatusControllerName))
	suite.True(sc3VersionBefore.Equal(sc3Migrated.Metadata().Version()), "expected sc3 to be left untouched")
}

func (suite *MigrationSuite) TestMakeMachineSetNodeOwnerEmpty() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	labelID := "custom"

	machineSet := omni.NewMachineSet("ms1")

	msnRunning := omni.NewMachineSetNode("running", machineSet)

	msnOwned := omni.NewMachineSetNode("owned", machineSet)

	msnOwned.Metadata().Finalizers().Add("fin")
	msnOwned.Metadata().Labels().Set(labelID, "val")
	msnOwned.Metadata().Annotations().Set(labelID, "val")

	msnTearingDown := omni.NewMachineSetNode("tearingDown", machineSet)
	msnTearingDown.Metadata().SetPhase(resource.PhaseTearingDown)

	suite.Require().NoError(suite.state.Create(ctx, msnRunning))
	suite.Require().NoError(suite.state.Create(ctx, msnOwned,
		state.WithCreateOwner(omnictrl.NewMachineSetNodeController().ControllerName)),
	)
	suite.Require().NoError(suite.state.Create(ctx, msnTearingDown,
		state.WithCreateOwner(omnictrl.NewMachineSetNodeController().ControllerName)),
	)

	_, err := suite.manager.Run(ctx, migration.WithFilter(filterWith("makeMachineSetNodesOwnerEmpty")))
	suite.Require().NoError(err)

	msnRunning, err = safe.ReaderGetByID[*omni.MachineSetNode](ctx, suite.state, msnRunning.Metadata().ID())
	suite.Require().NoError(err)

	msnOwned, err = safe.ReaderGetByID[*omni.MachineSetNode](ctx, suite.state, msnOwned.Metadata().ID())
	suite.Require().NoError(err)

	msnTearingDown, err = safe.ReaderGetByID[*omni.MachineSetNode](ctx, suite.state, msnTearingDown.Metadata().ID())
	suite.Require().NoError(err)

	_, labelSet := msnRunning.Metadata().Labels().Get(omni.LabelManagedByMachineSetNodeController)
	suite.Assert().False(labelSet)

	_, labelSet = msnOwned.Metadata().Labels().Get(omni.LabelManagedByMachineSetNodeController)
	suite.Assert().True(labelSet)
	suite.Assert().Empty(msnOwned.Metadata().Owner())

	val, _ := msnOwned.Metadata().Annotations().Get(labelID)
	suite.Assert().Equal("val", val)
	val, _ = msnOwned.Metadata().Annotations().Get(labelID)
	suite.Assert().Equal("val", val)

	suite.Assert().False(msnOwned.Metadata().Finalizers().Empty())

	_, labelSet = msnTearingDown.Metadata().Labels().Get(omni.LabelManagedByMachineSetNodeController)
	suite.Assert().True(labelSet)
	suite.Assert().Empty(msnTearingDown.Metadata().Owner())
	suite.Assert().Equal(resource.PhaseTearingDown, msnTearingDown.Metadata().Phase())
}

func (suite *MigrationSuite) TestChangeClusterMachineConfigPatchesOwner() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	labelID := "custom"

	cmcpRunning := omni.NewClusterMachineConfigPatches("owned")

	cmcpRunning.Metadata().Finalizers().Add("fin")
	cmcpRunning.Metadata().Labels().Set(labelID, "val")
	cmcpRunning.Metadata().Annotations().Set(labelID, "val")

	cmcpTearingDown := omni.NewClusterMachineConfigPatches("tearingDown")
	cmcpTearingDown.Metadata().SetPhase(resource.PhaseTearingDown)

	suite.Require().NoError(suite.state.Create(ctx, cmcpRunning,
		state.WithCreateOwner(omnictrl.NewMachineSetStatusController().ControllerName)),
	)
	suite.Require().NoError(suite.state.Create(ctx, cmcpTearingDown,
		state.WithCreateOwner(omnictrl.NewMachineSetStatusController().ControllerName)),
	)

	_, err := suite.manager.Run(ctx, migration.WithFilter(filterWith("changeClusterMachineConfigPatchesOwner")))

	suite.Require().NoError(err)

	cmcpRunning, err = safe.ReaderGetByID[*omni.ClusterMachineConfigPatches](ctx, suite.state, cmcpRunning.Metadata().ID())
	suite.Require().NoError(err)

	cmcpTearingDown, err = safe.ReaderGetByID[*omni.ClusterMachineConfigPatches](ctx, suite.state, cmcpTearingDown.Metadata().ID())
	suite.Require().NoError(err)

	owner := clustermachine.NewConfigPatchesController().ControllerName

	suite.Assert().Equal(owner, cmcpRunning.Metadata().Owner())

	val, _ := cmcpRunning.Metadata().Annotations().Get(labelID)
	suite.Assert().Equal("val", val)
	val, _ = cmcpRunning.Metadata().Annotations().Get(labelID)
	suite.Assert().Equal("val", val)

	suite.Assert().False(cmcpRunning.Metadata().Finalizers().Empty())

	suite.Assert().Equal(owner, cmcpTearingDown.Metadata().Owner())
	suite.Assert().Equal(resource.PhaseTearingDown, cmcpTearingDown.Metadata().Phase())
}

func TestMigrationSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MigrationSuite))
}

func filterWith(vals ...string) func(string) bool {
	return func(cur string) bool {
		return slices.Contains(vals, cur)
	}
}
