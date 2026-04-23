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

const (
	// RoleType is the type of Role resource.
	//
	// tsgen:RoleType
	RoleType = resource.Type("Roles.omni.sidero.dev")
)

// NewRole creates new Role resource.
func NewRole(id string) *Role {
	return typed.NewResource[RoleSpec, RoleExtension](
		resource.NewMetadata(resources.DefaultNamespace, RoleType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.RoleSpec{}),
	)
}

// Role resource describes an RBAC role.
type Role = typed.Resource[RoleSpec, RoleExtension]

// RoleSpec wraps specs.RoleSpec.
type RoleSpec = protobuf.ResourceSpec[specs.RoleSpec, *specs.RoleSpec]

// RoleExtension provides auxiliary methods for Role resource.
type RoleExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RoleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RoleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
