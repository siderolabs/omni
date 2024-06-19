// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package panichandler provides a panic handling errgroup.
package panichandler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const goroutinePanicked = "goroutine panicked"

// ErrPanic is the error returned when a task panics.
var ErrPanic = errors.New(goroutinePanicked)

// ErrGroup wraps golang.org/x/sync/errgroup.Group to handle panics by turning them into errors.
// It MUST be created using NewErrGroup.
type ErrGroup struct {
	eg *errgroup.Group
}

// NewErrGroup creates a new ErrGroup.
func NewErrGroup() *ErrGroup {
	return &ErrGroup{eg: &errgroup.Group{}}
}

// ErrGroupWithContext creates a new ErrGroup with the given context.
func ErrGroupWithContext(ctx context.Context) (*ErrGroup, context.Context) {
	eg, ctx := errgroup.WithContext(ctx)

	return &ErrGroup{eg: eg}, ctx
}

// Go runs the given function in a goroutine, handling panics by turning them into errors.
func (eg *ErrGroup) Go(f func() error) {
	eg.eg.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()

				err = errors.Join(err, fmt.Errorf("%w: %s\n%s", ErrPanic, r, string(stack)))
			}
		}()

		return f()
	})
}

// SetLimit sets the maximum number of goroutines that can run concurrently.
func (eg *ErrGroup) SetLimit(n int) {
	eg.eg.SetLimit(n)
}

// Wait waits for all goroutines to finish and returns the first error that occurred.
func (eg *ErrGroup) Wait() error {
	return eg.eg.Wait()
}

// Go runs the given function in a goroutine, handling panics by logging them.
//
// This function is a panic-handling wrapper for the "go" keyword.
func Go(f func(), logger *zap.Logger) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()

				if logger != nil {
					logger.DPanic(goroutinePanicked, zap.Any("panic", r), zap.String("stack", string(stack)))
				} else {
					log.Printf("[fallback logger] %s: %s\n%s", goroutinePanicked, r, string(stack))
				}
			}
		}()

		f()
	}()
}
