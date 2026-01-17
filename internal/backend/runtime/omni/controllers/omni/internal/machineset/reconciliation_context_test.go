// Copyright (c) 2025 Sidero Labs, Inc.
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

type fakePatchHelper struct {
	patches map[string][]*omni.ConfigPatch
}

func (fph *fakePatchHelper) Get(cm *omni.ClusterMachine, _ *omni.MachineSet) ([]*omni.ConfigPatch, error) {
	if fph.patches == nil {
		return nil, nil
	}

	return fph.patches[cm.Metadata().ID()], nil
}

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

	var configPatches []*omni.ConfigPatch

	version := resource.VersionUndefined.Next()

	//nolint:govet
	for _, tt := range []struct {
		name                         string
		machineSet                   *specs.MachineSetSpec
		lbUnhealthy                  bool
		machineSetNodes              []*omni.MachineSetNode
		machineStatuses              []*system.ResourceLabels[*omni.MachineStatus]
		clusterMachines              []*omni.ClusterMachine
		clusterMachineConfigStatuses []*omni.ClusterMachineConfigStatus
		clusterMachineConfigPatches  []*omni.ClusterMachineConfigPatches
		expectedQuota                machineset.ChangeQuota
		expectedTearingDown          []string
		expectedUpdating             []string

		expectedToUpdate   []string
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
				Update:   1,
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
				withUpdateInputVersions(withVersion(omni.NewClusterMachine("a"), version), configPatches...),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
				Update:   1,
			},
		},
		{
			name: "running machines 1 to update",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Unset,
			},
			clusterMachines: []*omni.ClusterMachine{
				withVersion(omni.NewClusterMachine("a"), version),
			},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", omni.NewMachineSet("")),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
				Update:   1,
			},
			expectedToUpdate: []string{"a"},
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
				withUpdateInputVersions(withVersion(omni.NewClusterMachine("a"), version), configPatches...),
				withUpdateInputVersions(withVersion(omni.NewClusterMachine("b"), version), configPatches...),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("b"), version),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
				omni.NewClusterMachineConfigPatches("b"),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
				Update:   1,
			},
			expectedToTeardown: []string{"a", "b"},
		},
		{
			name: "update locked noop",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Rolling,
			},
			machineSetNodes: []*omni.MachineSetNode{
				lockedMachine,
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus](lockedMachine.Metadata().ID()),
			},
			clusterMachines: []*omni.ClusterMachine{
				withVersion(omni.NewClusterMachine("b"), version),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("b"), version),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
				Update:   1,
			},
		},
		{
			name: "update locked quota",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Rolling,
			},
			machineSetNodes: []*omni.MachineSetNode{
				lockedMachine,
				omni.NewMachineSetNode("c", omni.NewMachineSet("")),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus](lockedMachine.Metadata().ID()),
				system.NewResourceLabels[*omni.MachineStatus]("c"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withVersion(omni.NewClusterMachine("b"), version),
				withVersion(omni.NewClusterMachine("c"), version),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("b"), version),
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("c"), version),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("b"),
				omni.NewClusterMachineConfigPatches("c"),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
				Update:   1,
			},
			expectedToUpdate: []string{"c"},
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
				tearingDown(withUpdateInputVersions(withVersion(omni.NewClusterMachine("a"), version), configPatches...)),
				withUpdateInputVersions(withVersion(omni.NewClusterMachine("b"), version), configPatches...),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("b"), version),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
				omni.NewClusterMachineConfigPatches("b"),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: 0,
				Update:   1,
			},
			expectedToTeardown:  []string{"b"},
			expectedTearingDown: []string{"a"},
		},
		{
			name: "1 updating",
			machineSet: &specs.MachineSetSpec{
				UpdateStrategy: specs.MachineSetSpec_Rolling,
				DeleteStrategy: specs.MachineSetSpec_Unset,
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine("a"),
			},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", omni.NewMachineSet("")),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				omni.NewClusterMachineConfigStatus("a"),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
				Update:   0,
			},
			expectedToUpdate: []string{"a"},
			expectedUpdating: []string{"a"},
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
				tearingDown(withUpdateInputVersions(withVersion(omni.NewClusterMachine("a"), version), configPatches...)),
			},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", omni.NewMachineSet("")),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
			},
			expectedQuota: machineset.ChangeQuota{
				Teardown: -1,
				Update:   3,
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
				Update:   1,
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
				&fakePatchHelper{},
				tt.machineSetNodes,
				tt.machineStatuses,
				tt.clusterMachines,
				tt.clusterMachineConfigStatuses,
				tt.clusterMachineConfigPatches,
				nil,
			)

			require.NoError(err)

			q := rc.CalculateQuota()

			assert.EqualValues(tt.expectedQuota, q)

			assert.EqualValues(tt.expectedToCreate, rc.GetMachinesToCreate(), "machines to create do not match")
			assert.EqualValues(tt.expectedToTeardown, rc.GetMachinesToTeardown(), "machines to destroy do not match")
			assert.EqualValues(tt.expectedToUpdate, rc.GetMachinesToUpdate(), "machines to update do not match")

			updating := rc.GetUpdatingMachines()
			assert.EqualValues(len(tt.expectedUpdating), len(updating), "updating machines do not match")

			for _, id := range tt.expectedUpdating {
				assert.True(updating.Contains(id))
			}

			tearingDown := rc.GetTearingDownMachines()
			assert.EqualValues(len(tt.expectedTearingDown), len(tearingDown), "tearing down machines do not match")

			for _, id := range tt.expectedTearingDown {
				assert.True(tearingDown.Contains(id))
			}
		})
	}
}
