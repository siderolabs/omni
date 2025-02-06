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

// NewBMCConfig creates a new BMCConfig resource.
func NewBMCConfig(id string) *BMCConfig {
	return typed.NewResource[BMCConfigSpec, BMCConfigExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, BMCConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.BMCConfigSpec{}),
	)
}

const (
	// BMCConfigType is the type of BMCConfig resource.
	//
	// tsgen:BMCConfigType
	BMCConfigType = resource.Type("BMCConfigs.omni.sidero.dev")
)

// BMCConfig resource describes a machine request.
type BMCConfig = typed.Resource[BMCConfigSpec, BMCConfigExtension]

// BMCConfigSpec wraps specs.BMCConfigSpec.
type BMCConfigSpec = protobuf.ResourceSpec[specs.BMCConfigSpec, *specs.BMCConfigSpec]

// BMCConfigExtension providers auxiliary methods for BMCConfig resource.
type BMCConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (BMCConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BMCConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
