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

// NewMachineDiscoveryServiceConfig creates a new MachineDiscoveryServiceConfig resource.
func NewMachineDiscoveryServiceConfig(id resource.ID) *MachineDiscoveryServiceConfig {
	return typed.NewResource[MachineDiscoveryServiceConfigSpec, MachineDiscoveryServiceConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineDiscoveryServiceConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineDiscoveryServiceConfigSpec{}),
	)
}

const (
	// MachineDiscoveryServiceConfigType is the type of the MachineDiscoveryServiceConfig resource.
	MachineDiscoveryServiceConfigType = resource.Type("MachineDiscoveryServiceConfigs.omni.sidero.dev")
)

// MachineDiscoveryServiceConfig keeps the discovery service endpoint Omni resolved from the
// machine config it generated and applied. It is the trusted source for the endpoint, replacing
// the value read back from the node.
type MachineDiscoveryServiceConfig = typed.Resource[MachineDiscoveryServiceConfigSpec, MachineDiscoveryServiceConfigExtension]

// MachineDiscoveryServiceConfigSpec wraps specs.MachineDiscoveryServiceConfigSpec.
type MachineDiscoveryServiceConfigSpec = protobuf.ResourceSpec[specs.MachineDiscoveryServiceConfigSpec, *specs.MachineDiscoveryServiceConfigSpec]

// MachineDiscoveryServiceConfigExtension provides auxiliary methods for MachineDiscoveryServiceConfig resource.
type MachineDiscoveryServiceConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineDiscoveryServiceConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineDiscoveryServiceConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Discovery Service Endpoint",
				JSONPath: "{.discoveryserviceendpoint}",
			},
		},
	}
}
