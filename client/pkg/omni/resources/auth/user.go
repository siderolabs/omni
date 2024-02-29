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

// NewUser creates a new User resource.
func NewUser(ns, id string) *User {
	return typed.NewResource[UserSpec, UserExtension](
		resource.NewMetadata(ns, UserType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.UserSpec{}),
	)
}

const (
	// UserType is the type of User resource.
	//
	// tsgen:UserType
	UserType = resource.Type("Users.omni.sidero.dev")
)

// User resource describes a user.
type User = typed.Resource[UserSpec, UserExtension]

// UserSpec wraps specs.UserSpec.
type UserSpec = protobuf.ResourceSpec[specs.UserSpec, *specs.UserSpec]

// UserExtension providers auxiliary methods for User resource.
type UserExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (UserExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             UserType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
