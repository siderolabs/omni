// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"fmt"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type TalosUpgradeStatusSuite struct {
	OmniSuite
}

func (suite *TalosUpgradeStatusSuite) TestReconcile() {
	suite.startRuntime()

	clusterName := "talos-upgrade-cluster"

	cluster, machines := suite.createCluster(clusterName, 3, 1)

	clusterStatus := omni.NewClusterStatus(resources.DefaultNamespace, clusterName)
	clusterStatus.TypedSpec().Value.Available = true
	clusterStatus.TypedSpec().Value.Ready = true
	clusterStatus.TypedSpec().Value.Phase = specs.ClusterStatusSpec_RUNNING

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	for _, res := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, res.Metadata().ID()).Metadata(),
			func(version *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
				versionSpec := version.TypedSpec().Value

				assertions.Equal(cluster.TypedSpec().Value.TalosVersion, versionSpec.TalosVersion)
			},
		)

		configStatus := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, res.Metadata().ID())

		helpers.CopyAllLabels(res, configStatus)

		configStatus.TypedSpec().Value.ClusterMachineConfigSha256 = "aaaa"
		configStatus.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
		configStatus.TypedSpec().Value.SchematicId = defaultSchematic

		suite.Require().NoError(suite.state.Create(suite.ctx, configStatus))
	}

	assertResource(
		&suite.OmniSuite,
		*omni.NewTalosUpgradeStatus(resources.DefaultNamespace, clusterName).Metadata(),
		func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
			assertions.Equal(TalosVersion, res.TypedSpec().Value.LastUpgradeVersion)
		},
	)

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, clusterStatus.Metadata(), func(res *omni.ClusterStatus) error {
		res.TypedSpec().Value.Ready = false

		return nil
	})

	suite.Require().NoError(err)

	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, cluster.Metadata(), func(res *omni.Cluster) error {
		res.TypedSpec().Value.TalosVersion = "1.3.6"

		return nil
	})

	suite.Require().NoError(err)

	assertResource(
		&suite.OmniSuite,
		*omni.NewTalosUpgradeStatus(resources.DefaultNamespace, clusterName).Metadata(),
		func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
			assertions.Equal("waiting for the cluster to be ready", res.TypedSpec().Value.Status)
			assertions.Equal("1.3.6", res.TypedSpec().Value.CurrentUpgradeVersion)
			assertions.Equal(TalosVersion, res.TypedSpec().Value.LastUpgradeVersion)
		},
	)

	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, clusterStatus.Metadata(), func(res *omni.ClusterStatus) error {
		res.TypedSpec().Value.Ready = true

		return nil
	})

	suite.Require().NoError(err)

	assertResource(
		&suite.OmniSuite,
		*omni.NewTalosUpgradeStatus(resources.DefaultNamespace, clusterName).Metadata(),
		func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
			assertions.Equal("updating machines 1/4", res.TypedSpec().Value.Status)
			assertions.Equal("1.3.6", res.TypedSpec().Value.CurrentUpgradeVersion)
			assertions.Equal(TalosVersion, res.TypedSpec().Value.LastUpgradeVersion)
		},
	)

	extensions := omni.NewMachineExtensions(resources.DefaultNamespace, machines[1].Metadata().ID())
	extensions.TypedSpec().Value.Extensions = []string{"some-extension"}

	suite.Require().NoError(suite.state.Create(suite.ctx, extensions))

	for i := range len(machines) {
		expectedSchematic := defaultSchematic
		if i == 1 {
			expectedSchematic = "c6ee5f479027e5ca84e5518c3a56d62e2283b6d30a5846e6295aa7113735df40"
		}

		rtestutils.AssertResource[*omni.ClusterMachineTalosVersion](suite.ctx, suite.T(), suite.state, machines[i].Metadata().ID(),
			func(r *omni.ClusterMachineTalosVersion, assertion *assert.Assertions) {
				assertion.Equal(expectedSchematic, r.TypedSpec().Value.SchematicId)
			},
		)

		configStatus := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, machines[i].Metadata().ID())
		configStatus.TypedSpec().Value.ClusterMachineConfigSha256 = "aaaa"

		_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, configStatus.Metadata(), func(res *omni.ClusterMachineConfigStatus) error {
			res.TypedSpec().Value.TalosVersion = "1.3.6"
			res.TypedSpec().Value.SchematicId = expectedSchematic

			return nil
		})

		suite.Require().NoError(err)

		assertResource(
			&suite.OmniSuite,
			*omni.NewTalosUpgradeStatus(resources.DefaultNamespace, clusterName).Metadata(),
			func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
				if i < len(machines)-1 {
					assertions.Equal(fmt.Sprintf("updating machines %d/4", i+2), res.TypedSpec().Value.Status)
					assertions.Equal("1.3.6", res.TypedSpec().Value.CurrentUpgradeVersion)
					assertions.Equal(TalosVersion, res.TypedSpec().Value.LastUpgradeVersion)
					assertions.Equal(specs.TalosUpgradeStatusSpec_Upgrading, res.TypedSpec().Value.Phase)
				} else {
					assertions.Empty(res.TypedSpec().Value.Step)
					assertions.Empty(res.TypedSpec().Value.Error)
					assertions.Empty(res.TypedSpec().Value.Status)
					assertions.Empty(res.TypedSpec().Value.CurrentUpgradeVersion)
					assertions.Equal("1.3.6", res.TypedSpec().Value.LastUpgradeVersion)
					assertions.Equal(specs.TalosUpgradeStatusSpec_Done, res.TypedSpec().Value.Phase)
				}
			},
		)
	}

	rtestutils.Destroy[*omni.ClusterMachine](suite.ctx, suite.T(), suite.state, []resource.ID{machines[0].Metadata().ID()})

	assertNoResource(
		&suite.OmniSuite,
		omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, machines[0].Metadata().ID()),
	)

	suite.destroyCluster(cluster)

	for _, res := range machines {
		assertNoResource(
			&suite.OmniSuite,
			omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, res.Metadata().ID()),
		)
	}
}

// TestUpdateVersionsMaintenance checks that machines' Talos version can be updated immediately
// if a machine is still running in the maintenance mode.
func (suite *TalosUpgradeStatusSuite) TestUpdateVersionsMaintenance() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineSetStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "talos-upgrade-cluster"

	cluster, machines := suite.createCluster(clusterName, 3, 1)

	clusterStatus := omni.NewClusterStatus(resources.DefaultNamespace, clusterName)
	clusterStatus.TypedSpec().Value.Available = true
	clusterStatus.TypedSpec().Value.Ready = false
	clusterStatus.TypedSpec().Value.Phase = specs.ClusterStatusSpec_SCALING_UP

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	for i, res := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, res.Metadata().ID()).Metadata(),
			func(version *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
				versionSpec := version.TypedSpec().Value

				assertions.Equal(cluster.TypedSpec().Value.TalosVersion, versionSpec.TalosVersion)
			},
		)

		configStatus := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, res.Metadata().ID())

		if i != 0 {
			configStatus.TypedSpec().Value.ClusterMachineConfigSha256 = "bbbb"
		}

		configStatus.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion

		suite.Require().NoError(suite.state.Create(suite.ctx, configStatus))
	}

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, cluster.Metadata(), func(res *omni.Cluster) error {
		res.TypedSpec().Value.TalosVersion = "1.5.5"

		return nil
	})

	suite.Require().NoError(err)

	for i, res := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, res.Metadata().ID()).Metadata(),
			func(version *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
				versionSpec := version.TypedSpec().Value

				expectedVersion := cluster.TypedSpec().Value.TalosVersion
				if i == 0 {
					expectedVersion = "1.5.5"
				}

				assertions.Equal(expectedVersion, versionSpec.TalosVersion)
			},
		)
	}
}

func (suite *TalosUpgradeStatusSuite) TestReconcileLocked() {
	suite.startRuntime()

	clusterName := "talos-upgrade-locked"

	cluster, machines := suite.createCluster(clusterName, 3, 1)

	clusterStatus := omni.NewClusterStatus(resources.DefaultNamespace, clusterName)
	clusterStatus.TypedSpec().Value.Available = true
	clusterStatus.TypedSpec().Value.Ready = true
	clusterStatus.TypedSpec().Value.Phase = specs.ClusterStatusSpec_RUNNING

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	for _, res := range machines {
		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, resource.NewMetadata(resources.DefaultNamespace, omni.MachineSetNodeType, res.Metadata().ID(), resource.VersionUndefined), func(
			r *omni.MachineSetNode,
		) error {
			r.Metadata().Annotations().Set(omni.MachineLocked, "")

			return nil
		})

		suite.Require().NoError(err)

		cmtv := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, res.Metadata().ID())

		cmtv.Metadata().Labels().Set(omni.LabelCluster, clusterName)

		cmtv.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion

		suite.Require().NoError(suite.state.Create(suite.ctx, cmtv, state.WithCreateOwner(omnictrl.NewTalosUpgradeStatusController().ControllerName)))
	}

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	for _, res := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, res.Metadata().ID()).Metadata(),
			func(version *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
				versionSpec := version.TypedSpec().Value

				assertions.Equal(defaultSchematic, versionSpec.SchematicId)
			},
		)
	}

	assertResource(
		&suite.OmniSuite,
		*omni.NewTalosUpgradeStatus(resources.DefaultNamespace, clusterName).Metadata(),
		func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
			assertions.Equal(specs.TalosUpgradeStatusSpec_Done, res.TypedSpec().Value.Phase)
		},
	)
}

func TestTalosUpgradeStatusSuite(t *testing.T) {
	suite.Run(t, new(TalosUpgradeStatusSuite))
}
