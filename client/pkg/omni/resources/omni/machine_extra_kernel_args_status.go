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

// NewMachineExtraKernelArgsStatus creates new MachineExtraKernelArgsStatus resource.
func NewMachineExtraKernelArgsStatus(ns string, id resource.ID) *MachineExtraKernelArgsStatus {
	return typed.NewResource[MachineExtraKernelArgsStatusSpec, MachineExtraKernelArgsStatusExtension](
		resource.NewMetadata(ns, MachineExtraKernelArgsStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineExtraKernelArgsStatusSpec{}),
	)
}

const (
	// MachineExtraKernelArgsStatusType is the type of the MachineExtraKernelArgsStatus resource.
	// tsgen:MachineExtraKernelArgsStatusType
	MachineExtraKernelArgsStatusType = resource.Type("MachineExtraKernelArgsStatuses.omni.sidero.dev")
)

// MachineExtraKernelArgsStatus represents status of each extension which is associated with the machine.
type MachineExtraKernelArgsStatus = typed.Resource[MachineExtraKernelArgsStatusSpec, MachineExtraKernelArgsStatusExtension]

// MachineExtraKernelArgsStatusSpec wraps specs.MachineExtraKernelArgsStatusSpec.
type MachineExtraKernelArgsStatusSpec = protobuf.ResourceSpec[specs.MachineExtraKernelArgsStatusSpec, *specs.MachineExtraKernelArgsStatusSpec]

// MachineExtraKernelArgsStatusExtension provides auxiliary methods for MachineExtraKernelArgsStatus resource.
type MachineExtraKernelArgsStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineExtraKernelArgsStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineExtraKernelArgsStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Args",
				JSONPath: "{.args}",
			},
		},
	}
}
