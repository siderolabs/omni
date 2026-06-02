// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth0

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"time"

	"github.com/auth0/go-jwt-middleware/v3/jwks"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/siderolabs/go-api-signature/pkg/jwt"
)

const (
	tokenValidationCacheDuration    = 5 * time.Minute
	tokenValidationAllowedClockSkew = 5 * time.Minute
	tokenValidationMaxAge           = 2 * time.Minute
)

// IDTokenVerifier is an Auth0 ID token verifier.
type IDTokenVerifier struct {
	validator *validator.Validator
}

// NewIDTokenVerifier creates a new ID token verifier.
func NewIDTokenVerifier(domain, clientID string) (*IDTokenVerifier, error) {
	issuerURL, err := url.Parse("https://" + domain + "/")
	if err != nil {
		return nil, err
	}

	provider, err := jwks.NewCachingProvider(
		jwks.WithIssuerURL(issuerURL),
		jwks.WithCacheTTL(tokenValidationCacheDuration),
	)
	if err != nil {
		return nil, err
	}

	idTokenValidator, err := validator.New(
		validator.WithKeyFunc(provider.KeyFunc),
		validator.WithAlgorithm(validator.RS256),
		validator.WithIssuer(issuerURL.String()),
		validator.WithAudience(clientID),
		validator.WithAllowedClockSkew(tokenValidationAllowedClockSkew),
		validator.WithCustomClaims(func() *CustomIDClaims {
			return &CustomIDClaims{}
		}),
	)
	if err != nil {
		return nil, err
	}

	return &IDTokenVerifier{
		validator: idTokenValidator,
	}, nil
}

// Verify verifies the given Auth0 ID token with the configured Auth0 domain (JWKs).
func (v *IDTokenVerifier) Verify(ctx context.Context, token string) (*jwt.Claims, error) {
	claims, err := v.validator.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	validatedClaims, ok := claims.(*validator.ValidatedClaims)
	if !ok {
		return nil, errors.New("unexpected claims type")
	}

	customClaims, ok := validatedClaims.CustomClaims.(*CustomIDClaims)
	if !ok {
		return nil, errors.New("unexpected custom claims type")
	}

	if customClaims.AuthTime == 0 {
		return nil, errors.New("auth_time claim is missing")
	}

	authTime := time.Unix(customClaims.AuthTime, 0)

	if authTime.After(time.Now().Add(tokenValidationAllowedClockSkew)) {
		return nil, errors.New("auth_time is in the future")
	}

	if time.Since(authTime) > tokenValidationMaxAge+tokenValidationAllowedClockSkew {
		return nil, errors.New("re-authentication required")
	}

	return &jwt.Claims{
		VerifiedEmail: customClaims.Email,
	}, nil
}

// IDClaims represents the claims of an ID token.
type IDClaims struct {
	validator.RegisteredClaims
	CustomIDClaims
}

// CustomIDClaims is the custom claims we expect to be present in ID tokens.
type CustomIDClaims struct {
	// Email is the email address of the user.
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	// AuthTime is the time when the user last authenticated, as a Unix timestamp.
	// It is included when max_age or prompt=login is used in the auth request.
	AuthTime int64 `json:"auth_time"`
}

// Validate validates the claims on the CustomIDClaims.
func (a *CustomIDClaims) Validate(_ context.Context) error {
	_, err := mail.ParseAddress(a.Email)
	if err != nil {
		return fmt.Errorf("email claim is not valid: %w: %s", err, a.Email)
	}

	if !a.EmailVerified {
		return &EmailNotVerifiedError{Email: a.Email}
	}

	return nil
}

// EmailNotVerifiedError is an error that occurs when the email address is not verified.
type EmailNotVerifiedError struct {
	Email string
}

// Error implements the error interface.
func (e EmailNotVerifiedError) Error() string {
	return "email not verified: " + e.Email
}
