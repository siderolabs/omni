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

// NewGRPCTunnelConfig creates new GRPCTunnelConfig state.
func NewGRPCTunnelConfig(id string) *GRPCTunnelConfig {
	return typed.NewResource[GRPCTunnelConfigSpec, GRPCTunnelConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, GRPCTunnelConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.GRPCTunnelConfigSpec{}),
	)
}

// GRPCTunnelConfigType is the type of GRPCTunnelConfig resource.
//
// tsgen:GRPCTunnelConfigType
const GRPCTunnelConfigType = resource.Type("GRPCTunnelConfigs.omni.sidero.dev")

// GRPCTunnelConfig resource is the per machine gRPC tunnel mode config.
type GRPCTunnelConfig = typed.Resource[GRPCTunnelConfigSpec, GRPCTunnelConfigExtension]

// GRPCTunnelConfigSpec wraps specs.GRPCTunnelConfigSpec.
type GRPCTunnelConfigSpec = protobuf.ResourceSpec[specs.GRPCTunnelConfigSpec, *specs.GRPCTunnelConfigSpec]

// GRPCTunnelConfigExtension providers auxiliary methods for GRPCTunnelConfig resource.
type GRPCTunnelConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (GRPCTunnelConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             GRPCTunnelConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     nil,
	}
}
