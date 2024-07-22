// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package audit provides a state wrapper that logs audit events.
package audit

import (
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// Data contains the audit data.
type Data struct {
	UserAgent string    `json:"user_agent,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Identity  string    `json:"identity,omitempty"`
	Role      role.Role `json:"role,omitempty"`
	Email     string    `json:"email,omitempty"`
}
