// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package configpatch provides a helper to lookup config patches by machine/machine-set.
package configpatch

import (
	"context"
	"fmt"
	"iter"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xiter"

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

	return slices.Collect(xiter.Filter(
		func(configPatch *omni.ConfigPatch) bool {
			return configPatch.Metadata().Phase() == resource.PhaseRunning
		},
		xiter.Concat(
			asIter(machine, machineSet, clusterPatchList),
			machinePatchList.All(),
		),
	)), nil
}

func asIter(machine *omni.ClusterMachine, machineSet *omni.MachineSet, clusterPatchList safe.List[*omni.ConfigPatch]) iter.Seq[*omni.ConfigPatch] {
	return func(yield func(*omni.ConfigPatch) bool) {
		for i := range 3 {
			for patch := range clusterPatchList.All() {
				machineSetName, machineSetOk := patch.Metadata().Labels().Get(omni.LabelMachineSet)
				clusterMachineName, clusterMachineOk := patch.Metadata().Labels().Get(omni.LabelClusterMachine)

				var toYield *omni.ConfigPatch

				switch {
				// machine set patch
				case i == 1 && machineSetOk && machineSetName == machineSet.Metadata().ID():
					toYield = patch
				// cluster machine patch
				case i == 2 && clusterMachineOk && clusterMachineName == machine.Metadata().ID():
					toYield = patch
				// cluster patch
				case i == 0 && !machineSetOk && !clusterMachineOk:
					toYield = patch
				}

				if toYield != nil && !yield(toYield) {
					return
				}
			}
		}
	}
}
