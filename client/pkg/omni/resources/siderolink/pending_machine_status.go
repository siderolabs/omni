// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewPendingMachineStatus creates a new machine being in the limbo state.
func NewPendingMachineStatus(id string) *PendingMachineStatus {
	return typed.NewResource[PendingMachineStatusSpec, PendingMachineStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, PendingMachineStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.PendingMachineStatusSpec{}),
	)
}

// PendingMachineStatusType is the type of PendingMachineStatus resource.
//
// tsgen:PendingMachineStatusType
const PendingMachineStatusType = resource.Type("PendingMachineStatuses.omni.sidero.dev")

// PendingMachineStatus resource keeps the state of the machine being in the limbo state.
type PendingMachineStatus = typed.Resource[PendingMachineStatusSpec, PendingMachineStatusExtension]

// PendingMachineStatusSpec wraps specs.SiderolinkSpec.
type PendingMachineStatusSpec = protobuf.ResourceSpec[specs.PendingMachineStatusSpec, *specs.PendingMachineStatusSpec]

// PendingMachineStatusExtension providers auxiliary methods for PendingMachineStatus resource.
type PendingMachineStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (PendingMachineStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PendingMachineStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
