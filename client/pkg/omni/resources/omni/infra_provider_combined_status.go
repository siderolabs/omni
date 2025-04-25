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

// NewInfraProviderCombinedStatus creates a new InfraProviderCombinedStatus resource.
func NewInfraProviderCombinedStatus(id string) *InfraProviderCombinedStatus {
	return typed.NewResource[InfraProviderCombinedStatusSpec, InfraProviderCombinedStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, InfraProviderCombinedStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraProviderCombinedStatusSpec{}),
	)
}

const (
	// InfraProviderCombinedStatusType is the type of InfraProviderCombinedStatus resource.
	//
	// tsgen:InfraProviderCombinedStatusType
	InfraProviderCombinedStatusType = resource.Type("InfraProviderCombinedStatuses.omni.sidero.dev")
)

// InfraProviderCombinedStatus describes the combined status of the infra provider.
// It merges the provider health status and provider status into a single resource.
type InfraProviderCombinedStatus = typed.Resource[InfraProviderCombinedStatusSpec, InfraProviderCombinedStatusExtension]

// InfraProviderCombinedStatusSpec wraps specs.InfraProviderCombinedStatusSpec.
type InfraProviderCombinedStatusSpec = protobuf.ResourceSpec[specs.InfraProviderCombinedStatusSpec, *specs.InfraProviderCombinedStatusSpec]

// InfraProviderCombinedStatusExtension providers auxiliary methods for InfraProviderCombinedStatus resource.
type InfraProviderCombinedStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (InfraProviderCombinedStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfraProviderCombinedStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
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
