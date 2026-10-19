// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"maps"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
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
		claims          jwt.MapClaims
		expectedLabels  map[string]string
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
			name:     "groups and roles",
			issuedAt: now,
			// values of other types give no labels
			claims: jwt.MapClaims{"groups": []any{"devs", "Omni Admins", 42}, "roles": map[string]any{"reader": true}},
			expectedLabels: map[string]string{
				authres.OIDCLabelPrefix + "groups/devs":        "",
				authres.OIDCLabelPrefix + "groups/Omni Admins": "",
			},
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

			claims := jwt.MapClaims{
				"iss":            testIssuer,
				"aud":            testClientID,
				"sub":            "1",
				"iat":            tt.issuedAt.Unix(),
				"exp":            now.Add(time.Hour).Unix(),
				"email":          testEmail,
				"email_verified": !tt.unverifiedEmail,
			}

			maps.Copy(claims, tt.claims)

			token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
			require.NoError(t, err)

			email, labels, err := verifier.VerifyWithLabels(t.Context(), token)
			if tt.unverifiedEmail {
				require.ErrorAs(t, err, new(*auth.EmailNotVerifiedError))
			}

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)

				return
			}

			require.NoError(t, err)
			require.Equal(t, testEmail, email)

			if tt.expectedLabels != nil {
				require.Equal(t, tt.expectedLabels, labels)
			}
		})
	}
}
