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

// NewMachineRequestSet creates new MachineRequestSet state.
func NewMachineRequestSet(id string) *MachineRequestSet {
	return typed.NewResource[MachineRequestSetSpec, MachineRequestSetExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineRequestSetType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineRequestSetSpec{}),
	)
}

// MachineRequestSetType is the type of MachineRequestSet resource.
//
// tsgen:MachineRequestSetType
const MachineRequestSetType = resource.Type("MachineRequestSets.omni.sidero.dev")

// MachineRequestSet resource describes a set of machine requests which are using the same configuration.
type MachineRequestSet = typed.Resource[MachineRequestSetSpec, MachineRequestSetExtension]

// MachineRequestSetSpec wraps specs.MachineRequestSetSpec.
type MachineRequestSetSpec = protobuf.ResourceSpec[specs.MachineRequestSetSpec, *specs.MachineRequestSetSpec]

// MachineRequestSetExtension providers auxiliary methods for MachineRequestSet resource.
type MachineRequestSetExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineRequestSetExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineRequestSetType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "ProviderID",
				JSONPath: "{.providerid}",
			},
			{
				Name:     "MachineCount",
				JSONPath: "{.machinecount}",
			},
		},
	}
}
