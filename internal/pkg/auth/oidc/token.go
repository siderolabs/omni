// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/siderolabs/go-api-signature/pkg/jwt"
)

// IDTokenVerifier is an Auth0 ID token verifier.
type IDTokenVerifier struct {
	verifier             *oidc.IDTokenVerifier
	allowUnverifiedEmail bool
}

// NewIDTokenVerifier creates a new ID token verifier.
func NewIDTokenVerifier(ctx context.Context, provider *oidc.Provider, clientID string,
	allowUnverifiedEmail bool,
) (*IDTokenVerifier, error) {
	verifier := provider.Verifier(&oidc.Config{
		ClientID: clientID,
	})

	return &IDTokenVerifier{
		verifier:             verifier,
		allowUnverifiedEmail: allowUnverifiedEmail,
	}, nil
}

// Verify verifies the given Auth0 ID token with the configured Auth0 domain (JWKs).
func (v *IDTokenVerifier) Verify(ctx context.Context, token string) (*jwt.Claims, error) {
	oidcToken, err := v.verifier.Verify(ctx, token)
	if err != nil {
		return nil, err
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}

	if err = oidcToken.Claims(&claims); err != nil {
		return nil, err
	}

	_, err = mail.ParseAddress(claims.Email)
	if err != nil {
		return nil, fmt.Errorf("email claim is not valid: %w: %s", err, claims.Email)
	}

	if !claims.EmailVerified && !v.allowUnverifiedEmail {
		return nil, &EmailNotVerifiedError{Email: claims.Email}
	}

	return &jwt.Claims{
		VerifiedEmail: strings.ToLower(claims.Email),
	}, nil
}

// EmailNotVerifiedError is an error that occurs when the email address is not verified.
type EmailNotVerifiedError struct {
	Email string
}

// Error implements the error interface.
func (e EmailNotVerifiedError) Error() string {
	return "email not verified: " + e.Email
}
