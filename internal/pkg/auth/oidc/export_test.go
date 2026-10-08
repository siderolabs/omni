// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc

import (
	"crypto"

	"github.com/coreos/go-oidc/v3/oidc"
)

// NewTestIDTokenVerifier builds an IDTokenVerifier that checks signatures against the given key instead of the provider JWKS.
func NewTestIDTokenVerifier(issuer, clientID string, key crypto.PublicKey) *IDTokenVerifier {
	return &IDTokenVerifier{
		verifier: oidc.NewVerifier(issuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{key}}, &oidc.Config{ClientID: clientID}),
	}
}
