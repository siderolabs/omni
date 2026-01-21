// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
)

//nolint:maintidx
func TestControlPlanesHandler(t *testing.T) {
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
		patches                      map[string][]*omni.ConfigPatch
		etcdStatus                   *check.EtcdStatusResult

		expectError      bool
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
				omni.NewClusterMachine("a"),
				tearingDown(omni.NewClusterMachine("b")),
			},
			expectOperations: []machineset.Operation{
				&machineset.Create{ID: "c"},
			},
		},
		{
			name:       "destroy tearing down",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine("a"),
				tearingDownNoFinalizers(omni.NewClusterMachine("b")),
				omni.NewClusterMachine("c"),
			},
			expectOperations: []machineset.Operation{
				&machineset.Teardown{ID: "b"},
			},
		},
		{
			name:       "destroy one",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine("a"),
				omni.NewClusterMachine("c"),
				omni.NewClusterMachine("d"),
			},
			expectOperations: []machineset.Operation{
				&machineset.Teardown{
					ID: "c",
				},
			},
			etcdStatus: &check.EtcdStatusResult{
				Members: map[string]check.EtcdMemberStatus{
					"a": {
						Healthy: true,
					},
					"c": {
						Healthy: true,
					},
				},
				HealthyMembers: 2,
			},
		},
		{
			name:       "requeue due to unhealthy etcd",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine("a"),
				omni.NewClusterMachine("c"),
			},
			expectError:      true,
			expectOperations: []machineset.Operation{},
			etcdStatus: &check.EtcdStatusResult{
				Members: map[string]check.EtcdMemberStatus{
					"a": {
						Healthy: true,
					},
					"c": {
						Healthy: false,
					},
				},
				HealthyMembers: 1,
			},
		},
		{
			name:       "update with outdated",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", machineSet),
				omni.NewMachineSetNode("b", machineSet),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("a"), version.Next())),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine("b"), version)),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches("a"),
				omni.NewClusterMachineConfigPatches("b"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("a"), version),
				withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus("b"), version),
			},
			expectOperations: []machineset.Operation{},
			etcdStatus: &check.EtcdStatusResult{
				Members: map[string]check.EtcdMemberStatus{
					"a": {
						Healthy: true,
					},
					"b": {
						Healthy: true,
					},
				},
				HealthyMembers: 1,
			},
			patches: map[string][]*omni.ConfigPatch{
				"b": {
					omni.NewConfigPatch("1"),
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
			etcdStatus: &check.EtcdStatusResult{
				Members: map[string]check.EtcdMemberStatus{
					"a": {
						Healthy: true,
					},
				},
				HealthyMembers: 1,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			machineSet.TypedSpec().Value = tt.machineSet
			machineSet.Metadata().Labels().Set(omni.LabelCluster, tt.name)

			cluster := omni.NewCluster(tt.name)
			cluster.TypedSpec().Value.TalosVersion = "v1.5.4"
			cluster.TypedSpec().Value.KubernetesVersion = "v1.29.1"

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

			etcdStatus := tt.etcdStatus
			if etcdStatus == nil {
				etcdStatus = &check.EtcdStatusResult{}
			}

			operations, err := machineset.ReconcileControlPlanes(t.Context(), rc, func(context.Context) (*check.EtcdStatusResult, error) {
				return etcdStatus, nil
			})

			if !tt.expectError {
				require.NoError(err)
			}

			require.Equal(len(tt.expectOperations), len(operations))

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
