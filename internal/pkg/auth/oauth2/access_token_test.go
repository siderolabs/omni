// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oauth2_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goOauth2 "golang.org/x/oauth2"

	"github.com/siderolabs/omni/internal/pkg/auth/oauth2"
)

// issuerTransport routes the requests the issuer makes at the real authorization server URL to a
// local test server, recording the URL that was asked for so that the test can assert on it.
type issuerTransport struct { //nolint:govet // the recorded URL reads better last
	server *httptest.Server

	requestedURL string
}

func (t *issuerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requestedURL = req.URL.String()

	serverURL, err := url.Parse(t.server.URL)
	if err != nil {
		return nil, err
	}

	rerouted := req.Clone(req.Context())
	rerouted.URL.Scheme = serverURL.Scheme
	rerouted.URL.Host = serverURL.Host
	rerouted.Host = ""

	return http.DefaultTransport.RoundTrip(rerouted)
}

func TestAccessTokenIssuerIssueToken(t *testing.T) {
	t.Parallel()

	var form url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())

		form = r.PostForm

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"token-value","token_type":"Bearer","expires_in":86400}`)) //nolint:errcheck
	}))
	t.Cleanup(server.Close)

	transport := &issuerTransport{server: server}

	issuer, err := oauth2.NewAccessTokenIssuer(oauth2.ClientCredentials{
		Domain:       "tenant.example.com",
		Audience:     "https://image-factory.example.com",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), goOauth2.HTTPClient, &http.Client{Transport: transport})

	before := time.Now()

	token, err := issuer.IssueToken(ctx)
	require.NoError(t, err)

	assert.Equal(t, "https://tenant.example.com/oauth/token", transport.requestedURL)
	assert.Equal(t, "client_credentials", form.Get("grant_type"))
	assert.Equal(t, "https://image-factory.example.com", form.Get("audience"))
	assert.Equal(t, "client-id", form.Get("client_id"))
	assert.Equal(t, "client-secret", form.Get("client_secret"))

	assert.Equal(t, "token-value", token.Token)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.False(t, token.IssuedAt.Before(before))
	// expires_in is relative to the response, so the recovered lifetime is approximate.
	assert.InDelta(t, (24 * time.Hour).Seconds(), token.Lifetime().Seconds(), 60)
}

func TestAccessTokenIssuerErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct { //nolint:govet // test table, alignment is irrelevant
		name     string
		response string
		status   int
		expected string
	}{
		{
			name:     "empty access token",
			response: `{"access_token":"","token_type":"Bearer","expires_in":86400}`,
			status:   http.StatusOK,
			expected: "server response missing access_token",
		},
		{
			name:     "no expiration",
			response: `{"access_token":"token-value","token_type":"Bearer"}`,
			status:   http.StatusOK,
			expected: "the authorization server returned a token without an expiration",
		},
		{
			name:     "rejected grant",
			response: `{"error":"access_denied","error_description":"Service not enabled within domain"}`,
			status:   http.StatusForbidden,
			expected: "failed to request an access token",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				w.Write([]byte(test.response)) //nolint:errcheck
			}))
			t.Cleanup(server.Close)

			issuer, err := oauth2.NewAccessTokenIssuer(oauth2.ClientCredentials{
				Domain:       "tenant.example.com",
				Audience:     "https://image-factory.example.com",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			})
			require.NoError(t, err)

			ctx := context.WithValue(t.Context(), goOauth2.HTTPClient, &http.Client{Transport: &issuerTransport{server: server}})

			_, err = issuer.IssueToken(ctx)
			require.ErrorContains(t, err, test.expected)
		})
	}
}

func TestNewAccessTokenIssuerValidation(t *testing.T) {
	t.Parallel()

	complete := oauth2.ClientCredentials{
		Domain:       "tenant.example.com",
		Audience:     "https://image-factory.example.com",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}

	for _, test := range []struct {
		name     string
		modify   func(*oauth2.ClientCredentials)
		expected string
	}{
		{name: "no domain", modify: func(c *oauth2.ClientCredentials) { c.Domain = "" }, expected: "domain is not set"},
		{name: "no audience", modify: func(c *oauth2.ClientCredentials) { c.Audience = "" }, expected: "audience is not set"},
		{name: "no client ID", modify: func(c *oauth2.ClientCredentials) { c.ClientID = "" }, expected: "client ID is not set"},
		{name: "no client secret", modify: func(c *oauth2.ClientCredentials) { c.ClientSecret = "" }, expected: "client secret is not set"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			creds := complete
			test.modify(&creds)

			_, err := oauth2.NewAccessTokenIssuer(creds)
			require.ErrorContains(t, err, test.expected)
		})
	}
}
