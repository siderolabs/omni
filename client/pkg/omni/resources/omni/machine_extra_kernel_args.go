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

// NewMachineExtraKernelArgs creates new MachineExtraKernelArgs configuration resource.
func NewMachineExtraKernelArgs(ns string, id resource.ID) *MachineExtraKernelArgs {
	return typed.NewResource[MachineExtraKernelArgsSpec, MachineExtraKernelArgsExtension](
		resource.NewMetadata(ns, MachineExtraKernelArgsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineExtraKernelArgsSpec{}),
	)
}

const (
	// MachineExtraKernelArgsType is the type of the MachineExtraKernelArgs resource.
	// tsgen:MachineExtraKernelArgsType
	MachineExtraKernelArgsType = resource.Type("MachineExtraKernelArgs.omni.sidero.dev")
)

// MachineExtraKernelArgs describes the desired extraKernelArgs list for a particular machine.
type MachineExtraKernelArgs = typed.Resource[MachineExtraKernelArgsSpec, MachineExtraKernelArgsExtension]

// MachineExtraKernelArgsSpec wraps specs.MachineExtraKernelArgsSpec.
type MachineExtraKernelArgsSpec = protobuf.ResourceSpec[specs.MachineExtraKernelArgsSpec, *specs.MachineExtraKernelArgsSpec]

// MachineExtraKernelArgsExtension provides auxiliary methods for MachineExtraKernelArgs resource.
type MachineExtraKernelArgsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineExtraKernelArgsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineExtraKernelArgsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "args",
				JSONPath: "{.args}",
			},
		},
	}
}
