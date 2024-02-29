// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"errors"
	"time"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	authpb "github.com/siderolabs/go-api-signature/api/auth"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type publicKey struct {
	expiration time.Time
	id         string
	username   string
	data       []byte
}

// validatePublicKey validates the public key in the request and returns a publicKey.
func validatePublicKey(keypb *authpb.PublicKey, opts ...pgp.ValidationOption) (publicKey, error) {
	if keypb.GetPgpData() == nil && keypb.GetWebauthnData() == nil {
		return publicKey{}, errors.New("no public key data provided")
	}

	if keypb.GetWebauthnData() != nil {
		return publicKey{}, status.Error(codes.Unimplemented, "unimplemented") // todo: implement webauthn
	}

	return validatePGPPublicKey(keypb.GetPgpData(), opts...)
}

func validatePGPPublicKey(armored []byte, opts ...pgp.ValidationOption) (publicKey, error) {
	pgpKey, err := pgpcrypto.NewKeyFromArmored(string(armored))
	if err != nil {
		return publicKey{}, err
	}

	key, err := pgp.NewKey(pgpKey)
	if err != nil {
		return publicKey{}, err
	}

	err = key.Validate(opts...)
	if err != nil {
		return publicKey{}, err
	}

	if key.IsPrivate() {
		return publicKey{}, errors.New("PGP key contains private key")
	}

	lifetimeSecs := pgpKey.GetEntity().PrimaryIdentity().SelfSignature.KeyLifetimeSecs
	if lifetimeSecs == nil {
		return publicKey{}, errors.New("PGP key has no expiration")
	}

	expiration := pgpKey.GetEntity().PrimaryKey.CreationTime.Add(time.Duration(*lifetimeSecs) * time.Second)

	return publicKey{
		data:       armored,
		id:         pgpKey.GetFingerprint(),
		username:   pgpKey.GetEntity().PrimaryIdentity().UserId.Name,
		expiration: expiration,
	}, nil
}
