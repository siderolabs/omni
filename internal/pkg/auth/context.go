// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

// EnabledAuthContextKey is the context key for enabled authentication. Value has the type bool.
type EnabledAuthContextKey struct{}

// GRPCMessageContextKey is the context key for the GRPC message. It is only set if authentication is enabled.
type GRPCMessageContextKey struct{}

// VerifiedEmailContextKey is the context key for the verified email address. Value has the type string.
type VerifiedEmailContextKey struct{}

// UserIDContextKey is the context key for the user ID. Value has the type string.
type UserIDContextKey struct{}

// RoleContextKey is the context key for the role. Value has the type role.Role.
type RoleContextKey struct{}

// IdentityContextKey is the context key for the user identity. Value has the type string.
type IdentityContextKey struct{}
