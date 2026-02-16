// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
)

func newClusterMachineStatus(id string, stage specs.ClusterMachineStatusSpec_Stage, ready, connected bool) *omni.ClusterMachineStatus {
	res := omni.NewClusterMachineStatus(id)
	res.TypedSpec().Value.Ready = ready
	res.TypedSpec().Value.Stage = stage

	if connected {
		res.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
	}

	return res
}

func newMachineSet(machineCount int) *omni.MachineSet {
	res := omni.NewMachineSet("test")
	res.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		MachineCount: uint32(machineCount),
	}

	return res
}

//nolint:maintidx
func TestStatusHandler(t *testing.T) {
	ms := omni.NewMachineSet("")

	var patches []*omni.ConfigPatch

	//nolint:govet
	for _, tt := range []struct {
		name                         string
		machineSet                   *omni.MachineSet
		machineSetNodes              []*omni.MachineSetNode
		clusterMachines              []*omni.ClusterMachine
		machineStatuses              []*system.ResourceLabels[*omni.MachineStatus]
		clusterMachineStatuses       []*omni.ClusterMachineStatus
		clusterMachineConfigStatuses []*omni.ClusterMachineConfigStatus
		pendingMachineUpdates        []*omni.MachinePendingUpdates
		expectedStatus               *specs.MachineSetStatusSpec
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
				omni.NewMachineSetNode("a", ms),
				omni.NewMachineSetNode("b", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine("b"), patches...),
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
				omni.NewMachineSetNode("a", ms),
				omni.NewMachineSetNode("b", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine("a"),
				omni.NewClusterMachine("b"),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				omni.NewClusterMachineConfigStatus("a"),
				omni.NewClusterMachineConfigStatus("b"),
			},
			pendingMachineUpdates: []*omni.MachinePendingUpdates{
				withSpecSetter(omni.NewMachinePendingUpdates("a"), func(res *omni.MachinePendingUpdates) {
					res.TypedSpec().Value.ConfigDiff = "-"
				}),
				withSpecSetter(omni.NewMachinePendingUpdates("b"), func(res *omni.MachinePendingUpdates) {
					res.TypedSpec().Value.ConfigDiff = "-"
				}),
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
				ConfigHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
		{
			name: "scaling down",
			machineSetNodes: []*omni.MachineSetNode{
				omni.NewMachineSetNode("a", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine("b"), patches...),
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
				omni.NewMachineSetNode("a", ms),
				omni.NewMachineSetNode("b", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
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
				omni.NewMachineSetNode("a", ms),
				omni.NewMachineSetNode("b", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine("b"), patches...),
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
				omni.NewMachineSetNode("a", ms),
				omni.NewMachineSetNode("b", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine("b"), patches...),
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
				omni.NewMachineSetNode("b", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
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
				omni.NewMachineSetNode("a", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			machineSet: newMachineSet(4),
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
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
				omni.NewMachineSetNode("a", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
			},
			machineSet: newMachineSet(0),
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
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
				omni.NewMachineSetNode("a", ms),
				omni.NewMachineSetNode("b", ms),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				withUpdateInputVersions(omni.NewClusterMachine("a"), patches...),
				withUpdateInputVersions(omni.NewClusterMachine("b"), patches...),
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
				withLabels(omni.NewMachineSetNode("a", ms), pair.MakePair(omni.MachineLocked, "")),
				withLabels(omni.NewMachineSetNode("b", ms), pair.MakePair(omni.MachineLocked, "")),
			},
			machineStatuses: []*system.ResourceLabels[*omni.MachineStatus]{
				system.NewResourceLabels[*omni.MachineStatus]("a"),
				system.NewResourceLabels[*omni.MachineStatus]("b"),
			},
			clusterMachines: []*omni.ClusterMachine{
				omni.NewClusterMachine("a"),
				omni.NewClusterMachine("b"),
			},
			clusterMachineStatuses: []*omni.ClusterMachineStatus{
				newClusterMachineStatus("a", specs.ClusterMachineStatusSpec_RUNNING, true, true),
				newClusterMachineStatus("b", specs.ClusterMachineStatusSpec_RUNNING, true, true),
			},
			clusterMachineConfigStatuses: []*omni.ClusterMachineConfigStatus{
				omni.NewClusterMachineConfigStatus("a"),
				omni.NewClusterMachineConfigStatus("b"),
			},
			pendingMachineUpdates: []*omni.MachinePendingUpdates{
				withSpecSetter(omni.NewMachinePendingUpdates("a"), func(res *omni.MachinePendingUpdates) {
					res.TypedSpec().Value.ConfigDiff = "something"
				}),
				withSpecSetter(omni.NewMachinePendingUpdates("b"), func(res *omni.MachinePendingUpdates) {
					res.TypedSpec().Value.ConfigDiff = "something"
				}),
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
				ConfigHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			machineSet := tt.machineSet
			if machineSet == nil {
				machineSet = omni.NewMachineSet("test")
			}

			machineSet.Metadata().Labels().Set(omni.LabelCluster, "test")

			require := require.New(t)

			clusterMachineConfigStatuses := make([]*omni.ClusterMachineConfigStatus, 0, len(tt.clusterMachines))

			for _, cm := range tt.clusterMachines {
				version := resource.VersionUndefined.Next()

				cm.Metadata().SetVersion(version)

				clusterMachineConfigStatuses = append(clusterMachineConfigStatuses, withSpecSetter(
					withClusterMachineConfigVersionSetter(omni.NewClusterMachineConfigStatus(cm.Metadata().ID()), version),
					func(r *omni.ClusterMachineConfigStatus) {
						r.TypedSpec().Value.ClusterMachineConfigSha256 = cm.Metadata().ID()
					},
				))
			}

			if tt.clusterMachineConfigStatuses != nil {
				clusterMachineConfigStatuses = tt.clusterMachineConfigStatuses
			}

			rc, err := machineset.NewReconciliationContext(
				omni.NewCluster("test"),
				machineSet,
				newHealthyLB("test"),
				tt.machineSetNodes,
				tt.machineStatuses,
				tt.clusterMachines,
				clusterMachineConfigStatuses,
				tt.pendingMachineUpdates,
				tt.clusterMachineStatuses,
			)

			require.NoError(err)

			machineSetStatus := omni.NewMachineSetStatus("doesn't matter")
			machineSetConfigStatus := omni.NewMachineSetConfigStatus("doesn't matter")

			machineset.ReconcileStatus(rc, machineSetStatus, machineSetConfigStatus)

			require.True(tt.expectedStatus.EqualVT(machineSetStatus.TypedSpec().Value), "machine set status doesn't match %s", cmp.Diff(
				tt.expectedStatus,
				machineSetStatus.TypedSpec().Value,
				IgnoreUnexported(tt.expectedStatus, &specs.Machines{}, &specs.MachineSetSpec_MachineAllocation{}),
			))

			expectedConfigStatus := &specs.MachineSetConfigStatusSpec{
				ConfigUpdatesAllowed: true,
				ShouldResetGraceful:  true,
			}

			require.True(expectedConfigStatus.EqualVT(machineSetConfigStatus.TypedSpec().Value), "machine set config status doesn't match %s", cmp.Diff(
				expectedConfigStatus,
				machineSetConfigStatus.TypedSpec().Value,
				IgnoreUnexported(expectedConfigStatus, &specs.Machines{}, &specs.MachineSetSpec_MachineAllocation{}),
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
