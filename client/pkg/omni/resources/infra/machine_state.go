// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachineState creates a new MachineState resource.
func NewMachineState(id string) *MachineState {
	return typed.NewResource[MachineStateSpec, MachineStateExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, InfraMachineStateType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraMachineStateSpec{}),
	)
}

const (
	// InfraMachineStateType is the type of MachineState resource.
	//
	// tsgen:InfraMachineStateType
	InfraMachineStateType = resource.Type("InfraMachineStates.omni.sidero.dev")
)

// MachineState resource describes an infra machine state.
//
// It is a shared resource between the respective infra provider and Omni - both can read and write it.
type MachineState = typed.Resource[MachineStateSpec, MachineStateExtension]

// MachineStateSpec wraps specs.MachineStateSpec.
type MachineStateSpec = protobuf.ResourceSpec[specs.InfraMachineStateSpec, *specs.InfraMachineStateSpec]

// MachineStateExtension providers auxiliary methods for MachineState resource.
type MachineStateExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineStateExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfraMachineStateType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Installed",
				JSONPath: "{.installed}",
			},
		},
	}
}
