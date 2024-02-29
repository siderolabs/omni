// Copyright (c) 2024 Sidero Labs, Inc.
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

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/siderolabs/go-api-signature/pkg/jwt"

	"github.com/siderolabs/omni/internal/pkg/config"
)

const (
	tokenValidationCacheDuration    = 5 * time.Minute
	tokenValidationAllowedClockSkew = 5 * time.Minute
)

// IDTokenVerifier is an Auth0 ID token verifier.
type IDTokenVerifier struct {
	validator *validator.Validator
}

// NewIDTokenVerifier creates a new ID token verifier.
func NewIDTokenVerifier(domain string) (*IDTokenVerifier, error) {
	issuerURL, err := url.Parse("https://" + domain + "/")
	if err != nil {
		return nil, err
	}

	provider := jwks.NewCachingProvider(issuerURL, tokenValidationCacheDuration)

	idTokenValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{config.Config.Auth.Auth0.ClientID},
		validator.WithAllowedClockSkew(tokenValidationAllowedClockSkew),
		validator.WithCustomClaims(func() validator.CustomClaims {
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
}

// Validate validates the claims on the CustomIDClaims.
func (a *CustomIDClaims) Validate(_ context.Context) error {
	_, err := mail.ParseAddress(a.Email)
	if err != nil {
		return fmt.Errorf("email claim is not valid: %w: %s", err, a.Email)
	}

	if !a.EmailVerified {
		return errors.New("email is not verified")
	}

	return nil
}
