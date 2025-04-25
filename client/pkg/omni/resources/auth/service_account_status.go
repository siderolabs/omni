// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package auth

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewServiceAccountStatus creates a new ServiceAccountStatus resource.
func NewServiceAccountStatus(id string) *ServiceAccountStatus {
	return typed.NewResource[ServiceAccountStatusSpec, ServiceAccountStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, ServiceAccountStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ServiceAccountStatusSpec{}),
	)
}

const (
	// ServiceAccountStatusType is the type of ServiceAccountStatus resource.
	//
	// tsgen:ServiceAccountStatusType
	ServiceAccountStatusType = resource.Type("ServiceAccountStatuses.omni.sidero.dev")
)

// ServiceAccountStatus resource describes a service account status.
type ServiceAccountStatus = typed.Resource[ServiceAccountStatusSpec, ServiceAccountStatusExtension]

// ServiceAccountStatusSpec wraps specs.ServiceAccountStatusSpec.
type ServiceAccountStatusSpec = protobuf.ResourceSpec[specs.ServiceAccountStatusSpec, *specs.ServiceAccountStatusSpec]

// ServiceAccountStatusExtension providers auxiliary methods for ServiceAccountStatus resource.
type ServiceAccountStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ServiceAccountStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ServiceAccountStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
