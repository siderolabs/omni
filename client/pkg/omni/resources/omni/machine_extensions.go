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

// NewMachineExtensions creates new MachineExtensions resource.
func NewMachineExtensions(id resource.ID) *MachineExtensions {
	return typed.NewResource[MachineExtensionsSpec, MachineExtensionsExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineExtensionsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineExtensionsSpec{}),
	)
}

const (
	// MachineExtensionsType is the type of the MachineExtensions resource.
	// tsgen:MachineExtensionsType
	MachineExtensionsType = resource.Type("MachineExtensions.omni.sidero.dev")
)

// MachineExtensions describes the desired extensions list for a particular machine.
type MachineExtensions = typed.Resource[MachineExtensionsSpec, MachineExtensionsExtension]

// MachineExtensionsSpec wraps specs.MachineExtensionsSpec.
type MachineExtensionsSpec = protobuf.ResourceSpec[specs.MachineExtensionsSpec, *specs.MachineExtensionsSpec]

// MachineExtensionsExtension provides auxiliary methods for MachineExtensions resource.
type MachineExtensionsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineExtensionsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineExtensionsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Extensions",
				JSONPath: "{.extensions}",
			},
		},
	}
}
