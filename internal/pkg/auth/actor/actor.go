// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package actor implements the context marking for internal/external actors.
package actor

import (
	"context"

	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// internalActorContextKey is the key for internal actor context.
type internalActorContextKey struct{}

// MarkContextAsInternalActor returns a new derived context from the given context, marked as an internal actor.
func MarkContextAsInternalActor(ctx context.Context) context.Context {
	return ctxstore.WithValue(ctx, internalActorContextKey{})
}

// ContextIsInternalActor returns true if the given context is marked as an internal actor.
func ContextIsInternalActor(ctx context.Context) bool {
	_, ok := ctxstore.Value[internalActorContextKey](ctx)

	return ok
}
