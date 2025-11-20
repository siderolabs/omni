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

// NewCloudPlatformConfig creates a new CloudPlatformConfig resource.
func NewCloudPlatformConfig(id string) *CloudPlatformConfig {
	return typed.NewResource[CloudPlatformConfigSpec, CloudPlatformConfigExtension](
		resource.NewMetadata(resources.VirtualNamespace, CloudPlatformConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.PlatformConfigSpec{}),
	)
}

const (
	// CloudPlatformConfigType is the type of CloudPlatformConfig resource.
	//
	// tsgen:CloudPlatformConfigType
	CloudPlatformConfigType = resource.Type("CloudPlatformConfigs.omni.sidero.dev")
)

// CloudPlatformConfig resource describes a cloud platform configuration for the installation media wizard.
type CloudPlatformConfig = typed.Resource[CloudPlatformConfigSpec, CloudPlatformConfigExtension]

// CloudPlatformConfigSpec wraps specs.CloudPlatformConfigSpec.
type CloudPlatformConfigSpec = protobuf.ResourceSpec[specs.PlatformConfigSpec, *specs.PlatformConfigSpec]

// CloudPlatformConfigExtension providers auxiliary methods for CloudPlatformConfig resource.
type CloudPlatformConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (CloudPlatformConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CloudPlatformConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
