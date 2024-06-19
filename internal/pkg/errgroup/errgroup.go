// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package errgroup is a small wrapper around Go's x/sync/errgroup.Group.
package errgroup

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/siderolabs/omni/client/pkg/panichandler"
)

// EGroup defines common interface for Group and x/sync/errgroup.Group.
type EGroup interface {
	Go(func() error)
	Wait() error
}

// Group is wrapper around Go's x/sync/errgroup.Group. It's not a drop-in replacement for it, because
// it requires initialization with WithContext.
type Group struct {
	group EGroup
	ctx   context.Context //nolint:containedctx
}

// WithContext returns a new Group and an associated Context derived from ctx.
//
// The derived Context is canceled the first time a function passed to Go
// returns or the first time Wait returns, whichever occurs
// first.
func WithContext(ctx context.Context) (*Group, context.Context) {
	withContext, newCtx := panichandler.ErrGroupWithContext(ctx)

	return &Group{group: withContext, ctx: newCtx}, newCtx
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (g *Group) Wait() error {
	return g.group.Wait()
}

// Go is small wrapper around errgroup.Group.Go. When f returns nil error, and group ctx was not canceled
// it instead returns ReturnError thus canceling the group.
func (g *Group) Go(f func() error) {
	GoWithContext(g.ctx, g.group, f)
}

// ReturnError contains a stack trace of the function which called Group.Go.
type ReturnError struct{ stack string }

func (e *ReturnError) Error() string {
	return fmt.Sprintf("sentinel error: function returned with nil error: %s", e.stack)
}

// GoWithContext is a small wrapper around errgroup.Group.Go. When f returns nil error, and ctx was not canceled
// it instead returns ReturnError.
//
// It may be needed in situations where you use errgroup.Group without initializing it with WithContext.
func GoWithContext(ctx context.Context, eg EGroup, f func() error) {
	stack := debug.Stack()

	eg.Go(func() error {
		err := f()
		if err != nil {
			return err
		}

		if ctx.Err() == nil {
			// If the context is not canceled, that means the function didn't return because it was
			// canceled. So instead of nil we return a ReturnError.
			return &ReturnError{stack: string(stack)}
		}

		return nil
	})
}
