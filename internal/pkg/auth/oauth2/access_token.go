// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oauth2

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ClientCredentials describes an OAuth2 client credentials application: the authorization server
// that issues the tokens, the API the tokens are meant for, and the application's own credentials.
type ClientCredentials struct {
	// Domain is the authorization server's domain, e.g. "mycompany.example.com". A scheme must not
	// be included.
	Domain string
	// Audience is the identifier of the API the token is requested for, e.g.
	// "https://image-factory.example.com". It is not part of RFC 6749: Auth0 and the servers that
	// follow it reject a client credentials grant without one.
	Audience string
	// ClientID is the application's client ID.
	ClientID string
	// ClientSecret is the application's client secret.
	ClientSecret string
}

// Validate reports whether every field needed for a client credentials grant is set.
func (c ClientCredentials) Validate() error {
	var errs []error

	if c.Domain == "" {
		errs = append(errs, errors.New("domain is not set"))
	}

	if c.Audience == "" {
		errs = append(errs, errors.New("audience is not set"))
	}

	if c.ClientID == "" {
		errs = append(errs, errors.New("client ID is not set"))
	}

	if c.ClientSecret == "" {
		errs = append(errs, errors.New("client secret is not set"))
	}

	return errors.Join(errs...)
}

// AccessToken is an access token issued by the authorization server.
type AccessToken struct {
	// Token is the token itself, to be sent as a bearer token.
	Token string
	// IssuedAt is when the token was requested. The authorization server reports a token's lifetime
	// as a duration relative to the response, so the issue time has to be recorded on our side to
	// recover it.
	IssuedAt time.Time
	// ExpiresAt is when the token stops being accepted.
	ExpiresAt time.Time
	// TokenType is the type the authorization server returned, normally "Bearer".
	TokenType string
}

// Lifetime returns the validity period of the token.
func (t AccessToken) Lifetime() time.Duration {
	return t.ExpiresAt.Sub(t.IssuedAt)
}

// TokenIssuer issues access tokens.
//
// It is an interface so that callers which only need a token — the token rotation controller, most
// of all — can be tested without reaching an authorization server.
type TokenIssuer interface {
	IssueToken(ctx context.Context) (AccessToken, error)
}

// AccessTokenIssuer issues access tokens through the OAuth2 client credentials grant.
//
// It deliberately does not cache: each call performs a token request. Callers that need a token to
// outlive a single process are expected to persist it themselves.
type AccessTokenIssuer struct {
	config clientcredentials.Config
	now    func() time.Time
}

// NewAccessTokenIssuer creates a token issuer for the given OAuth2 client credentials application.
func NewAccessTokenIssuer(creds ClientCredentials) (*AccessTokenIssuer, error) {
	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid OAuth2 client credentials: %w", err)
	}

	tokenURL, err := url.Parse("https://" + creds.Domain + "/oauth/token")
	if err != nil {
		return nil, fmt.Errorf("invalid authorization server domain %q: %w", creds.Domain, err)
	}

	return &AccessTokenIssuer{
		config: clientcredentials.Config{
			ClientID:       creds.ClientID,
			ClientSecret:   creds.ClientSecret,
			TokenURL:       tokenURL.String(),
			EndpointParams: url.Values{"audience": {creds.Audience}},
			AuthStyle:      oauth2.AuthStyleInParams,
		},
		now: time.Now,
	}, nil
}

// IssueToken requests a new access token from the authorization server.
func (i *AccessTokenIssuer) IssueToken(ctx context.Context) (AccessToken, error) {
	issuedAt := i.now()

	token, err := i.config.Token(ctx)
	if err != nil {
		return AccessToken{}, fmt.Errorf("failed to request an access token: %w", err)
	}

	if token.AccessToken == "" {
		return AccessToken{}, errors.New("the authorization server returned an empty access token")
	}

	if token.Expiry.IsZero() {
		return AccessToken{}, errors.New("the authorization server returned a token without an expiration")
	}

	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	return AccessToken{
		Token:     token.AccessToken,
		TokenType: tokenType,
		IssuedAt:  issuedAt,
		ExpiresAt: token.Expiry,
	}, nil
}
