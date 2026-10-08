// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc

import (
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// NewTestHandler builds a Handler pointed at the given authorization and token endpoints, skipping the provider
// discovery that NewOIDCHandler needs a live identity provider for.
func NewTestHandler(authURL, tokenURL, logoutURL string, requireReauthForNewKeys bool, logger *zap.Logger) *Handler {
	return &Handler{
		logger:                  logger,
		key:                     "test-key",
		endpoint:                "https://omni.example.com",
		logoutURL:               logoutURL,
		requireReauthForNewKeys: requireReauthForNewKeys,
		oauth2Config: oauth2.Config{
			ClientID:    "omni",
			RedirectURL: "https://omni.example.com" + RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
		},
	}
}
