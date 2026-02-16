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

// NewMachineSetConfigStatus creates new MachineSetConfigStatus resource.
func NewMachineSetConfigStatus(id resource.ID) *MachineSetConfigStatus {
	return typed.NewResource[MachineSetConfigStatusSpec, MachineSetConfigStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineSetConfigStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineSetConfigStatusSpec{}),
	)
}

const (
	// MachineSetConfigStatusType is the type of the MachineSetConfigStatus resource.
	// tsgen:MachineSetConfigStatusType
	MachineSetConfigStatusType = resource.Type("MachineSetConfigStatuses.omni.sidero.dev")
)

// MachineSetConfigStatus describes current machine set configuration status.
type MachineSetConfigStatus = typed.Resource[MachineSetConfigStatusSpec, MachineSetConfigStatusExtension]

// MachineSetConfigStatusSpec wraps specs.MachineSetConfigStatusSpec.
type MachineSetConfigStatusSpec = protobuf.ResourceSpec[specs.MachineSetConfigStatusSpec, *specs.MachineSetConfigStatusSpec]

// MachineSetConfigStatusExtension provides auxiliary methods for MachineSetConfigStatus resource.
type MachineSetConfigStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineSetConfigStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineSetConfigStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
