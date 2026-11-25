// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package token

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/models"
	"github.com/siderolabs/omni/internal/pkg/authz/grants"
)

// Lifetime of the token.
const Lifetime = 5 * time.Minute

// Storage implements storing and handling OIDC access tokens, handling roles and claims.
//
//nolint:govet
type Storage struct {
	state state.State

	mu     sync.Mutex
	tokens map[string]*models.Token
}

// NewStorage creates a new token storage.
func NewStorage(st state.State) *Storage {
	return &Storage{
		tokens: map[string]*models.Token{},
		state:  st,
	}
}

// CreateAccessToken implements the op.Storage interface.
//
// It will be called for all requests able to return an access token (Authorization Code Flow, Implicit Flow, JWT Profile, ...).
func (s *Storage) CreateAccessToken(_ context.Context, request op.TokenRequest) (string, time.Time, error) {
	var applicationID string

	// if authenticated for an app (auth code / implicit flow) we must save the client_id to the token
	authReq, ok := request.(*models.AuthRequest)
	if ok {
		applicationID = authReq.ApplicationID
	}

	token := s.accessToken(applicationID, "", request.GetSubject(), request.GetAudience(), request.GetScopes())

	return token.ID, token.Expiration, nil
}

// CreateAccessAndRefreshTokens implements the op.Storage interface.
//
// It will be called for all requests able to return an access and refresh token (Authorization Code Flow, Refresh Token Request).
func (s *Storage) CreateAccessAndRefreshTokens(context.Context, op.TokenRequest, string) (string, string, time.Time, error) {
	return "", "", time.Time{}, errors.New("not implemented")
}

// TokenRequestByRefreshToken implements the op.Storage interface.
//
// It will be called after parsing and validation of the refresh token request.
func (s *Storage) TokenRequestByRefreshToken(context.Context, string) (op.RefreshTokenRequest, error) {
	return nil, errors.New("not implemented")
}

// TerminateSession implements the op.Storage interface.
//
// It will be called after the user signed out, therefore the access and refresh token of the user of this client must be removed.
func (s *Storage) TerminateSession(_ context.Context, userID string, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, token := range s.tokens {
		if token.ApplicationID == clientID && token.Subject == userID {
			delete(s.tokens, token.ID)

			return nil
		}
	}

	return nil
}

// RevokeToken implements the op.Storage interface.
//
// It will be called after parsing and validation of the token revocation request.
func (s *Storage) RevokeToken(_ context.Context, token string, _ string, clientID string) *oidc.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	accessToken, ok := s.tokens[token]
	if ok {
		if accessToken.ApplicationID != clientID {
			return oidc.ErrInvalidClient().WithDescription("token was not issued for this client")
		}

		// if it is an access token, just remove it
		// you could also remove the corresponding refresh token if really necessary
		delete(s.tokens, accessToken.ID)

		return nil
	}

	return nil
}

// accessToken will store an access_token in-memory based on the provided information.
func (s *Storage) accessToken(applicationID, refreshTokenID, subject string, audience, scopes []string) *models.Token {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := &models.Token{
		ID:             uuid.NewString(),
		ApplicationID:  applicationID,
		RefreshTokenID: refreshTokenID,
		Subject:        subject,
		Audience:       audience,
		Expiration:     time.Now().Add(Lifetime),
		Scopes:         scopes,
	}

	s.tokens[token.ID] = token

	return token
}

// SetUserinfoFromScopes implements the op.Storage interface.
//
// It will be called for the creation of an id_token, so we'll just pass it to the private function without any further check.
func (s *Storage) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, _ string, scopes []string) error {
	return s.setUserInfo(ctx, userinfo, userID, scopes)
}

// SetUserinfoFromToken implements the op.Storage interface.
//
// It will be called for the userinfo endpoint, so we read the token and pass the information from that to the private function.
func (s *Storage) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, _, _ string) error {
	token, ok := func() (*models.Token, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()

		token, ok := s.tokens[tokenID]

		if ok && time.Now().After(token.Expiration) {
			return nil, false
		}

		return token, ok
	}()
	if !ok {
		return errors.New("token is invalid or has expired")
	}

	return s.setUserInfo(ctx, userinfo, token.Subject, token.Scopes)
}

// SetIntrospectionFromToken implements the op.Storage interface.
//
// It will be called for the introspection endpoint, so we read the token and pass the information from that to the private function.
func (s *Storage) SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	token, ok := func() (*models.Token, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()

		token, ok := s.tokens[tokenID]

		return token, ok
	}()
	if !ok {
		return errors.New("token is invalid or has expired")
	}

	// check if the client is part of the requested audience
	for _, aud := range token.Audience {
		if aud == clientID {
			// the introspection response only has to return a boolean (active) if the token is active
			// this will automatically be done by the library if you don't return an error
			// you can also return further information about the user / associated token
			// e.g. the userinfo (equivalent to userinfo endpoint)
			err := s.setResponse(ctx, introspection, subject, token.Scopes)
			if err != nil {
				return err
			}

			// ...and also the requested scopes...
			introspection.Scope = slices.Clone(token.Scopes)

			// ...and the client the token was issued to
			introspection.ClientID = token.ApplicationID

			return nil
		}
	}

	return errors.New("token is not valid for this client")
}

// GetKeyByIDAndUserID implements the op.Storage interface.
//
// It will be called to validate the signatures of a JWT (JWT Profile Grant and Authentication).
func (s *Storage) GetKeyByIDAndUserID(context.Context, string, string) (*jose.JSONWebKey, error) {
	return nil, errors.New("not implemented")
}

// setResponse sets the info based on the user, scopes and if necessary the clientID.
func (s *Storage) setResponse(ctx context.Context, ir *oidc.IntrospectionResponse, userID string, scopes []string) error {
	for _, currentScope := range scopes {
		switch {
		case currentScope == oidc.ScopeOpenID:
			ir.Subject = userID
		case strings.HasPrefix(currentScope, external.ScopeClusterPrefix):
			cluster := currentScope[len(external.ScopeClusterPrefix):]

			impersonateGroups, err := grants.KubernetesGroups(ctx, s.state, userID, cluster)
			if err != nil {
				return err
			}

			if ir.Claims == nil {
				ir.Claims = map[string]any{}
			}

			ir.Claims["cluster"] = cluster
			ir.Claims["groups"] = impersonateGroups
		}
	}

	return nil
}

// setUserInfo sets the info based on the user, scopes and if necessary the clientID.
func (s *Storage) setUserInfo(ctx context.Context, userInfo *oidc.UserInfo, userID string, scopes []string) error {
	for _, currentScope := range scopes {
		switch {
		case currentScope == oidc.ScopeOpenID:
			userInfo.Subject = userID
		case strings.HasPrefix(currentScope, external.ScopeClusterPrefix):
			cluster := currentScope[len(external.ScopeClusterPrefix):]

			impersonateGroups, err := grants.KubernetesGroups(ctx, s.state, userID, cluster)
			if err != nil {
				return err
			}

			userInfo.AppendClaims("cluster", cluster)
			userInfo.AppendClaims("groups", impersonateGroups)
		}
	}

	return nil
}

// GetPrivateClaimsFromScopes implements the op.Storage interface.
//
// It will be called for the creation of a JWT access token to assert claims for custom scopes.
func (s *Storage) GetPrivateClaimsFromScopes(ctx context.Context, userID, _ string, scopes []string) (map[string]any, error) {
	claims := map[string]any{}

	for _, scope := range scopes {
		if strings.HasPrefix(scope, external.ScopeClusterPrefix) {
			cluster := scope[len(external.ScopeClusterPrefix):]

			impersonateGroups, err := grants.KubernetesGroups(ctx, s.state, userID, cluster)
			if err != nil {
				return nil, fmt.Errorf("failed to get impersonate groups: %w", err)
			}

			claims["cluster"] = cluster
			claims["groups"] = impersonateGroups
		}
	}

	return claims, nil
}

// ValidateJWTProfileScopes implements the op.Storage interface.
//
// It will be called to validate the scopes of a JWT Profile Authorization Grant request.
func (s *Storage) ValidateJWTProfileScopes(_ context.Context, _ string, scopes []string) ([]string, error) {
	var allowedScopes []string

	for _, scope := range scopes {
		switch {
		case scope == oidc.ScopeOpenID:
			allowedScopes = append(allowedScopes, scope)
		case strings.HasPrefix(scope, external.ScopeClusterPrefix):
			allowedScopes = append(allowedScopes, scope)
		}
	}

	return allowedScopes, nil
}

// GetRefreshTokenInfo implements the op.Storage interface.
func (s *Storage) GetRefreshTokenInfo(context.Context, string, string) (userID string, tokenID string, err error) {
	return "", "", errors.New("not implemented")
}
