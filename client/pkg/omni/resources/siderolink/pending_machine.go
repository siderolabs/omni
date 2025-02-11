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

// NewPendingMachine creates a new machine being in the limbo state.
func NewPendingMachine(id string, spec *specs.SiderolinkSpec) *PendingMachine {
	return typed.NewResource[PendingMachineSpec, PendingMachineExtension](
		resource.NewMetadata(resources.EphemeralNamespace, PendingMachineType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(spec),
	)
}

// PendingMachineType is the type of PendingMachine resource.
//
// tsgen:PendingMachineType
const PendingMachineType = resource.Type("PendingMachines.omni.sidero.dev")

// PendingMachine resource keeps the state of the machine being in the limbo state.
type PendingMachine = typed.Resource[PendingMachineSpec, PendingMachineExtension]

// PendingMachineSpec wraps specs.SiderolinkSpec.
type PendingMachineSpec = protobuf.ResourceSpec[specs.SiderolinkSpec, *specs.SiderolinkSpec]

// PendingMachineExtension providers auxiliary methods for PendingMachine resource.
type PendingMachineExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (PendingMachineExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PendingMachineType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
