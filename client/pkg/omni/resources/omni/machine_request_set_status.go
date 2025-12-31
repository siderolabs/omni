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

// NewMachineRequestSetStatus creates new MachineRequestSetStatus.
func NewMachineRequestSetStatus(id string) *MachineRequestSetStatus {
	return typed.NewResource[MachineRequestSetStatusSpec, MachineRequestSetStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineRequestSetStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineRequestSetStatusSpec{}),
	)
}

// MachineRequestSetStatusType is the type of MachineRequestSetStatus resource.
//
// tsgen:MachineRequestSetStatusType
const MachineRequestSetStatusType = resource.Type("MachineRequestSetStatuses.omni.sidero.dev")

// MachineRequestSetStatus resource describes the status of the machine pool.
type MachineRequestSetStatus = typed.Resource[MachineRequestSetStatusSpec, MachineRequestSetStatusExtension]

// MachineRequestSetStatusSpec wraps specs.MachineRequestSetStatusSpec.
type MachineRequestSetStatusSpec = protobuf.ResourceSpec[specs.MachineRequestSetStatusSpec, *specs.MachineRequestSetStatusSpec]

// MachineRequestSetStatusExtension providers auxiliary methods for MachineRequestSetStatus resource.
type MachineRequestSetStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineRequestSetStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineRequestSetStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
