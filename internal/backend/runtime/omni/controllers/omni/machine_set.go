// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineSetController manages MachineSet resource lifecycle.
type MachineSetController = cleanup.Controller[*omni.MachineSet]

// NewMachineSetController returns a new MachineSet controller.
func NewMachineSetController() *MachineSetController {
	return cleanup.NewController(
		cleanup.Settings[*omni.MachineSet]{
			Name: "MachineSetController",
			Handler: cleanup.Combine(
				cleanup.RemoveOutputs[*omni.MachineSetNode](func(machineSet *omni.MachineSet) state.ListOption {
					return state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()))
				}),
				cleanup.RemoveOutputs[*omni.ExtensionsConfiguration](func(machineSet *omni.MachineSet) state.ListOption {
					return state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()))
				}),

				withFinalizerCheck(cleanup.RemoveOutputs[*omni.ConfigPatch](func(machineSet *omni.MachineSet) state.ListOption {
					clusterName, _ := machineSet.Metadata().Labels().Get(omni.LabelCluster)

					return state.WithLabelQuery(
						resource.LabelEqual(omni.LabelCluster, clusterName),
						resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()),
					)
				}), func(machineSet *omni.MachineSet) error {
					_, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
					if !ok {
						return fmt.Errorf("machine set doesn't have %q label", omni.LabelCluster)
					}

					return nil
				}),
			),
		},
	)
}
