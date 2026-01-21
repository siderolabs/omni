// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authrequest

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/siderolabs/omni/internal/backend/oidc/internal/models"
)

// Storage implements authentication request storage.
type Storage struct {
	authRequests map[string]*models.AuthRequest
	codes        map[string]string
	lock         sync.Mutex
}

// NewStorage creates a new auth request storage.
func NewStorage() *Storage {
	return &Storage{
		authRequests: map[string]*models.AuthRequest{},
		codes:        map[string]string{},
	}
}

// CreateAuthRequest implements the op.Storage interface
// it will be called after parsing and validation of the authentication request.
func (s *Storage) CreateAuthRequest(_ context.Context, authReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	request := models.AuthRequestToInternal(authReq, userID)
	request.ID = uuid.NewString()

	s.authRequests[request.ID] = request

	return request, nil
}

// AuthRequestByID implements the op.Storage interface
// it will be called after the Login UI redirects back to the OIDC endpoint.
func (s *Storage) AuthRequestByID(_ context.Context, id string) (op.AuthRequest, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	request, ok := s.authRequests[id]
	if !ok {
		return nil, errors.New("request not found")
	}

	return request, nil
}

// AuthRequestByCode implements the op.Storage interface
// it will be called after parsing and validation of the token request (in an authorization code flow).
func (s *Storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	// for this example we read the id by code and then get the request by id
	requestID, ok := func() (string, bool) {
		s.lock.Lock()
		defer s.lock.Unlock()

		requestID, ok := s.codes[code]

		return requestID, ok
	}()
	if !ok {
		return nil, errors.New("code invalid or expired")
	}

	return s.AuthRequestByID(ctx, requestID)
}

// SaveAuthCode implements the op.Storage interface
// it will be called after the authentication has been successful and before redirecting the user agent to the redirect_uri
// (in an authorization code flow).
func (s *Storage) SaveAuthCode(_ context.Context, id string, code string) error {
	// for this example we'll just save the authRequestID to the code
	s.lock.Lock()
	defer s.lock.Unlock()

	s.codes[code] = id

	return nil
}

// DeleteAuthRequest implements the op.Storage interface.
//
// It will be called after creating the token response (id and access tokens) for a valid
//   - authentication request (in an implicit flow)
//   - token request (in an authorization code flow).
func (s *Storage) DeleteAuthRequest(_ context.Context, id string) error {
	// you can simply delete all reference to the auth request
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.authRequests, id)

	for code, requestID := range s.codes {
		if id == requestID {
			delete(s.codes, code)

			return nil
		}
	}

	return nil
}

// AuthenticateRequest implements the `authenticate` interface of the login.
func (s *Storage) AuthenticateRequest(requestID, identity string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	request, ok := s.authRequests[requestID]
	if !ok {
		return errors.New("request not found")
	}

	request.UserID = identity
	request.MarkAsAuthenticated(time.Now())

	return nil
}
