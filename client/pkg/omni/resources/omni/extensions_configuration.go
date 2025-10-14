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

// NewExtensionsConfiguration creates new extensions configuration resource.
func NewExtensionsConfiguration(ns string, id resource.ID) *ExtensionsConfiguration {
	return typed.NewResource[ExtensionsConfigurationSpec, ExtensionsConfigurationExtension](
		resource.NewMetadata(ns, ExtensionsConfigurationType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ExtensionsConfigurationSpec{}),
	)
}

const (
	// ExtensionsConfigurationType is the type of the ExtensionsConfiguration resource.
	// tsgen:ExtensionsConfigurationType
	ExtensionsConfigurationType = resource.Type("ExtensionsConfigurations.omni.sidero.dev")
)

// ExtensionsConfiguration describes desired machine extensions list for a particular machine, machine set or cluster.
type ExtensionsConfiguration = typed.Resource[ExtensionsConfigurationSpec, ExtensionsConfigurationExtension]

// ExtensionsConfigurationSpec wraps specs.ExtensionsConfigurationSpec.
type ExtensionsConfigurationSpec = protobuf.ResourceSpec[specs.ExtensionsConfigurationSpec, *specs.ExtensionsConfigurationSpec]

// ExtensionsConfigurationExtension provides auxiliary methods for ExtensionsConfiguration resource.
type ExtensionsConfigurationExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ExtensionsConfigurationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtensionsConfigurationType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
