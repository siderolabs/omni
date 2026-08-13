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

// NewMachineInstallDiskConfig creates new MachineInstallDiskConfig resource.
func NewMachineInstallDiskConfig(id resource.ID) *MachineInstallDiskConfig {
	return typed.NewResource[MachineInstallDiskConfigSpec, MachineInstallDiskConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineInstallDiskConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineInstallDiskConfigSpec{}),
	)
}

const (
	// MachineInstallDiskConfigType is the type of the MachineInstallDiskConfig resource.
	// tsgen:MachineInstallDiskConfigType
	MachineInstallDiskConfigType = resource.Type("MachineInstallDiskConfigs.omni.sidero.dev")
)

// MachineInstallDiskConfig describes the user's install disk selection for a machine.
type MachineInstallDiskConfig = typed.Resource[MachineInstallDiskConfigSpec, MachineInstallDiskConfigExtension]

// MachineInstallDiskConfigSpec wraps specs.MachineInstallDiskConfigSpec.
type MachineInstallDiskConfigSpec = protobuf.ResourceSpec[specs.MachineInstallDiskConfigSpec, *specs.MachineInstallDiskConfigSpec]

// MachineInstallDiskConfigExtension provides auxiliary methods for MachineInstallDiskConfig resource.
type MachineInstallDiskConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineInstallDiskConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineInstallDiskConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Disk",
				JSONPath: "{.disk}",
			},
			{
				Name:     "Disk Selector",
				JSONPath: "{.diskselector}",
			},
		},
	}
}
