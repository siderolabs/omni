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

// NewProvider creates a new InfraProvider resource.
func NewProvider(id string) *Provider {
	return typed.NewResource[ProviderSpec, ProviderExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, ProviderType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraProviderSpec{}),
	)
}

const (
	// ProviderType is the type of InfraProvider resource.
	//
	// tsgen:ProviderType
	ProviderType = resource.Type("InfraProviders.omni.sidero.dev")
)

// Provider resource describes a infra provider registered in Omni.
type Provider = typed.Resource[ProviderSpec, ProviderExtension]

// ProviderSpec wraps specs.ProviderSpec.
type ProviderSpec = protobuf.ResourceSpec[specs.InfraProviderSpec, *specs.InfraProviderSpec]

// ProviderExtension providers auxiliary methods for InfraProvider resource.
type ProviderExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ProviderExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ProviderType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
