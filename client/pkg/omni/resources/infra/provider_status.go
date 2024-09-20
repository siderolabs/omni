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

// NewProviderStatus creates a new InfraProviderStatus resource.
func NewProviderStatus(id string) *ProviderStatus {
	return typed.NewResource[ProviderStatusSpec, ProviderStatusExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, InfraProviderStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraProviderStatusSpec{}),
	)
}

const (
	// InfraProviderStatusType is the type of InfraProviderStatus resource.
	//
	// tsgen:InfraProviderStatusType
	InfraProviderStatusType = resource.Type("InfraProviderStatuses.omni.sidero.dev")
)

// ProviderStatus resource describes a infra provider status.
// The status is reported back by the infra provider.
type ProviderStatus = typed.Resource[ProviderStatusSpec, ProviderStatusExtension]

// ProviderStatusSpec wraps specs.ProviderStatusSpec.
type ProviderStatusSpec = protobuf.ResourceSpec[specs.InfraProviderStatusSpec, *specs.InfraProviderStatusSpec]

// ProviderStatusExtension providers auxiliary methods for InfraProviderStatus resource.
type ProviderStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ProviderStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfraProviderStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: "{.name}",
			},
			{
				Name:     "Description",
				JSONPath: "{.description}",
			},
		},
	}
}
