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

// NewPublicKey creates a new PublicKey resource.
func NewPublicKey(ns, id string) *PublicKey {
	return typed.NewResource[PublicKeySpec, PublicKeyExtension](
		resource.NewMetadata(ns, PublicKeyType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.PublicKeySpec{}),
	)
}

const (
	// PublicKeyType is the type of PublicKey resource.
	//
	// tsgen:PublicKeyType
	PublicKeyType = resource.Type("PublicKeys.omni.sidero.dev")
)

// PublicKey resource describes a user public key.
type PublicKey = typed.Resource[PublicKeySpec, PublicKeyExtension]

// PublicKeySpec wraps specs.PublicKeySpec.
type PublicKeySpec = protobuf.ResourceSpec[specs.PublicKeySpec, *specs.PublicKeySpec]

// PublicKeyExtension providers auxiliary methods for PublicKey resource.
type PublicKeyExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (PublicKeyExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PublicKeyType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
