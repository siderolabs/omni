// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package storage implements OIDC storage for requests, tokens, etc.
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/client"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage/authrequest"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage/keys"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage/token"
)

// Storage implements the op.Storage interface.
//
// We implement this on top of mostly ephemeral in-memory storage.
type Storage struct {
	authRequestStorage *authrequest.Storage
	tokenStorage       *token.Storage
	keyStorage         *keys.Storage
}

// Interface check.
var _ op.Storage = &Storage{}

// NewStorage creates a new instance of OIDC storage.
func NewStorage(st state.State, logger *zap.Logger) *Storage {
	logger = logger.With(logging.Component("oidc_storage"))

	clk := clock.New()

	return &Storage{
		authRequestStorage: authrequest.NewStorage(),
		tokenStorage:       token.NewStorage(st, clk),
		keyStorage:         keys.NewStorage(st, clk, logger),
	}
}

// Health implements the op.Storage interface.
func (s *Storage) Health(context.Context) error {
	return nil
}

// KeySet implements the op.Storage interface.
//
// It will be called to get the current (public) keys, among others for the keys_endpoint or for validating access_tokens on the userinfo_endpoint, ...
func (s *Storage) KeySet(context.Context) ([]op.Key, error) {
	return s.keyStorage.KeySet()
}

// GetPublicKeyByID looks up the public key with the given ID.
func (s *Storage) GetPublicKeyByID(keyID string) (any, error) {
	return s.keyStorage.GetPublicKeyByID(keyID)
}

// GetClientByClientID implements the op.Storage interface.
//
// In our case we have a single fixed client.
func (s *Storage) GetClientByClientID(_ context.Context, clientID string) (op.Client, error) {
	if clientID != external.DefaultClientID {
		return nil, errors.New("client not found")
	}

	return client.Client{}, nil
}

// AuthorizeClientIDSecret implements the op.Storage interface.
//
// We don't use a client_secret, so stub implementation.
func (s *Storage) AuthorizeClientIDSecret(context.Context, string, string) error {
	return nil
}

// CreateAuthRequest implements the op.Storage interface
// it will be called after parsing and validation of the authentication request.
func (s *Storage) CreateAuthRequest(ctx context.Context, authReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	return s.authRequestStorage.CreateAuthRequest(ctx, authReq, userID)
}

// AuthRequestByID implements the op.Storage interface
// it will be called after the Login UI redirects back to the OIDC endpoint.
func (s *Storage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	return s.authRequestStorage.AuthRequestByID(ctx, id)
}

// AuthRequestByCode implements the op.Storage interface
// it will be called after parsing and validation of the token request (in an authorization code flow).
func (s *Storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	return s.authRequestStorage.AuthRequestByCode(ctx, code)
}

// SaveAuthCode implements the op.Storage interface
// it will be called after the authentication has been successful and before redirecting the user agent to the redirect_uri
// (in an authorization code flow).
func (s *Storage) SaveAuthCode(ctx context.Context, id string, code string) error {
	return s.authRequestStorage.SaveAuthCode(ctx, id, code)
}

// DeleteAuthRequest implements the op.Storage interface.
//
// It will be called after creating the token response (id and access tokens) for a valid
//   - authentication request (in an implicit flow)
//   - token request (in an authorization code flow).
func (s *Storage) DeleteAuthRequest(ctx context.Context, id string) error {
	return s.authRequestStorage.DeleteAuthRequest(ctx, id)
}

// GetPrivateClaimsFromScopes implements the op.Storage interface.
//
// It will be called for the creation of a JWT access token to assert claims for custom scopes.
func (s *Storage) GetPrivateClaimsFromScopes(ctx context.Context, userID, clientID string, scopes []string) (claims map[string]any, err error) {
	return s.tokenStorage.GetPrivateClaimsFromScopes(ctx, userID, clientID, scopes)
}

// ValidateJWTProfileScopes implements the op.Storage interface.
//
// It will be called to validate the scopes of a JWT Profile Authorization Grant request.
func (s *Storage) ValidateJWTProfileScopes(ctx context.Context, userID string, scopes []string) ([]string, error) {
	return s.tokenStorage.ValidateJWTProfileScopes(ctx, userID, scopes)
}

// SetUserinfoFromScopes implements the op.Storage interface.
//
// It will be called for the creation of an id_token, so we'll just pass it to the private function without any further check.
func (s *Storage) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, clientID string, scopes []string) error {
	return s.tokenStorage.SetUserinfoFromScopes(ctx, userinfo, userID, clientID, scopes)
}

// SetUserinfoFromToken implements the op.Storage interface.
//
// It will be called for the userinfo endpoint, so we read the token and pass the information from that to the private function.
func (s *Storage) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, origin string) error {
	return s.tokenStorage.SetUserinfoFromToken(ctx, userinfo, tokenID, subject, origin)
}

// SetIntrospectionFromToken implements the op.Storage interface.
//
// It will be called for the introspection endpoint, so we read the token and pass the information from that to the private function.
func (s *Storage) SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	return s.tokenStorage.SetIntrospectionFromToken(ctx, introspection, tokenID, subject, clientID)
}

// GetKeyByIDAndClientID implements the op.Storage interface.
//
// It will be called to validate the signatures of a JWT (JWT Profile Grant and Authentication).
func (s *Storage) GetKeyByIDAndClientID(ctx context.Context, keyID, clientID string) (*jose.JSONWebKey, error) {
	return s.tokenStorage.GetKeyByIDAndUserID(ctx, keyID, clientID)
}

// CreateAccessToken implements the op.Storage interface.
//
// It will be called for all requests able to return an access token (Authorization Code Flow, Implicit Flow, JWT Profile, ...).
func (s *Storage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	return s.tokenStorage.CreateAccessToken(ctx, request)
}

// CreateAccessAndRefreshTokens implements the op.Storage interface.
//
// It will be called for all requests able to return an access and refresh token (Authorization Code Flow, Refresh Token Request).
func (s *Storage) CreateAccessAndRefreshTokens(ctx context.Context, req op.TokenRequest, id string) (string, string, time.Time, error) {
	return s.tokenStorage.CreateAccessAndRefreshTokens(ctx, req, id)
}

// TokenRequestByRefreshToken implements the op.Storage interface.
//
// It will be called after parsing and validation of the refresh token request.
func (s *Storage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	return s.tokenStorage.TokenRequestByRefreshToken(ctx, refreshToken)
}

// TerminateSession implements the op.Storage interface.
//
// It will be called after the user signed out, therefore the access and refresh token of the user of this client must be removed.
func (s *Storage) TerminateSession(ctx context.Context, userID string, clientID string) error {
	return s.tokenStorage.TerminateSession(ctx, userID, clientID)
}

// RevokeToken implements the op.Storage interface.
//
// It will be called after parsing and validation of the token revocation request.
func (s *Storage) RevokeToken(ctx context.Context, token string, userID string, clientID string) *oidc.Error {
	return s.tokenStorage.RevokeToken(ctx, token, userID, clientID)
}

// SigningKey returns the active and currently used signing key.
func (s *Storage) SigningKey(context.Context) (op.SigningKey, error) {
	return s.keyStorage.GetCurrentSigningKey()
}

// GetRefreshTokenInfo implements the op.Storage interface.
func (s *Storage) GetRefreshTokenInfo(ctx context.Context, clientID string, token string) (userID string, tokenID string, err error) {
	return s.tokenStorage.GetRefreshTokenInfo(ctx, clientID, token)
}

// SignatureAlgorithms implements the op.Storage interface.
func (s *Storage) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}

// AuthenticateRequest implements the `authenticate` interface of the login.
func (s *Storage) AuthenticateRequest(requestID, identity string) error {
	return s.authRequestStorage.AuthenticateRequest(requestID, identity)
}

// Run runs the key refresher in a loop.
func (s *Storage) Run(ctx context.Context) error {
	return s.keyStorage.RunRefreshKey(ctx)
}
