// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package auth

import (
	"fmt"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/plain"

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

// GetSignatureVerifier from the public key resource.
func GetSignatureVerifier(pubKey *PublicKey) (message.SignatureVerifier, error) {
	switch pubKey.TypedSpec().Value.Type {
	case specs.PublicKeySpec_PGP, specs.PublicKeySpec_UNKNOWN:
		key, err := pgpcrypto.NewKeyFromArmored(string(pubKey.TypedSpec().Value.GetPublicKey()))
		if err != nil {
			return nil, err
		}

		return pgp.NewKey(key)
	case specs.PublicKeySpec_PLAIN:
		return plain.ParseKey(pubKey.TypedSpec().Value.PublicKey)
	}

	return nil, fmt.Errorf("unsupported key type %s", pubKey.TypedSpec().Value.Type)
}
