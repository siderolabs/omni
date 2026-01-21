// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/k8sproxy"
)

func TestJWTValidation(t *testing.T) {
	for _, tt := range []struct { //nolint:govet
		name                  string
		claims                k8sproxy.Claims
		expectedErrorContains string
	}{
		{
			name: "missing-subject",
			claims: k8sproxy.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "issuer",
					Subject:   "",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
				Cluster: "cluster-a",
				Groups:  []string{"group-a"},
			},
			expectedErrorContains: "subject is empty",
		},
		{
			name: "missing-cluster",
			claims: k8sproxy.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "issuer",
					Subject:   "subject-a",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
				Cluster: "",
				Groups:  []string{"group-a"},
			},
			expectedErrorContains: "cluster is empty",
		},
		{
			name: "missing-expiration",
			claims: k8sproxy.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "issuer",
					Subject:   "subject-a",
					ExpiresAt: nil,
				},
				Cluster: "cluster-a",
				Groups:  []string{"group-a"},
			},
			expectedErrorContains: "expiration is empty",
		},
		{
			name: "no-groups",
			claims: k8sproxy.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "issuer",
					Subject:   "subject-a",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
				Cluster: "cluster-a",
				Groups:  []string{},
			},
			expectedErrorContains: "groups is empty",
		},
		{
			name: "expired",
			claims: k8sproxy.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "issuer",
					Subject:   "subject-a",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				},
				Cluster: "cluster-a",
				Groups:  []string{"group-a"},
			},
			expectedErrorContains: "token is expired by",
		},
		{
			name: "not-yet-valid",
			claims: k8sproxy.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "issuer",
					Subject:   "subject-a",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					NotBefore: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
				Cluster: "cluster-a",
				Groups:  []string{"group-a"},
			},
			expectedErrorContains: "token is not valid yet",
		},
		{
			name: "valid",
			claims: k8sproxy.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "issuer",
					Subject:   "subject-a",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				},
				Cluster: "cluster-a",
				Groups:  []string{"group-a"},
			},
			expectedErrorContains: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.Valid()

			if tt.expectedErrorContains != "" {
				var validationError *jwt.ValidationError

				require.ErrorAs(t, err, &validationError)
				assert.ErrorContains(t, validationError, tt.expectedErrorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
