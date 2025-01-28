// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"context"

	"github.com/siderolabs/go-api-signature/pkg/message"

	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// Authenticator represents an authenticator.
type Authenticator struct {
	Verifier message.SignatureVerifier
	Identity string
	UserID   string
	Role     role.Role
}

// AuthenticatorFunc represents a function that returns an authenticator for the given public key fingerprint.
type AuthenticatorFunc func(ctx context.Context, fingerprint string) (*Authenticator, error)
