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

// NewMachineOperationStatus creates new MachineOperationStatus state.
func NewMachineOperationStatus(id string) *MachineOperationStatus {
	return typed.NewResource[MachineOperationStatusSpec, MachineOperationStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, MachineOperationStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineOperationStatusSpec{}),
	)
}

// MachineOperationStatusType is the type of MachineOperationStatus resource.
//
// tsgen:MachineOperationStatusType
const MachineOperationStatusType = resource.Type("MachineOperationStatuses.omni.sidero.dev")

// MachineOperationStatus resource describes the install or upgrade Omni runs on a machine, or the last failed one.
// Every machine has one, and it is idle while the machine has no MachineOperation.
//
// MachineOperationStatus resource ID is a Machine UUID.
type MachineOperationStatus = typed.Resource[MachineOperationStatusSpec, MachineOperationStatusExtension]

// MachineOperationStatusSpec wraps specs.MachineOperationStatusSpec.
type MachineOperationStatusSpec = protobuf.ResourceSpec[specs.MachineOperationStatusSpec, *specs.MachineOperationStatusSpec]

// MachineOperationStatusExtension provides auxiliary methods for MachineOperationStatus resource.
type MachineOperationStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineOperationStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineOperationStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Stage",
				JSONPath: "{.stage}",
			},
			{
				Name:     "Message",
				JSONPath: "{.message}",
			},
		},
	}
}
