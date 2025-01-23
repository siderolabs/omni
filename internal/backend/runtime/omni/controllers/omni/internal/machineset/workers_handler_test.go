// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
)

func TestWorkersHandler(t *testing.T) {
	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "test")

	version := resource.VersionUndefined.Next()

	//nolint:govet
	for _, tt := range []struct {
		name                         string
		machineSet                   *specs.MachineSetSpec
		machineSetNodes              []*omni.MachineSetNode
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
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", machineSet),
				omni.NewMachineSetNode(resources.DefaultNamespace, "c", machineSet),
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
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", machineSet),
				omni.NewMachineSetNode(resources.DefaultNamespace, "c", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "a"), version)),
				tearingDownNoFinalizers(omni.NewClusterMachine(resources.DefaultNamespace, "b")),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "a"), version),
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
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "a"), version)),
				tearingDownNoFinalizers(omni.NewClusterMachine(resources.DefaultNamespace, "b")),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "c"), version)),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "d"), version)),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "a"), version),
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
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", machineSet),
				omni.NewMachineSetNode(resources.DefaultNamespace, "c", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "a"), version)),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "c"), version)),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "d"), version)),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "a"),
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "c"),
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "d"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "a"), version),
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "d"), version),
			},
			pendingConfigPatches: map[resource.ID][]*omni.ConfigPatch{
				"a": {
					omni.NewConfigPatch(resources.DefaultNamespace, "1"),
				},
			},
			expectOperations: []machineset.Operation{
				&machineset.Create{
					ID: "b",
				},
				&machineset.Teardown{
					ID: "d",
				},
				&machineset.Update{
					ID: "a",
				},
			},
		},
		{
			name:       "update a machine",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine(resources.DefaultNamespace, "a"),
			},
			expectOperations: []machineset.Operation{
				&machineset.Update{
					ID: "a",
				},
			},
		},
		{
			name:       "no actions",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "a"), version)),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "a"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "a"), version),
			},
			expectOperations: []machineset.Operation{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			machineSet.TypedSpec().Value = tt.machineSet
			machineSet.Metadata().Labels().Set(omni.LabelCluster, tt.name)

			cluster := omni.NewCluster(resources.DefaultNamespace, tt.name)
			cluster.TypedSpec().Value.TalosVersion = "v1.6.0"
			cluster.TypedSpec().Value.KubernetesVersion = "v1.29.0"

			rc, err := machineset.NewReconciliationContext(
				cluster,
				machineSet,
				newHealthyLB(cluster.Metadata().ID()),
				&fakePatchHelper{
					tt.pendingConfigPatches,
				},
				tt.machineSetNodes,
				tt.clusterMachines,
				tt.clusterMachineConfigStatuses,
				tt.clusterMachineConfigPatches,
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
				case *machineset.Update:
					update, ok := expected.(*machineset.Update)

					require.True(ok, "the operation at %d is not update", i)
					require.Equal(update.ID, value.ID)
				case *machineset.Teardown:
					destroy, ok := expected.(*machineset.Teardown)

					require.True(ok, "the operation at %d is not destroy", i)
					require.Equal(destroy.ID, value.ID)
				}
			}
		})
	}
}
