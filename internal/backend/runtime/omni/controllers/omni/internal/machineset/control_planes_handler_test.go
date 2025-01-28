// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
)

//nolint:maintidx
func TestControlPlanesHandler(t *testing.T) {
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
		patches                      map[string][]*omni.ConfigPatch
		etcdStatus                   *check.EtcdStatusResult

		expectError      bool
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
				omni.NewClusterMachine(resources.DefaultNamespace, "a"),
				tearingDown(omni.NewClusterMachine(resources.DefaultNamespace, "b")),
			},
			expectOperations: []machineset.Operation{
				&machineset.Create{ID: "c"},
			},
		},
		{
			name:       "destroy tearing down",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine(resources.DefaultNamespace, "a"),
				tearingDownNoFinalizers(omni.NewClusterMachine(resources.DefaultNamespace, "b")),
				omni.NewClusterMachine(resources.DefaultNamespace, "c"),
			},
			expectOperations: []machineset.Operation{
				&machineset.Teardown{ID: "b"},
			},
		},
		{
			name:       "destroy one",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine(resources.DefaultNamespace, "a"),
				omni.NewClusterMachine(resources.DefaultNamespace, "c"),
				omni.NewClusterMachine(resources.DefaultNamespace, "d"),
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
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine(resources.DefaultNamespace, "a"),
				omni.NewClusterMachine(resources.DefaultNamespace, "c"),
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
			etcdStatus: &check.EtcdStatusResult{
				Members: map[string]check.EtcdMemberStatus{
					"a": {
						Healthy: true,
					},
				},
				HealthyMembers: 1,
			},
		},
		{
			name:       "update with outdated",
			machineSet: &specs.MachineSetSpec{},
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", machineSet),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", machineSet),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "a"), version.Next())),
				withUpdateInputVersions[*omni.ClusterMachine, *omni.ConfigPatch](withVersion(omni.NewClusterMachine(resources.DefaultNamespace, "b"), version)),
			},
			clusterMachineConfigPatches: []*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "a"),
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "b"),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "a"), version),
				withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "b"), version),
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
					omni.NewConfigPatch(resources.DefaultNamespace, "1"),
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

			cluster := omni.NewCluster(resources.DefaultNamespace, tt.name)
			cluster.TypedSpec().Value.TalosVersion = "v1.5.4"
			cluster.TypedSpec().Value.KubernetesVersion = "v1.29.1"

			patchHelper := &fakePatchHelper{
				patches: tt.patches,
			}

			rc, err := machineset.NewReconciliationContext(
				cluster,
				machineSet,
				newHealthyLB(cluster.Metadata().ID()),
				patchHelper,
				tt.machineSetNodes,
				tt.clusterMachines,
				tt.clusterMachineConfigStatuses,
				tt.clusterMachineConfigPatches,
				nil,
			)

			require.NoError(err)

			etcdStatus := tt.etcdStatus
			if etcdStatus == nil {
				etcdStatus = &check.EtcdStatusResult{}
			}

			operations, err := machineset.ReconcileControlPlanes(context.Background(), rc, func(context.Context) (*check.EtcdStatusResult, error) {
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
