// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package client provides an OIDC client.
package client

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
)

const loginPath = "/omni/oidc-login/%s"

// Client represents the default client for kubectl.
type Client struct{}

// GetID must return the client_id.
func (Client) GetID() string {
	return external.DefaultClientID
}

// RedirectURIs must return the registered redirect_uris for Code and Implicit Flow.
func (Client) RedirectURIs() []string {
	// these are the URLs used by the kubectl plugin
	return []string{
		"http://localhost:8000",
		"http://localhost:18000",
		external.KeyCodeRedirectURL,
	}
}

// PostLogoutRedirectURIs must return the registered post_logout_redirect_uris for sign-outs.
func (Client) PostLogoutRedirectURIs() []string {
	return []string{}
}

// ApplicationType must return the type of the client (app, native, user agent).
func (Client) ApplicationType() op.ApplicationType {
	return op.ApplicationTypeNative
}

// AuthMethod must return the authentication method (client_secret_basic, client_secret_post, none, private_key_jwt).
func (Client) AuthMethod() oidc.AuthMethod {
	return oidc.AuthMethodNone
}

// ResponseTypes must return all allowed response types (code, id_token token, id_token)
// these must match with the allowed grant types.
func (Client) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}

// GrantTypes must return all allowed grant types (authorization_code, refresh_token, urn:ietf:params:oauth:grant-type:jwt-bearer).
func (Client) GrantTypes() []oidc.GrantType {
	return []oidc.GrantType{oidc.GrantTypeCode}
}

// LoginURL will be called to redirect the user (agent) to the login UI
// you could implement some logic here to redirect the users to different login UIs depending on the client.
func (Client) LoginURL(id string) string {
	return fmt.Sprintf(loginPath, url.PathEscape(id))
}

// AccessTokenType must return the type of access token the client uses (Bearer (opaque) or JWT).
func (Client) AccessTokenType() op.AccessTokenType {
	return 0
}

// IDTokenLifetime must return the lifetime of the client's id_tokens.
func (Client) IDTokenLifetime() time.Duration {
	return external.OIDCTokenLifetime
}

// DevMode enables the use of non-compliant configs such as redirect_uris (e.g. http schema for user agent client).
func (Client) DevMode() bool {
	return constants.IsDebugBuild
}

// RestrictAdditionalIdTokenScopes allows specifying which custom scopes shall be asserted into the id_token.
func (Client) RestrictAdditionalIdTokenScopes() func(scopes []string) []string { //nolint:revive
	return func(scopes []string) []string {
		return scopes
	}
}

// RestrictAdditionalAccessTokenScopes allows specifying which custom scopes shall be asserted into the JWT access_token.
func (Client) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string {
		return scopes
	}
}

// IsScopeAllowed enables Client specific custom scopes validation
// in this example we allow the CustomScope for all clients.
func (Client) IsScopeAllowed(scope string) bool {
	return strings.HasPrefix(scope, external.ScopeClusterPrefix)
}

// IDTokenUserinfoClaimsAssertion allows specifying if claims of scope profile, email, phone and address are asserted into the id_token
// even if an access token if issued which violates the OIDC Core spec.
//
// (5.4. Requesting Claims using Scope Values: https://openid.net/specs/openid-connect-core-1_0.html#ScopeClaims)
// some clients though require that e.g. email is always in the id_token when requested even if an access_token is issued.
func (Client) IDTokenUserinfoClaimsAssertion() bool {
	return false
}

// ClockSkew enables clients to instruct the OP to apply a clock skew on the various times and expirations
// (subtract from issued_at, add to expiration, ...).
func (Client) ClockSkew() time.Duration {
	return 5 * time.Minute
}
