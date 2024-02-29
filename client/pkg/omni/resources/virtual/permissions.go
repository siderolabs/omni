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

const (
	// PermissionsID is the ID of Permissions resource.
	//
	// tsgen:PermissionsID
	PermissionsID resource.ID = "permissions"
)

// NewPermissions creates a new Permissions resource.
func NewPermissions() *Permissions {
	return typed.NewResource[PermissionsSpec, PermissionsExtension](
		resource.NewMetadata(resources.VirtualNamespace, PermissionsType, PermissionsID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.PermissionsSpec{}),
	)
}

const (
	// PermissionsType is the type of Permissions resource.
	//
	// tsgen:PermissionsType
	PermissionsType = resource.Type("Permissions.omni.sidero.dev")
)

// Permissions resource describes a user's global set of permissions.
type Permissions = typed.Resource[PermissionsSpec, PermissionsExtension]

// PermissionsSpec wraps specs.PermissionsSpec.
type PermissionsSpec = protobuf.ResourceSpec[specs.PermissionsSpec, *specs.PermissionsSpec]

// PermissionsExtension providers auxiliary methods for Permissions resource.
type PermissionsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (PermissionsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PermissionsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
