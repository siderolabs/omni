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

// NewSchematic creates new schematic resource.
func NewSchematic(ns string, id resource.ID) *Schematic {
	return typed.NewResource[SchematicSpec, SchematicExtension](
		resource.NewMetadata(ns, SchematicType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SchematicSpec{}),
	)
}

const (
	// SchematicType is the type of the Schematic resource.
	// tsgen:SchematicType
	SchematicType = resource.Type("Schematics.omni.sidero.dev")
)

// Schematic describes previosly generated image factory schematic.
type Schematic = typed.Resource[SchematicSpec, SchematicExtension]

// SchematicSpec wraps specs.SchematicSpec.
type SchematicSpec = protobuf.ResourceSpec[specs.SchematicSpec, *specs.SchematicSpec]

// SchematicExtension provides auxiliary methods for Schematic resource.
type SchematicExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SchematicExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SchematicType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
