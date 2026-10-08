// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth0

import (
	"context"
	"crypto/rsa"
)

// NewTestIDTokenVerifier builds an IDTokenVerifier that checks signatures against the given key instead of the Auth0 JWKS.
func NewTestIDTokenVerifier(issuer, clientID string, key *rsa.PublicKey, requireReauthForNewKeys bool) (*IDTokenVerifier, error) {
	return newIDTokenVerifier(issuer, clientID, func(context.Context) (any, error) { return key, nil }, requireReauthForNewKeys)
}
