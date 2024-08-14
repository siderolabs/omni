// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package configpatch provides a helper to lookup config patches by machine/machine-set.
package configpatch

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Helper provides a way to lookup config patches by machine/machine-set.
type Helper struct {
	allConfigPatches safe.List[*omni.ConfigPatch]
}

// NewHelper creates a new config patch helper.
func NewHelper(ctx context.Context, r controller.Reader) (*Helper, error) {
	allConfigPatches, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, r)
	if err != nil {
		return nil, err
	}

	return &Helper{
		allConfigPatches: allConfigPatches,
	}, nil
}

// Get collects all machine config patches.
func (h *Helper) Get(machine *omni.ClusterMachine, machineSet *omni.MachineSet) ([]*omni.ConfigPatch, error) {
	clusterName, ok := machine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, fmt.Errorf("cluster machine %q doesn't have cluster label set", machine.Metadata().ID())
	}

	clusterPatchList := h.allConfigPatches.FilterLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName))

	machinePatchList := h.allConfigPatches.FilterLabelQuery(resource.LabelEqual(omni.LabelMachine, machine.Metadata().ID()))

	clusterPatches := make([]*omni.ConfigPatch, 0, clusterPatchList.Len())
	machineSetPatches := make([]*omni.ConfigPatch, 0, clusterPatchList.Len())
	clusterMachinePatches := make([]*omni.ConfigPatch, 0, clusterPatchList.Len())

	for patch := range clusterPatchList.All() {
		machineSetName, machineSetOk := patch.Metadata().Labels().Get(omni.LabelMachineSet)
		clusterMachineName, clusterMachineOk := patch.Metadata().Labels().Get(omni.LabelClusterMachine)

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
	patches = slices.AppendSeq(patches, machinePatchList.All())

	return xslices.Filter(patches, func(configPatch *omni.ConfigPatch) bool {
		return configPatch.Metadata().Phase() == resource.PhaseRunning
	}), nil
}
