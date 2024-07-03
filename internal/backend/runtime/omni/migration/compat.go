// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MaintenanceConfigPatchPrefix deprecated maintenance config patch prefix.
const MaintenanceConfigPatchPrefix = "950-maintenance-config-"

// getConfigPatches collects all machine config patches.
func getConfigPatches(ctx context.Context, r controller.Reader, machine resource.Resource, machineSet *omni.MachineSet, prefix string) ([]*omni.ConfigPatch, error) {
	clusterName, ok := machine.Metadata().Labels().Get(prefix + deprecatedCluster)
	if !ok {
		return nil, fmt.Errorf("cluster machine %q doesn't have cluster label set", machine.Metadata().ID())
	}

	clusterPatchList, err := safe.ReaderListAll[*omni.ConfigPatch](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(prefix+deprecatedCluster, clusterName)),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cluster name %q, error listing cluster config patches: %w",
			clusterName,
			err,
		)
	}

	machinePatchList, err := safe.ReaderListAll[*omni.ConfigPatch](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(prefix+"machine", machine.Metadata().ID())),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cluster name %q, cluster machine %q, error listing machine config patches: %w",
			clusterName,
			machine.Metadata().ID(),
			err,
		)
	}

	clusterPatches := make([]*omni.ConfigPatch, 0, clusterPatchList.Len())
	machineSetPatches := make([]*omni.ConfigPatch, 0, clusterPatchList.Len())
	clusterMachinePatches := make([]*omni.ConfigPatch, 0, clusterPatchList.Len())

	for iter := clusterPatchList.Iterator(); iter.Next(); {
		patch := iter.Value()

		machineSetName, machineSetOk := patch.Metadata().Labels().Get(prefix + "machine-set")
		clusterMachineName, clusterMachineOk := patch.Metadata().Labels().Get(prefix + "cluster-machine")

		switch {
		// machine set patch
		case machineSetOk && machineSetName == machineSet.Metadata().ID():
			machineSetPatches = append(machineSetPatches, patch)
		// cluster machine patch
		case clusterMachineOk && clusterMachineName == machine.Metadata().ID():
			clusterMachinePatches = append(clusterMachinePatches, patch)
		// cluster patch
		case !machineSetOk && !clusterMachineOk:
			clusterPatches = append(clusterPatches, patch)
		}
	}

	patches := make([]*omni.ConfigPatch, 0, clusterPatchList.Len()+machinePatchList.Len())

	patches = append(patches, clusterPatches...)
	patches = append(patches, machineSetPatches...)
	patches = append(patches, clusterMachinePatches...)

	for iter := machinePatchList.Iterator(); iter.Next(); {
		patch := iter.Value()

		patches = append(patches, patch)
	}

	return patches, nil
}
