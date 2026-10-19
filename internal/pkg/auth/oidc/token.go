// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/siderolabs/go-api-signature/pkg/jwt"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
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
	email, _, err := v.VerifyWithLabels(ctx, token)
	if err != nil {
		return nil, err
	}

	return &jwt.Claims{
		VerifiedEmail: email,
	}, nil
}

// VerifyWithLabels verifies the given ID token and returns the email and the identity labels built from the groups and roles claims.
func (v *IDTokenVerifier) VerifyWithLabels(ctx context.Context, token string) (string, map[string]string, error) {
	oidcToken, err := v.verifier.Verify(ctx, token)
	if err != nil {
		return "", nil, err
	}

	if err = auth.CheckIDTokenAge(oidcToken.IssuedAt); err != nil {
		return "", nil, err
	}

	var claims struct {
		Email         string   `json:"email"`
		Groups        []string `json:"groups"`
		Roles         []string `json:"roles"`
		EmailVerified bool     `json:"email_verified"`
	}

	// groups or roles of other types give no labels
	if err = oidcToken.Claims(&claims); err != nil && !errors.As(err, new(*json.UnmarshalTypeError)) {
		return "", nil, err
	}

	_, err = mail.ParseAddress(claims.Email)
	if err != nil {
		return "", nil, fmt.Errorf("email claim is not valid: %w: %s", err, claims.Email)
	}

	if !claims.EmailVerified && !v.allowUnverifiedEmail {
		return "", nil, &auth.EmailNotVerifiedError{Email: claims.Email}
	}

	labels := map[string]string{}

	for claim, values := range map[string][]string{"groups": claims.Groups, "roles": claims.Roles} {
		for _, value := range values {
			if value != "" {
				labels[authres.OIDCLabelPrefix+claim+"/"+value] = ""
			}
		}
	}

	return strings.ToLower(claims.Email), labels, nil
}
