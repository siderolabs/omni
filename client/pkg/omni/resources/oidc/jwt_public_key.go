// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oidc

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

// NewJWTPublicKey creates new JWTPublicKey state.
func NewJWTPublicKey(ns, id string) *JWTPublicKey {
	return typed.NewResource[JWTPublicKeySpec, JWTPublicKeyExtension](
		resource.NewMetadata(ns, JWTPublicKeyType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.JWTPublicKeySpec{}),
	)
}

// JWTPublicKeyType is the type of JWTPublicKey resource.
const JWTPublicKeyType = resource.Type("JWTPublicKeys.system.sidero.dev")

// JWTPublicKey resource describes current DB version (migrations state).
type JWTPublicKey = typed.Resource[JWTPublicKeySpec, JWTPublicKeyExtension]

// JWTPublicKeySpec wraps specs.JWTPublicKeySpec.
type JWTPublicKeySpec = protobuf.ResourceSpec[specs.JWTPublicKeySpec, *specs.JWTPublicKeySpec]

// JWTPublicKeyExtension providers auxiliary methods for JWTPublicKey resource.
type JWTPublicKeyExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (JWTPublicKeyExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             JWTPublicKeyType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}
