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

// NewAPIConfig creates new SiderolinkAPIConfig.
func NewAPIConfig() *APIConfig {
	return typed.NewResource[APIConfigSpec, APIConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, APIConfigType, ConfigID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SiderolinkAPIConfigSpec{}),
	)
}

// APIConfigType is the type of SiderolinkAPIConfig resource.
//
// tsgen:APIConfigType
const APIConfigType = resource.Type("SiderolinkAPIConfigs.omni.sidero.dev")

// APIConfig resource keeps SideroLink connection params (API URL, log and events ports).
type APIConfig = typed.Resource[APIConfigSpec, APIConfigExtension]

// APIConfigSpec wraps specs.APIConfigSpec.
type APIConfigSpec = protobuf.ResourceSpec[specs.SiderolinkAPIConfigSpec, *specs.SiderolinkAPIConfigSpec]

// APIConfigExtension providers auxiliary methods for SiderolinkAPIConfig resource.
type APIConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (APIConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             APIConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: Namespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
