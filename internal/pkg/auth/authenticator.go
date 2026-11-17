// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"context"

	"github.com/siderolabs/go-api-signature/pkg/message"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// Authenticator represents an authenticator.
type Authenticator struct {
	Verifier message.SignatureVerifier
	Identity string
	UserID   string
	Role     role.Role
	// Suspended is set when the instance is suspended.
	Suspended bool
}

// AuthenticatorFunc represents a function that returns an authenticator for the given public key fingerprint.
type AuthenticatorFunc func(ctx context.Context, fingerprint string) (*Authenticator, error)

// ContextWithAuthenticator returns a child context carrying the caller who signed the request with the key of the given fingerprint.
func ContextWithAuthenticator(ctx context.Context, authenticator *Authenticator, fingerprint string) context.Context {
	ctx = ctxstore.WithValue(ctx, UserIDContextKey{UserID: authenticator.UserID})
	ctx = ctxstore.WithValue(ctx, IdentityContextKey{Identity: authenticator.Identity})
	ctx = ctxstore.WithValue(ctx, RoleContextKey{Role: authenticator.Role})
	ctx = ctxstore.WithValue(ctx, FingerprintContextKey{Fingerprint: fingerprint})

	if authenticator.Suspended {
		ctx = ctxstore.WithValue(ctx, SuspendedContextKey{})
	}

	return ctx
}
