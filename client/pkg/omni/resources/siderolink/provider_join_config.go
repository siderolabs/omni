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

// NewProviderJoinConfig creates new ProviderJoinConfig state.
func NewProviderJoinConfig(id string) *ProviderJoinConfig {
	return typed.NewResource[ProviderJoinConfigSpec, ProviderJoinConfigExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, ProviderJoinConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ProviderJoinConfigSpec{}),
	)
}

// ProviderJoinConfigType is the type of ProviderJoinConfig resource.
//
// tsgen:ProviderJoinConfigType
const ProviderJoinConfigType = resource.Type("ProviderJoinConfigs.omni.sidero.dev")

// ProviderJoinConfig resource is the per provider SideroLink join config.
type ProviderJoinConfig = typed.Resource[ProviderJoinConfigSpec, ProviderJoinConfigExtension]

// ProviderJoinConfigSpec wraps specs.ProviderJoinConfigSpec.
type ProviderJoinConfigSpec = protobuf.ResourceSpec[specs.ProviderJoinConfigSpec, *specs.ProviderJoinConfigSpec]

// ProviderJoinConfigExtension providers auxiliary methods for ProviderJoinConfig resource.
type ProviderJoinConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ProviderJoinConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ProviderJoinConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns:     nil,
	}
}
