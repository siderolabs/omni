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

// NewProviderHealthStatus creates a new InfraProviderHealthStatus resource.
func NewProviderHealthStatus(id string) *ProviderHealthStatus {
	return typed.NewResource[ProviderHealthStatusSpec, ProviderHealthStatusExtension](
		resource.NewMetadata(resources.InfraProviderEphemeralNamespace, InfraProviderHealthStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraProviderHealthStatusSpec{}),
	)
}

const (
	// InfraProviderHealthStatusType is the type of InfraProviderHealthStatus resource.
	//
	// tsgen:InfraProviderHealthStatusType
	InfraProviderHealthStatusType = resource.Type("InfraProviderHealthStatuses.omni.sidero.dev")
)

// ProviderHealthStatus resource describes an infra provider health status.
// The status is reported back by the infra provider.
type ProviderHealthStatus = typed.Resource[ProviderHealthStatusSpec, ProviderHealthStatusExtension]

// ProviderHealthStatusSpec wraps specs.ProviderHealthStatusSpec.
type ProviderHealthStatusSpec = protobuf.ResourceSpec[specs.InfraProviderHealthStatusSpec, *specs.InfraProviderHealthStatusSpec]

// ProviderHealthStatusExtension providers auxiliary methods for InfraProviderHealthStatus resource.
type ProviderHealthStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ProviderHealthStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfraProviderHealthStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderEphemeralNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Last Heartbeat Timestamp",
				JSONPath: "{.lastheartbeattimestamp}",
			},
			{
				Name:     "Error",
				JSONPath: "{.error}",
			},
		},
	}
}
