// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth0_test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/auth0"
)

const (
	testIssuer   = "https://omni.eu.auth0.com/"
	testClientID = "client-id"
	testEmail    = "user@example.com"
)

func TestIDTokenVerifier(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	now := time.Now()

	for _, tt := range []struct {
		authTime                *time.Time
		issuedAt                *time.Time
		name                    string
		expectedError           string
		requireReauthForNewKeys bool
		unverifiedEmail         bool
	}{
		{
			name:                    "required, fresh login",
			requireReauthForNewKeys: true,
			authTime:                new(now.Add(-time.Minute)),
		},
		{
			name:            "not required, unverified email",
			unverifiedEmail: true,
			expectedError:   "email not verified",
		},
		{
			name:                    "required, auth_time missing",
			requireReauthForNewKeys: true,
			expectedError:           "auth_time claim is missing",
		},
		{
			name:                    "required, old login",
			requireReauthForNewKeys: true,
			authTime:                new(now.Add(-time.Hour)),
			expectedError:           "re-authentication required",
		},
		{
			name: "not required, auth_time missing",
		},
		{
			name:     "not required, old login",
			authTime: new(now.Add(-time.Hour)),
		},
		{
			name:          "not required, token issued long ago",
			issuedAt:      new(now.Add(-time.Hour)),
			expectedError: "token is too old",
		},
		{
			name:          "not required, token issued in the future",
			issuedAt:      new(now.Add(time.Hour)),
			expectedError: `"iat" not satisfied`,
		},
		{
			name:     "not required, auth_time in the future",
			authTime: new(now.Add(time.Hour)),
		},
		{
			name:                    "required, auth_time in the future",
			requireReauthForNewKeys: true,
			authTime:                new(now.Add(time.Hour)),
			expectedError:           "login time is in the future",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			verifier, err := auth0.NewTestIDTokenVerifier(testIssuer, testClientID, &key.PublicKey, tt.requireReauthForNewKeys)
			require.NoError(t, err)

			issuedAt := now
			if tt.issuedAt != nil {
				issuedAt = *tt.issuedAt
			}

			claims := jwt.MapClaims{
				"iss":            testIssuer,
				"aud":            testClientID,
				"sub":            "google-oauth2|1",
				"iat":            issuedAt.Unix(),
				"exp":            now.Add(time.Hour).Unix(),
				"email":          testEmail,
				"email_verified": !tt.unverifiedEmail,
			}

			if tt.authTime != nil {
				claims["auth_time"] = tt.authTime.Unix()
			}

			token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
			require.NoError(t, err)

			verified, err := verifier.Verify(t.Context(), token)
			if tt.unverifiedEmail {
				require.ErrorAs(t, err, new(*auth.EmailNotVerifiedError))
			}

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)

				return
			}

			require.NoError(t, err)
			require.Equal(t, testEmail, verified.VerifiedEmail)
		})
	}
}
