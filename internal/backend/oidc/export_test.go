// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc

import "golang.org/x/oauth2"

// NewTestHandler builds a Handler pointed at the given authorization endpoint, skipping the provider
// discovery that NewOIDCHandler needs a live identity provider for.
func NewTestHandler(authURL string) *Handler {
	return &Handler{
		key:      "test-key",
		endpoint: "https://omni.example.com",
		oauth2Config: oauth2.Config{
			ClientID:    "omni",
			RedirectURL: "https://omni.example.com" + RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL: authURL,
			},
		},
	}
}
