// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package authenticator answers who a signature-authenticated caller is and what role they hold.
package authenticator

import (
	"context"
	"errors"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// New returns the authenticator used by every surface that authenticates by request signature.
func New(st state.State, suspended bool) auth.AuthenticatorFunc {
	return func(ctx context.Context, fingerprint string) (*auth.Authenticator, error) {
		ctx = actor.MarkContextAsInternalActor(ctx)

		pubKey, err := safe.StateGet[*authres.PublicKey](ctx, st, authres.NewPublicKey(fingerprint).Metadata())
		if err != nil {
			return nil, err
		}

		if pubKey.TypedSpec().Value.Expiration.AsTime().Before(time.Now()) {
			return nil, errors.New("public key expired")
		}

		if !pubKey.TypedSpec().Value.Confirmed { //nolint:staticcheck
			return nil, errors.New("public key not confirmed")
		}

		userID, labelExists := pubKey.Metadata().Labels().Get(authres.LabelPublicKeyUserID)
		if !labelExists {
			return nil, errors.New("public key has no user ID label")
		}

		verifier, err := authres.GetSignatureVerifier(pubKey)
		if err != nil {
			return nil, err
		}

		user, err := safe.StateGet[*authres.User](ctx, st, resource.NewMetadata(resources.DefaultNamespace, authres.UserType, userID, resource.VersionUndefined))
		if err != nil {
			return nil, err
		}

		finalRole, err := role.Parse(user.TypedSpec().Value.GetRole())
		if err != nil {
			return nil, err
		}

		// a suspended instance is read only, which only lowers a role. The roles below Reader are
		// outside that ordering and are left alone.
		if suspended && finalRole.Check(role.Reader) == nil {
			finalRole = role.Reader
		}

		return &auth.Authenticator{
			UserID:   userID,
			Identity: pubKey.TypedSpec().Value.GetIdentity().GetEmail(),
			Role:     finalRole,
			Verifier: verifier,
		}, nil
	}
}
