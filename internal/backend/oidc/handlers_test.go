// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/oidc"
)

const testAuthURL = "https://idp.example.com/authorize"

// TestLoginForcesReauthentication pins the behavior that makes logging out of Omni mean something. The
// provider session survives an Omni logout when it publishes no end-session endpoint, so without
// prompt=login the next authorization request is answered with the user who just logged out.
func TestLoginForcesReauthentication(t *testing.T) {
	for _, target := range []string{
		"/login?flow=frontend",
		"/login?flow=cli&public-key-id=key-id",
		"/login",
	} {
		t.Run(target, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
			rec := httptest.NewRecorder()

			oidc.NewTestHandler(testAuthURL).Login(rec, req)

			resp := rec.Result()
			defer resp.Body.Close() //nolint:errcheck

			require.Equal(t, http.StatusFound, resp.StatusCode)

			redirectURL, err := url.Parse(resp.Header.Get("Location"))
			require.NoError(t, err)

			assert.Equal(t, "idp.example.com", redirectURL.Host)
			assert.Equal(t, "login", redirectURL.Query().Get("prompt"))
		})
	}
}
