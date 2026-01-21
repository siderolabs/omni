// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
)

func TestWorkersHandler(t *testing.T) {
	machineSet := omni.NewMachineSet("test")

	version := resource.VersionUndefined.Next()

	//nolint:govet
	for _, tt := range []struct {
		name                         string
		machineSet                   *specs.MachineSetSpec
		machineSetNodes              []*omni.MachineSetNode
		machineStatuses              []*system.ResourceLabels[*omni.MachineStatus]
		clusterMachines              []*omni.ClusterMachine
		clusterMachineConfigStatuses []*omni.ClusterMachineConfigStatus
		clusterMachineConfigPatches  []*omni.ClusterMachineConfigPatches

		pendingConfigPatches map[resource.ID][]*omni.ConfigPatch

		expectRequeue    bool
		expectOperations []machineset.Operation
	}{
		{
			name:       "create nodes",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
				omni.NewMachineSetNode("b", machineSet),
				omni.NewMachineSetNode("c", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
				system.NewResourceLabels[*omni.MachineStatus]("c"),
			},
			expectOperations: []machineset.Operation{
				&machineset.Create{ID: "a"},
				&machineset.Create{ID: "b"},
				&machineset.Create{ID: "c"},
			},
		},
		{
			name:       "create nodes when scaling down",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
				omni.NewMachineSetNode("b", machineSet),
				omni.NewMachineSetNode("c", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
				system.NewResourceLabels[*omni.MachineStatus]("c"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("a"), version)),
				tearingDownNoFinalizers(omni.NewClusterMachine("b")),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
			},
			expectOperations: []machineset.Operation{
				&machineset.Destroy{ID: "b"},
				&machineset.Create{ID: "c"},
			},
		},
		{
			name:       "destroy multiple",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("a"), version)),
				tearingDownNoFinalizers(omni.NewClusterMachine("b")),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("c"), version)),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("d"), version)),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
			},
			expectOperations: []machineset.Operation{
				&machineset.Destroy{ID: "b"},
				&machineset.Teardown{ID: "c"},
				&machineset.Teardown{ID: "d"},
			},
		},
		{
			name:       "destroy, create and update at the same time",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
				omni.NewMachineSetNode("b", machineSet),
				omni.NewMachineSetNode("c", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
				system.NewResourceLabels[*omni.MachineStatus]("c"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("a"), version)),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("c"), version)),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("d"), version)),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
				omni.NewClusterMachineConfigPatches("c"),
				omni.NewClusterMachineConfigPatches("d"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("d"), version),
			},
			pendingConfigPatches: map[resource.ID][]*omni.ConfigPatch{
				"a": {
					omni.NewConfigPatch("1"),
				},
			},
			expectOperations: []machineset.Operation{
				&machineset.Create{
					ID: "b",
				},
				&machineset.Teardown{
					ID: "d",
				},
			},
		},
		{
			name:       "no actions",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("a"), version)),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
			},
			expectOperations: []machineset.Operation{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			machineSet.TypedSpec().Value = tt.machineSet
			machineSet.Metadata().Labels().Set(omni.LabelCluster, tt.name)

			cluster := omni.NewCluster(tt.name)
			cluster.TypedSpec().Value.TalosVersion = "v1.6.0"
			cluster.TypedSpec().Value.KubernetesVersion = "v1.29.0"

			rc, err := machineset.NewReconciliationContext(
				cluster,
				machineSet,
				newHealthyLB(cluster.Metadata().ID()),
				tt.machineSetNodes,
				tt.machineStatuses,
				tt.clusterMachines,
				tt.clusterMachineConfigStatuses,
				nil,
				nil,
			)

			require.NoError(err)

			operations := machineset.ReconcileWorkers(rc)

			require.Equal(len(tt.expectOperations), len(operations), "%#v", operations)

			for i, op := range operations {
				expected := tt.expectOperations[i]

				switch value := op.(type) {
				case *machineset.Create:
					create, ok := expected.(*machineset.Create)

					require.True(ok, "the operation at %d is not create", i)
					require.Equal(create.ID, value.ID)
				case *machineset.Teardown:
					destroy, ok := expected.(*machineset.Teardown)

					require.True(ok, "the operation at %d is not destroy", i)
					require.Equal(destroy.ID, value.ID)
				}
			}
		})
	}
}
