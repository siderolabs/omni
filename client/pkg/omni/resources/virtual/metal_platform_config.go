// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMetalPlatformConfig creates a new MetalPlatformConfig resource.
func NewMetalPlatformConfig(id string) *MetalPlatformConfig {
	return typed.NewResource[MetalPlatformConfigSpec, MetalPlatformConfigExtension](
		resource.NewMetadata(resources.VirtualNamespace, MetalPlatformConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.PlatformConfigSpec{}),
	)
}

const (
	// MetalPlatformConfigType is the type of MetalPlatformConfig resource.
	//
	// tsgen:MetalPlatformConfigType
	MetalPlatformConfigType = resource.Type("MetalPlatformConfigs.omni.sidero.dev")
)

// MetalPlatformConfig resource describes the metal platform configuration for the installation media wizard.
type MetalPlatformConfig = typed.Resource[MetalPlatformConfigSpec, MetalPlatformConfigExtension]

// MetalPlatformConfigSpec wraps specs.MetalPlatformConfigSpec.
type MetalPlatformConfigSpec = protobuf.ResourceSpec[specs.PlatformConfigSpec, *specs.PlatformConfigSpec]

// MetalPlatformConfigExtension providers auxiliary methods for MetalPlatformConfig resource.
type MetalPlatformConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MetalPlatformConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MetalPlatformConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
