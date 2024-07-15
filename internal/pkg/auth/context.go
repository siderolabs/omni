// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"github.com/siderolabs/go-api-signature/pkg/message"

	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// EnabledAuthContextKey is the context key for enabled authentication.
type EnabledAuthContextKey struct{ Enabled bool }

// GRPCMessageContextKey is the context key for the GRPC message. It is only set if authentication is enabled.
type GRPCMessageContextKey struct{ Message *message.GRPC }

// VerifiedEmailContextKey is the context key for the verified email address.
type VerifiedEmailContextKey struct{ Email string }

// UserIDContextKey is the context key for the user ID. Value has the type string.
type UserIDContextKey struct{ UserID string }

// RoleContextKey is the context key for the role. Value has the type role.Role.
type RoleContextKey struct{ Role role.Role }

// IdentityContextKey is the context key for the user identity.
type IdentityContextKey struct{ Identity string }
