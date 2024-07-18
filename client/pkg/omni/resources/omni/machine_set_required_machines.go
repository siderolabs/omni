// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachineSetRequiredMachines creates new MachineSetRequiredMachines resource.
func NewMachineSetRequiredMachines(ns string, id resource.ID) *MachineSetRequiredMachines {
	return typed.NewResource[MachineSetRequiredMachinesSpec, MachineSetRequiredMachinesExtension](
		resource.NewMetadata(ns, MachineSetRequiredMachinesType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineSetRequiredMachinesSpec{}),
	)
}

const (
	// MachineSetRequiredMachinesType is the type of the MachineSetRequiredMachines resource.
	// tsgen:MachineSetRequiredMachinesType
	MachineSetRequiredMachinesType = resource.Type("MachineSetRequiredMachines.omni.sidero.dev")
)

// MachineSetRequiredMachines describes machine set resource.
type MachineSetRequiredMachines = typed.Resource[MachineSetRequiredMachinesSpec, MachineSetRequiredMachinesExtension]

// MachineSetRequiredMachinesSpec wraps specs.MachineSetRequiredMachinesSpec.
type MachineSetRequiredMachinesSpec = protobuf.ResourceSpec[specs.MachineSetRequiredMachinesSpec, *specs.MachineSetRequiredMachinesSpec]

// MachineSetRequiredMachinesExtension provides auxiliary methods for MachineSetRequiredMachines resource.
type MachineSetRequiredMachinesExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineSetRequiredMachinesExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineSetRequiredMachinesType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Required Additional Machines",
				JSONPath: "{.requiredadditionalmachines}",
			},
		},
	}
}
