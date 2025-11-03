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

// NewSchematicConfiguration creates new schematic configuration resource.
func NewSchematicConfiguration(ns string, id resource.ID) *SchematicConfiguration {
	return typed.NewResource[SchematicConfigurationSpec, SchematicConfigurationExtension](
		resource.NewMetadata(ns, SchematicConfigurationType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SchematicConfigurationSpec{}),
	)
}

const (
	// SchematicConfigurationType is the type of the SchematicConfiguration resource.
	// tsgen:SchematicConfigurationType
	SchematicConfigurationType = resource.Type("SchematicConfigurations.omni.sidero.dev")
)

// SchematicConfiguration describes desired machine schematic for the particular machine, machine set or cluster.
type SchematicConfiguration = typed.Resource[SchematicConfigurationSpec, SchematicConfigurationExtension]

// SchematicConfigurationSpec wraps specs.SchematicConfigurationSpec.
type SchematicConfigurationSpec = protobuf.ResourceSpec[specs.SchematicConfigurationSpec, *specs.SchematicConfigurationSpec]

// SchematicConfigurationExtension provides auxiliary methods for SchematicConfiguration resource.
type SchematicConfigurationExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SchematicConfigurationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SchematicConfigurationType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Schematic ID",
				JSONPath: "{.schematicid}",
			},
			{
				Name:     "Talos Version",
				JSONPath: "{.talosversion}",
			},
		},
	}
}
