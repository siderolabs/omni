// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package actor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	withIdentity := func(identity string) context.Context {
		return ctxstore.WithValue(context.Background(), auth.IdentityContextKey{Identity: identity})
	}

	withVerifiedEmail := func(email string) context.Context {
		return ctxstore.WithValue(context.Background(), auth.VerifiedEmailContextKey{Email: email})
	}

	for _, tc := range []struct {
		name         string
		ctx          context.Context //nolint:containedctx // fixture
		wantType     actor.Type
		wantBasename string
	}{
		{
			name:     "internal actor",
			ctx:      actor.MarkContextAsInternalActor(context.Background()),
			wantType: actor.TypeInternal,
		},
		{
			// Most user-initiated writes pass through management handlers that call
			// MarkContextAsInternalActor on the user's context. Identity must take precedence
			// so we attribute the call to the originating user, not to Internal.
			name:     "internal mark with user identity attributes to user",
			ctx:      actor.MarkContextAsInternalActor(withIdentity("alice@example.com")),
			wantType: actor.TypeUser,
		},
		{
			// Same flow for JWT/SAML-authenticated UI users — identity comes from VerifiedEmail.
			name:     "internal mark with verified email attributes to user",
			ctx:      actor.MarkContextAsInternalActor(withVerifiedEmail("alice@example.com")),
			wantType: actor.TypeUser,
		},
		{
			name:         "infra provider service account",
			ctx:          withIdentity("aws-1" + access.InfraProviderServiceAccountNameSuffix),
			wantType:     actor.TypeInfraProvider,
			wantBasename: "aws-1",
		},
		{
			name:         "regular service account",
			ctx:          withIdentity("ci-bot" + access.ServiceAccountNameSuffix),
			wantType:     actor.TypeServiceAccount,
			wantBasename: "ci-bot",
		},
		{
			name:     "user identity",
			ctx:      withIdentity("alice@example.com"),
			wantType: actor.TypeUser,
		},
		{
			// JWT (Auth0/OIDC) and SAML interceptors only set VerifiedEmailContextKey, not Identity.
			name:     "user via verified email",
			ctx:      withVerifiedEmail("alice@example.com"),
			wantType: actor.TypeUser,
		},
		{
			// IdentityContextKey takes precedence when both are set.
			name: "identity wins over verified email",
			ctx: ctxstore.WithValue(
				withVerifiedEmail("user@example.com"),
				auth.IdentityContextKey{Identity: "ci-bot" + access.ServiceAccountNameSuffix},
			),
			wantType:     actor.TypeServiceAccount,
			wantBasename: "ci-bot",
		},
		{
			name:     "no identity, no internal mark",
			ctx:      context.Background(),
			wantType: actor.TypeUnknown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := actor.Classify(tc.ctx)
			assert.Equal(t, tc.wantType, got.Type)
			assert.Equal(t, tc.wantBasename, got.BaseName())
		})
	}
}
