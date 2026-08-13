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

// NewMachineInstallDiskStatus creates new MachineInstallDiskStatus resource.
func NewMachineInstallDiskStatus(id resource.ID) *MachineInstallDiskStatus {
	return typed.NewResource[MachineInstallDiskStatusSpec, MachineInstallDiskStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineInstallDiskStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineInstallDiskStatusSpec{}),
	)
}

const (
	// MachineInstallDiskStatusType is the type of the MachineInstallDiskStatus resource.
	// tsgen:MachineInstallDiskStatusType
	MachineInstallDiskStatusType = resource.Type("MachineInstallDiskStatuses.omni.sidero.dev")
)

// MachineInstallDiskStatus describes the resolved install disk of a machine.
type MachineInstallDiskStatus = typed.Resource[MachineInstallDiskStatusSpec, MachineInstallDiskStatusExtension]

// MachineInstallDiskStatusSpec wraps specs.MachineInstallDiskStatusSpec.
type MachineInstallDiskStatusSpec = protobuf.ResourceSpec[specs.MachineInstallDiskStatusSpec, *specs.MachineInstallDiskStatusSpec]

// MachineInstallDiskStatusExtension provides auxiliary methods for MachineInstallDiskStatus resource.
type MachineInstallDiskStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineInstallDiskStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineInstallDiskStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Disk",
				JSONPath: "{.disk}",
			},
			{
				Name:     "Message",
				JSONPath: "{.message}",
			},
		},
	}
}
