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

// NewMachineSetStatus creates new MachineSetStatus resource.
func NewMachineSetStatus(id resource.ID) *MachineSetStatus {
	return typed.NewResource[MachineSetStatusSpec, MachineSetStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineSetStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineSetStatusSpec{}),
	)
}

const (
	// MachineSetStatusType is the type of the MachineSetStatus resource.
	// tsgen:MachineSetStatusType
	MachineSetStatusType = resource.Type("MachineSetStatuses.omni.sidero.dev")
)

// MachineSetStatus describes current machine set status.
type MachineSetStatus = typed.Resource[MachineSetStatusSpec, MachineSetStatusExtension]

// MachineSetStatusSpec wraps specs.MachineSetStatusSpec.
type MachineSetStatusSpec = protobuf.ResourceSpec[specs.MachineSetStatusSpec, *specs.MachineSetStatusSpec]

// MachineSetStatusExtension provides auxiliary methods for MachineSetStatus resource.
type MachineSetStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineSetStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineSetStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
