// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package models

import (
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/zitadel/oidc/pkg/oidc"
)

// AuthRequest is a request for authentication.
type AuthRequest struct { //nolint:govet
	ID            string
	CreationDate  time.Time
	ApplicationID string
	CallbackURI   string
	TransferState string
	Prompt        []string
	LoginHint     string
	MaxAuthAge    *time.Duration
	UserID        string
	Scopes        []string
	ResponseType  oidc.ResponseType
	Nonce         string
	CodeChallenge *OIDCCodeChallenge

	authenticated bool
	authTime      time.Time
}

// GetID implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetID() string {
	return a.ID
}

// GetACR implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetACR() string {
	return ""
}

// GetAMR implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetAMR() []string {
	if a.authenticated {
		return []string{"pop"}
	}

	return nil
}

// GetAudience implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetAudience() []string {
	return []string{a.ApplicationID}
}

// GetAuthTime implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetAuthTime() time.Time {
	return a.authTime
}

// GetClientID implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetClientID() string {
	return a.ApplicationID
}

// GetCodeChallenge implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetCodeChallenge() *oidc.CodeChallenge {
	return codeChallengeToOIDC(a.CodeChallenge)
}

// GetNonce implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetNonce() string {
	return a.Nonce
}

// GetRedirectURI implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetRedirectURI() string {
	return a.CallbackURI
}

// GetResponseType implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetResponseType() oidc.ResponseType {
	return a.ResponseType
}

// GetResponseMode implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetResponseMode() oidc.ResponseMode {
	return ""
}

// GetScopes implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetScopes() []string {
	return a.Scopes
}

// GetState implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetState() string {
	return a.TransferState
}

// GetSubject implements the oidc.AuthRequest interface.
func (a *AuthRequest) GetSubject() string {
	return a.UserID
}

// Done implements the oidc.AuthRequest interface.
func (a *AuthRequest) Done() bool {
	return a.authenticated
}

// MarkAsAuthenticated marks the request as authenticated.
func (a *AuthRequest) MarkAsAuthenticated(authTime time.Time) {
	a.authenticated = true
	a.authTime = authTime
}

func promptToInternal(oidcPrompt oidc.SpaceDelimitedArray) []string {
	prompts := make([]string, 0, len(oidcPrompt))

	for _, oidcPrompt := range oidcPrompt {
		switch oidcPrompt {
		case oidc.PromptNone,
			oidc.PromptLogin,
			oidc.PromptConsent,
			oidc.PromptSelectAccount:
			prompts = append(prompts, oidcPrompt)
		}
	}

	return prompts
}

func maxAgeToInternal(maxAge *uint) *time.Duration {
	if maxAge == nil {
		return nil
	}

	return pointer.To(time.Duration(*maxAge) * time.Second)
}

// AuthRequestToInternal converts an oidc.AuthRequest to an internal AuthRequest.
func AuthRequestToInternal(authReq *oidc.AuthRequest, userID string) *AuthRequest {
	return &AuthRequest{
		CreationDate:  time.Now(),
		ApplicationID: authReq.ClientID,
		CallbackURI:   authReq.RedirectURI,
		TransferState: authReq.State,
		Prompt:        promptToInternal(authReq.Prompt),
		LoginHint:     authReq.LoginHint,
		MaxAuthAge:    maxAgeToInternal(authReq.MaxAge),
		UserID:        userID,
		Scopes:        authReq.Scopes,
		ResponseType:  authReq.ResponseType,
		Nonce:         authReq.Nonce,
		CodeChallenge: &OIDCCodeChallenge{
			Challenge: authReq.CodeChallenge,
			Method:    string(authReq.CodeChallengeMethod),
		},
	}
}
