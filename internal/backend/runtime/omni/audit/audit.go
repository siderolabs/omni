// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package audit provides a state wrapper that logs audit events.
package audit

import (
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

const (
	// Auth0 is auth0 confirmation type.
	Auth0 = "auth0"
	// SAML is SAML confirmation type.
	SAML = "saml"
)

// Data contains the audit data.
type Data struct {
	UserAgent           string    `json:"user_agent,omitempty"`
	IPAddress           string    `json:"ip_address,omitempty"`
	UserID              string    `json:"user_id,omitempty"`
	Role                role.Role `json:"role,omitempty"`
	Email               string    `json:"email,omitempty"`
	ConfirmationType    string    `json:"confirmation_type,omitempty"`
	Fingerprint         string    `json:"fingerprint,omitempty"`
	PublicKeyExpiration int64     `json:"public_key_expiration,omitempty"`
}
