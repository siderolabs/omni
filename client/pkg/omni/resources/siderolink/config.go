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

const (
	// ConfigType is the type of Config resource.
	//
	// tsgen:ConfigType
	ConfigType = resource.Type("Configs.omni.sidero.dev")
	// ConfigID is the config resource name.
	//
	// tsgen:ConfigID
	ConfigID = resource.ID("siderolink-config")
)

// NewConfig creates new Config resource.
func NewConfig() *Config {
	return typed.NewResource[ConfigSpec, ConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, ConfigType, ConfigID, resource.VersionUndefined),
		protobuf.NewResourceSpec(
			&specs.SiderolinkConfigSpec{},
		),
	)
}

// Config resource keeps connected nodes state.
type Config = typed.Resource[ConfigSpec, ConfigExtension]

// ConfigSpec wraps specs.SiderolinkConfigSpec.
type ConfigSpec = protobuf.ResourceSpec[specs.SiderolinkConfigSpec, *specs.SiderolinkConfigSpec]

// ConfigExtension providers auxiliary methods for Config resource.
type ConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: Namespace,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}
