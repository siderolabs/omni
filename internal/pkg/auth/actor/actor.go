// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package actor implements the context marking for internal/external actors.
package actor

import (
	"context"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// internalActorContextKey is the key for internal actor context.
type internalActorContextKey struct{}

// tnfraProviderContextKey forces infra provider role and sets infrastructure provider name in the context.
type infraProviderContextKey struct {
	ProviderID string
}

// MarkContextAsInternalActor returns a new derived context from the given context, marked as an internal actor.
func MarkContextAsInternalActor(ctx context.Context) context.Context {
	return ctxstore.WithValue(ctx, internalActorContextKey{})
}

// MarkContextAsInfraProvider marks context as infra provider.
func MarkContextAsInfraProvider(ctx context.Context, name string) context.Context {
	fullID := name + "@infra-provider.serviceaccount.omni.sidero.dev"

	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: true})
	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: fullID})
	ctx = ctxstore.WithValue(ctx, auth.VerifiedEmailContextKey{Email: fullID})
	ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: role.InfraProvider})

	return ctx
}

// ContextIsInternalActor returns true if the given context is marked as an internal actor.
func ContextIsInternalActor(ctx context.Context) bool {
	_, ok := ctxstore.Value[internalActorContextKey](ctx)

	return ok
}

// ContextInfraProvider returns id of the infra provider if it's set in the context.
func ContextInfraProvider(ctx context.Context) string {
	value, ok := ctxstore.Value[infraProviderContextKey](ctx)

	if !ok {
		return ""
	}

	return value.ProviderID
}
