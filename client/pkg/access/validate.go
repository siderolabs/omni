// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import (
	"errors"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
	pgpcrypto "github.com/ProtonMail/gopenpgp/v3/crypto"
	authpb "github.com/siderolabs/go-api-signature/api/auth"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PublicKey is returned by the result of validation.
type PublicKey struct {
	Expiration time.Time
	ID         string
	Username   string
	Data       []byte
}

// ValidatePublicKey validates the public key in the request and returns a publicKey.
func ValidatePublicKey(keypb *authpb.PublicKey, opts ...pgp.ValidationOption) (PublicKey, error) {
	if keypb.GetPgpData() == nil && keypb.GetWebauthnData() == nil {
		return PublicKey{}, errors.New("no public key data provided")
	}

	if keypb.GetWebauthnData() != nil {
		return PublicKey{}, status.Error(codes.Unimplemented, "unimplemented") // todo: implement webauthn
	}

	return ValidatePGPPublicKey(keypb.GetPgpData(), opts...)
}

// ValidatePGPPublicKey validates the public key in the request and returns a publicKey.
func ValidatePGPPublicKey(armored []byte, opts ...pgp.ValidationOption) (PublicKey, error) {
	pgpKey, err := pgpcrypto.NewKeyFromArmored(string(armored))
	if err != nil {
		return PublicKey{}, err
	}

	key, err := pgp.NewKey(pgpKey)
	if err != nil {
		return PublicKey{}, err
	}

	err = key.Validate(opts...)
	if err != nil {
		return PublicKey{}, err
	}

	if key.IsPrivate() {
		return PublicKey{}, errors.New("PGP key contains private key")
	}

	sig, identity := pgpKey.GetEntity().PrimaryIdentity(time.Time{}, &packet.Config{})
	if sig == nil || identity == nil {
		return PublicKey{}, errors.New("PGP key has no primary identity")
	}

	lifetimeSecs := sig.KeyLifetimeSecs
	if lifetimeSecs == nil {
		return PublicKey{}, errors.New("PGP key has no expiration")
	}

	expiration := pgpKey.GetEntity().PrimaryKey.CreationTime.Add(time.Duration(*lifetimeSecs) * time.Second)

	return PublicKey{
		Data:       armored,
		ID:         pgpKey.GetFingerprint(),
		Username:   identity.UserId.Name,
		Expiration: expiration,
	}, nil
}
