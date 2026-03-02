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

// NewIdentityStatus creates a new IdentityStatus resource.
func NewIdentityStatus(id string) *IdentityStatus {
	return typed.NewResource[IdentityStatusSpec, IdentityStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, IdentityStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.IdentityStatusSpec{}),
	)
}

const (
	// IdentityStatusType is the type of IdentityStatus resource.
	//
	// tsgen:IdentityStatusType
	IdentityStatusType = resource.Type("IdentityStatuses.omni.sidero.dev")
)

// IdentityStatus resource aggregates identity, user, and activity data.
type IdentityStatus = typed.Resource[IdentityStatusSpec, IdentityStatusExtension]

// IdentityStatusSpec wraps specs.IdentityStatusSpec.
type IdentityStatusSpec = protobuf.ResourceSpec[specs.IdentityStatusSpec, *specs.IdentityStatusSpec]

// IdentityStatusExtension provides auxiliary methods for IdentityStatus resource.
type IdentityStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (IdentityStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             IdentityStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Role",
				JSONPath: "{.role}",
			},
			{
				Name:     "Last Active",
				JSONPath: "{.lastactive}",
			},
		},
	}
}
