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

// NewMachineOperation creates new MachineOperation state.
func NewMachineOperation(id string) *MachineOperation {
	return typed.NewResource[MachineOperationSpec, MachineOperationExtension](
		resource.NewMetadata(resources.EphemeralNamespace, MachineOperationType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineOperationSpec{}),
	)
}

// MachineOperationType is the type of MachineOperation resource.
//
// tsgen:MachineOperationType
const MachineOperationType = resource.Type("MachineOperations.omni.sidero.dev")

// MachineOperation resource describes the install or upgrade the lifecycle manager runs on a machine, or the last failed one.
// It is written by the lifecycle manager, and it exists only while there is an operation in flight, a failed one or a kept stage.
//
// MachineOperation resource ID is a Machine UUID.
type MachineOperation = typed.Resource[MachineOperationSpec, MachineOperationExtension]

// MachineOperationSpec wraps specs.MachineOperationSpec.
type MachineOperationSpec = protobuf.ResourceSpec[specs.MachineOperationSpec, *specs.MachineOperationSpec]

// MachineOperationExtension provides auxiliary methods for MachineOperation resource.
type MachineOperationExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineOperationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineOperationType,
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
