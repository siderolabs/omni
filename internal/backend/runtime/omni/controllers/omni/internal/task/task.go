// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package task implements generic controller tasks running in goroutines.
package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
)

// ID is a task ID.
type ID = string

// Spec configures a task.
type Spec[T any] interface {
	ID() ID
	RunTask(ctx context.Context, logger *zap.Logger, in T) error
}

// Task is a generic controller task that can run in a goroutine with restarts and panic handling.
type Task[T any, S Spec[T]] struct {
	spec S
	in   T

	logger *zap.Logger
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new task.
func New[T any, S Spec[T]](logger *zap.Logger, spec S, in T) *Task[T, S] {
	return &Task[T, S]{
		spec:   spec,
		in:     in,
		logger: logger.With(zap.String("task", spec.ID())),
	}
}

// Spec returns the task spec.
func (task *Task[T, S]) Spec() S {
	return task.spec
}

// Start the task in a separate goroutine.
func (task *Task[T, S]) Start(ctx context.Context) {
	task.wg.Add(1)

	ctx, task.cancel = context.WithCancel(ctx)

	go func() {
		defer task.wg.Done()

		task.runWithRestarts(ctx)
	}()
}

func (task *Task[T, S]) runWithRestarts(ctx context.Context) {
	backoff := backoff.NewExponentialBackOff()

	// disable number of retries limit
	backoff.MaxElapsedTime = 0

	for ctx.Err() == nil {
		err := task.runWithPanicHandler(ctx)

		// finished without an error
		if err == nil {
			task.logger.Info("task finished")

			return
		}

		interval := backoff.NextBackOff()

		task.logger.Error("restarting task", zap.Duration("interval", interval), zap.Error(err))

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (task *Task[T, S]) runWithPanicHandler(ctx context.Context) (err error) { //nolint:nonamedreturns
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)

			task.logger.Error("task panicked", zap.Stack("stack"), zap.Error(err))
		}
	}()

	return task.spec.RunTask(ctx, task.logger, task.in)
}

// Stop the task waiting for it to finish.
func (task *Task[T, S]) Stop() {
	task.cancel()

	task.wg.Wait()
}

// EqualSpec is like [Spec] but it requires an Equal method from the spec.
type EqualSpec[T any, S Spec[T]] interface {
	Spec[T]
	Equal(S) bool
}
