// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/oidc"
)

const (
	testIssuer   = "https://idp.example.com"
	testClientID = "client-id"
	testEmail    = "user@example.com"
)

func TestIDTokenVerifier(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	verifier := oidc.NewTestIDTokenVerifier(testIssuer, testClientID, &key.PublicKey)

	now := time.Now()

	for _, tt := range []struct {
		issuedAt        time.Time
		name            string
		expectedError   string
		unverifiedEmail bool
	}{
		{
			name:     "fresh token",
			issuedAt: now,
		},
		{
			name:     "backdated token",
			issuedAt: now.Add(-8 * time.Minute),
		},
		{
			name:          "token issued long ago",
			issuedAt:      now.Add(-time.Hour),
			expectedError: "token is too old",
		},
		{
			name:          "token issued in the future",
			issuedAt:      now.Add(time.Hour),
			expectedError: "token is issued in the future",
		},
		{
			name:            "unverified email",
			issuedAt:        now,
			unverifiedEmail: true,
			expectedError:   "email not verified",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
				"iss":            testIssuer,
				"aud":            testClientID,
				"sub":            "1",
				"iat":            tt.issuedAt.Unix(),
				"exp":            now.Add(time.Hour).Unix(),
				"email":          testEmail,
				"email_verified": !tt.unverifiedEmail,
			}).SignedString(key)
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
