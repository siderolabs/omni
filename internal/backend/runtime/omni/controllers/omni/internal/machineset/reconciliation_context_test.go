// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
)

//nolint:maintidx
func TestReconciliationContext(t *testing.T) {
	t.Parallel()

	tearingDownMachine := omni.NewClusterMachine("a")
	tearingDownMachine.Metadata().SetPhase(resource.PhaseTearingDown)

	updatedMachine := omni.NewClusterMachine("a")
	updatedMachine.Metadata().SetVersion(resource.VersionUndefined.Next().Next())

	lockedMachine := omni.NewMachineSetNode("b", omni.NewMachineSet(""))
	lockedMachine.Metadata().Annotations().Set(omni.MachineLocked, "")

	synced := omni.NewClusterMachine("a")
	helpers.UpdateInputsAnnotation(synced)

	version := resource.VersionUndefined.Next()

	//nolint:govet
	for _, tt := range []struct {
		name                         string
		machineSet                   *specs.MachineSetSpec
		lbUnhealthy                  bool
		machineSetNodes              []*omni.MachineSetNode
		machineStatuses              []*system.ResourceLabels[*omni.MachineStatus]
		clusterMachines              []*omni.ClusterMachine
		pendingMachineUpdates        []*omni.MachinePendingUpdates
		clusterMachineConfigStatuses []*omni.ClusterMachineConfigStatus
		expectedQuota                machineset.ChangeQuota
		expectedTearingDown          []string

		expectedToCreate   []string
		expectedToTeardown []string
		expectedToDestroy  []string
	}{
		{
			name: "rolling no machines",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategyConfig: &specs.MachineSetSpec_UpdateStrategyConfig{
					Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
						MaxParallelism: 1,
					},
				},
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: 1,
			},
		},
		{
			name: "running machines",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Unset,
			},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", omni.NewMachineSet("")),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withVersion(omni.NewClusterMachine("a"), version),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
			},
		},
		{
			name: "destroy machines",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Unset,
			},
			machineSetNodes: []*omni.MachineSetNode{
				tearingDown(omni.NewMachineSetNode("a", newMachineSet(1))),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withVersion(omni.NewClusterMachine("a"), version),
				withVersion(omni.NewClusterMachine("b"), version),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("b"), version),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
			},
			expectedToTeardown: []string{"a", "b"},
		},
		{
			name: "tearing down machines",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategyConfig: &specs.MachineSetSpec_UpdateStrategyConfig{
					Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
						MaxParallelism: 1,
					},
				},
			},
			clusterMachines: []*omni.ClusterMachine{
				tearingDown(withVersion(omni.NewClusterMachine("a"), version)),
				withVersion(omni.NewClusterMachine("b"), version),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("b"), version),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: 0,
			},
			expectedToTeardown:  []string{"b"},
			expectedTearingDown: []string{"a"},
		},
		{
			name: "workers tearing down rolling 3 in parallel",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				UpdateStrategyConfig: &specs.MachineSetSpec_UpdateStrategyConfig{
					Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
						MaxParallelism: 3,
					},
				},
				DeleteStrategy: specs.MachineSetSpec_Unset,
			},
			clusterMachines: []*omni.ClusterMachine{
				tearingDown(withVersion(omni.NewClusterMachine("a"), version)),
			},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", omni.NewMachineSet("")),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
			},
			expectedTearingDown: []string{"a"},
		},
		{
			name: "destroy without finalizers",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Unset,
			},
			clusterMachines: []*omni.ClusterMachine{
				tearingDownNoFinalizers(omni.NewClusterMachine("a")),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
			},
			expectedToDestroy: []string{"a"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require := require.New(t)
			assert := assert.New(t)

			machineSet := omni.NewMachineSet(tt.name)
			machineSet.TypedSpec().Value = tt.machineSet
			machineSet.Metadata().Labels().Set(omni.LabelCluster, tt.name)

			cluster := omni.NewCluster(tt.name)
			cluster.TypedSpec().Value.TalosVersion = "v1.6.4"
			cluster.TypedSpec().Value.KubernetesVersion = "v1.29.0"

			var loadbalancerStatus *omni.LoadBalancerStatus

			if !tt.lbUnhealthy {
				loadbalancerStatus = omni.NewLoadBalancerStatus(tt.name)
				loadbalancerStatus.TypedSpec().Value.Healthy = true
			}

			rc, err := machineset.NewReconciliationContext(
				cluster,
				machineSet,
				loadbalancerStatus,
				tt.machineSetNodes,
				tt.machineStatuses,
				tt.clusterMachines,
				tt.clusterMachineConfigStatuses,
				tt.pendingMachineUpdates,
				nil,
			)

			require.NoError(err)

			q := rc.CalculateQuota()

			assert.EqualValues(tt.expectedQuota, q)

			assert.EqualValues(tt.expectedToCreate, rc.GetMachinesToCreate(), "machines to create do not match")
			assert.EqualValues(tt.expectedToTeardown, rc.GetMachinesToTeardown(), "machines to destroy do not match")

			tearingDown := rc.GetTearingDownMachines()
			assert.EqualValues(len(tt.expectedTearingDown), len(tearingDown), "tearing down machines do not match")

			for _, id := range tt.expectedTearingDown {
				assert.True(tearingDown.Contains(id))
			}
		})
	}
}
