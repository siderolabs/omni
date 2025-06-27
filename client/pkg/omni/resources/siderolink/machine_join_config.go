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

// NewMachineJoinConfig creates new MachineJoinConfig state.
func NewMachineJoinConfig(id string) *MachineJoinConfig {
	return typed.NewResource[MachineJoinConfigSpec, MachineJoinConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineJoinConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineJoinConfigSpec{}),
	)
}

// MachineJoinConfigType is the type of MachineJoinConfig resource.
//
// tsgen:MachineJoinConfigType
const MachineJoinConfigType = resource.Type("MachineJoinConfigs.omni.sidero.dev")

// MachineJoinConfig resource is the per machine SideroLink join config.
type MachineJoinConfig = typed.Resource[MachineJoinConfigSpec, MachineJoinConfigExtension]

// MachineJoinConfigSpec wraps specs.MachineJoinConfigSpec.
type MachineJoinConfigSpec = protobuf.ResourceSpec[specs.MachineJoinConfigSpec, *specs.MachineJoinConfigSpec]

// MachineJoinConfigExtension providers auxiliary methods for MachineJoinConfig resource.
type MachineJoinConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineJoinConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineJoinConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     nil,
	}
}
