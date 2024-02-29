// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// CurrentUserID is the default and the only allowed ID for CurrentUser resource.
//
// tsgen:CurrentUserID
const CurrentUserID = "current"

// NewCurrentUser creates a new CurrentUser resource.
func NewCurrentUser() *CurrentUser {
	return typed.NewResource[CurrentUserSpec, CurrentUserExtension](
		resource.NewMetadata(resources.VirtualNamespace, CurrentUserType, CurrentUserID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.CurrentUserSpec{}),
	)
}

const (
	// CurrentUserType is the type of CurrentUser resource.
	//
	// tsgen:CurrentUserType
	CurrentUserType = resource.Type("CurrentUsers.omni.sidero.dev")
)

// CurrentUser resource describes a user current user.
type CurrentUser = typed.Resource[CurrentUserSpec, CurrentUserExtension]

// CurrentUserSpec wraps specs.CurrentUserSpec.
type CurrentUserSpec = protobuf.ResourceSpec[specs.CurrentUserSpec, *specs.CurrentUserSpec]

// CurrentUserExtension providers auxiliary methods for CurrentUser resource.
type CurrentUserExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (CurrentUserExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CurrentUserType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Identity",
				JSONPath: "{.identity}",
			},
			{
				Name:     "Role",
				JSONPath: "{.role}",
			},
		},
	}
}
