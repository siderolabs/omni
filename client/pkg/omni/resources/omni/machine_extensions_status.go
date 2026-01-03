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

// NewMachineExtensionsStatus creates new MachineExtensionsStatus resource.
func NewMachineExtensionsStatus(id resource.ID) *MachineExtensionsStatus {
	return typed.NewResource[MachineExtensionsStatusSpec, MachineExtensionsStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineExtensionsStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineExtensionsStatusSpec{}),
	)
}

const (
	// MachineExtensionsStatusType is the type of the MachineExtensionsStatus resource.
	// tsgen:MachineExtensionsStatusType
	MachineExtensionsStatusType = resource.Type("MachineExtensionsStatuses.omni.sidero.dev")
)

// MachineExtensionsStatus represents status of each extension which is associated with the machine.
type MachineExtensionsStatus = typed.Resource[MachineExtensionsStatusSpec, MachineExtensionsStatusExtension]

// MachineExtensionsStatusSpec wraps specs.MachineExtensionsStatusSpec.
type MachineExtensionsStatusSpec = protobuf.ResourceSpec[specs.MachineExtensionsStatusSpec, *specs.MachineExtensionsStatusSpec]

// MachineExtensionsStatusExtension provides auxiliary methods for MachineExtensionsStatus resource.
type MachineExtensionsStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineExtensionsStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineExtensionsStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Extensions",
				JSONPath: "{.extensions}",
			},
		},
	}
}
