// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset_test

import (
	"reflect"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
)

func newClusterMachineStatus(id string, stage specs.ClusterMachineStatusSpec_Stage, ready, connected bool) *omni.ClusterMachineStatus {
	res := omni.NewClusterMachineStatus(resources.DefaultNamespace, id)
	res.TypedSpec().Value.Ready = ready
	res.TypedSpec().Value.Stage = stage

	if connected {
		res.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
	}

	return res
}

func newMachineSet(machineCount int) *omni.MachineSet {
	res := omni.NewMachineSet(resources.DefaultNamespace, "test")
	res.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		MachineCount: uint32(machineCount),
	}

	return res
}

//nolint:maintidx
func TestStatusHandler(t *testing.T) {
	ms := omni.NewMachineSet("", "")

	var patches []*omni.ConfigPatch

	//nolint:govet
	for _, tt := range []struct {
		name                   string
		machineSet             *omni.MachineSet
		machineSetNodes        []*omni.MachineSetNode
		clusterMachines        []*omni.ClusterMachine
		clusterMachineStatuses []*omni.ClusterMachineStatus
		expectedStatus         *specs.MachineSetStatusSpec
	}{
		{
			name: "running no machines",
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase:      specs.MachineSetPhase_Running,
				Ready:      true,
				Machines:   &specs.Machines{},
				ConfigHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
		{
			name: "running 2 machines",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", ms),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "b"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:     2,
					Healthy:   2,
					Connected: 2,
					Requested: 2,
				},
				ConfigHash: "fb8e20fc2e4c3f248c60c39bd652f3c1347298bb977b8b4d5903b85055620603",
			},
		},
		{
			name: "pending update 2 machines",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", ms),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine(resources.DefaultNamespace, "a"),
				omni.NewClusterMachine(resources.DefaultNamespace, "b"),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Reconfiguring,
				Ready: false,
				Machines: &specs.Machines{
					Total:     2,
					Healthy:   2,
					Connected: 2,
					Requested: 2,
				},
				ConfigHash: "fb8e20fc2e4c3f248c60c39bd652f3c1347298bb977b8b4d5903b85055620603",
			},
		},
		{
			name: "scaling down",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "b"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_ScalingDown,
				Ready: false,
				Machines: &specs.Machines{
					Total:     2,
					Healthy:   2,
					Connected: 2,
					Requested: 1,
				},
				ConfigHash: "fb8e20fc2e4c3f248c60c39bd652f3c1347298bb977b8b4d5903b85055620603",
			},
		},
		{
			name: "scaling up",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", ms),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_ScalingUp,
				Ready: false,
				Machines: &specs.Machines{
					Total:     1,
					Healthy:   1,
					Connected: 1,
					Requested: 2,
				},
				ConfigHash: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
			},
		},
		{
			name: "running 2 machines, not ready",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", ms),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "b"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, false, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: false,
				Machines: &specs.Machines{
					Total:     2,
					Healthy:   1,
					Connected: 2,
					Requested: 2,
				},
				ConfigHash: "fb8e20fc2e4c3f248c60c39bd652f3c1347298bb977b8b4d5903b85055620603",
			},
		},
		{
			name: "running 2 machines, not connected",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", ms),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "b"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, false),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: false,
				Machines: &specs.Machines{
					Total:     2,
					Healthy:   2,
					Connected: 1,
					Requested: 2,
				},
				ConfigHash: "fb8e20fc2e4c3f248c60c39bd652f3c1347298bb977b8b4d5903b85055620603",
			},
		},
		{
			name: "scaling down and scaling up",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", ms),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_ScalingUp,
				Ready: false,
				Machines: &specs.Machines{
					Total:     1,
					Healthy:   1,
					Connected: 1,
					Requested: 1,
				},
				ConfigHash: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
			},
		},
		{
			name: "scaling up machine class",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
			},
			machineSet: newMachineSet(4),
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_ScalingUp,
				Ready: false,
				Machines: &specs.Machines{
					Total:     1,
					Healthy:   1,
					Connected: 1,
					Requested: 4,
				},
				MachineAllocation: &specs.MachineSetSpec_MachineAllocation{
					MachineCount: 4,
				},
				ConfigHash: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
			},
		},
		{
			name: "scaling down machine class",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
			},
			machineSet: newMachineSet(0),
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_ScalingDown,
				Ready: false,
				Machines: &specs.Machines{
					Total:     1,
					Healthy:   1,
					Connected: 1,
					Requested: 0,
				},
				MachineAllocation: &specs.MachineSetSpec_MachineAllocation{
					MachineCount: 0,
				},
				ConfigHash: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
			},
		},
		{
			name: "unready 2 machines",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms),
				omni.NewMachineSetNode(resources.DefaultNamespace, "b", ms),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine(resources.DefaultNamespace, "b"), patches...),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_BOOTING, true, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: false,
				Machines: &specs.Machines{
					Total:     2,
					Healthy:   1,
					Connected: 2,
					Requested: 2,
				},
				ConfigHash: "fb8e20fc2e4c3f248c60c39bd652f3c1347298bb977b8b4d5903b85055620603",
			},
		},
		{
			name: "locked update 2 machines",
			machineSetNodes: []*omni.MachineSetNode{
				withLabels(omni.NewMachineSetNode(resources.DefaultNamespace, "a", ms), pair.MakePair(omni.MachineLocked, "")),
				withLabels(omni.NewMachineSetNode(resources.DefaultNamespace, "b", ms), pair.MakePair(omni.MachineLocked, "")),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine(resources.DefaultNamespace, "a"),
				omni.NewClusterMachine(resources.DefaultNamespace, "b"),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			expectedStatus: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Reconfiguring,
				Ready: false,
				Machines: &specs.Machines{
					Total:     2,
					Healthy:   2,
					Connected: 2,
					Requested: 2,
				},
				ConfigHash: "fb8e20fc2e4c3f248c60c39bd652f3c1347298bb977b8b4d5903b85055620603",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			machineSet := tt.machineSet
			if machineSet == nil {
				machineSet = omni.NewMachineSet(resources.DefaultNamespace, "test")
			}

			machineSet.Metadata().Labels().Set(omni.LabelCluster, "test")

			require := require.New(t)

			clusterMachineConfigStatuses := make([]*omni.ClusterMachineConfigStatus, 0, len(tt.clusterMachines))
			clusterMachineConfigPatches := make([]*omni.ClusterMachineConfigPatches, 0, len(tt.clusterMachines))

			for _, cm := range tt.clusterMachines {
				version := resource.VersionUndefined.Next()

				cm.Metadata().SetVersion(version)

				clusterMachineConfigStatuses = append(clusterMachineConfigStatuses, withSpecSetter(
					withClusterMachineVersionSetter(omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, cm.Metadata().ID()), version),
					func(r *omni.ClusterMachineConfigStatus) {
						r.TypedSpec().Value.ClusterMachineConfigSha256 = cm.Metadata().ID()
					},
				))

				clusterMachineConfigPatches = append(clusterMachineConfigPatches, omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, cm.Metadata().ID()))
			}

			rc, err := machineset.NewReconciliationContext(
				omni.NewCluster(resources.DefaultNamespace, "test"),
				machineSet,
				newHealthyLB("test"),
				&fakePatchHelper{},
				tt.machineSetNodes,
				tt.clusterMachines,
				clusterMachineConfigStatuses,
				clusterMachineConfigPatches,
				tt.clusterMachineStatuses,
			)

			require.NoError(err)

			machineSetStatus := omni.NewMachineSetStatus(resources.DefaultNamespace, "doesn't matter")

			machineset.ReconcileStatus(rc, machineSetStatus)

			require.True(tt.expectedStatus.EqualVT(machineSetStatus.TypedSpec().Value), "machine set status doesn't match %s", cmp.Diff(
				tt.expectedStatus,
				machineSetStatus.TypedSpec().Value,
				IgnoreUnexported(tt.expectedStatus, &specs.Machines{}, &specs.MachineSetSpec_MachineAllocation{}),
			))
		})
	}
}

func IgnoreUnexported(vals ...any) cmp.Option {
	return cmpopts.IgnoreUnexported(xslices.Map(vals, func(v any) any {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		return val.Interface()
	})...)
}
