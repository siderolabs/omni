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

// NewIdentity creates a new Identity resource.
func NewIdentity(id string) *Identity {
	return typed.NewResource[IdentitySpec, IdentityExtension](
		resource.NewMetadata(resources.DefaultNamespace, IdentityType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.IdentitySpec{}),
	)
}

const (
	// IdentityType is the type of Identity resource.
	//
	// tsgen:IdentityType
	IdentityType = resource.Type("Identities.omni.sidero.dev")
)

// Identity resource describes a user identity.
type Identity = typed.Resource[IdentitySpec, IdentityExtension]

// IdentitySpec wraps specs.IdentitySpec.
type IdentitySpec = protobuf.ResourceSpec[specs.IdentitySpec, *specs.IdentitySpec]

// IdentityExtension providers auxiliary methods for Identity resource.
type IdentityExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (IdentityExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             IdentityType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
