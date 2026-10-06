// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package actor

import (
	"context"

	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/internal/pkg/auth"
)

// Type categorizes a request's caller to be used when per-actor logic is needed.
type Type string

// Type values.
const (
	TypeInternal       Type = "internal"
	TypeInfraProvider  Type = "infraprovider"
	TypeServiceAccount Type = "serviceaccount"
	TypeUser           Type = "user"
	TypeUnknown        Type = "unknown"
)

// Classification describes a request's caller.
type Classification struct {
	Type Type

	baseName string // base name for service-accounts
}

// BaseName returns the base name for service-accounts.
func (c Classification) BaseName() string { return c.baseName }

// Classify inspects the context and returns a Classification of the caller.
//
// Identity is checked first because most user-initiated state writes pass through management
// handlers that wrap the user's context with MarkContextAsInternalActor — that mark is additive
// and does not clear the identity set by the auth interceptors. We attribute the call to the
// originating user/service account when an identity is present, falling back to TypeInternal
// only for genuinely Omni-initiated calls (controllers, server boot, background jobs).
func Classify(ctx context.Context) Classification {
	identity := auth.IdentityFromContext(ctx)

	if identity == "" {
		if ContextIsInternalActor(ctx) {
			return Classification{Type: TypeInternal}
		}

		return Classification{Type: TypeUnknown}
	}

	sa, isSA := access.ParseServiceAccountFromFullID(identity)
	switch {
	case isSA && sa.IsInfraProvider:
		return Classification{Type: TypeInfraProvider, baseName: sa.BaseName}
	case isSA:
		return Classification{Type: TypeServiceAccount, baseName: sa.BaseName}
	default:
		return Classification{Type: TypeUser}
	}
}
