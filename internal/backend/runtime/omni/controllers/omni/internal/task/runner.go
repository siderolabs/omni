// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package task

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// EqualityFunc is used to compare two task specs.
type EqualityFunc[T any] func(x, y T) bool

// Runner manages running tasks.
type Runner[T any, S Spec[T]] struct {
	running      map[ID]*Task[T, S]
	equalityFunc EqualityFunc[S]
	mu           sync.Mutex
}

// NewRunner creates a new task runner.
func NewRunner[T any, S Spec[T]](equalityFunc EqualityFunc[S]) *Runner[T, S] {
	if equalityFunc == nil {
		panic("equalityFunc must not be nil")
	}

	return &Runner[T, S]{
		running:      make(map[ID]*Task[T, S]),
		equalityFunc: equalityFunc,
	}
}

// NewEqualRunner creates a new task runner from spec with Equal method.
func NewEqualRunner[S EqualSpec[T, S], T any]() *Runner[T, S] {
	return NewRunner[T, S](func(x, y S) bool { return x.Equal(y) })
}

// Stop all running tasks.
func (runner *Runner[T, S]) Stop() {
	for _, task := range runner.running {
		task.Stop()
	}
}

// StartTask starts a new task.
func (runner *Runner[T, S]) StartTask(ctx context.Context, logger *zap.Logger, id string, spec S, task T) {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	running, ok := runner.running[id]

	if ok {
		if runner.equalityFunc(spec, running.spec) {
			return
		}

		logger.Debug("replacing task", zap.String("task", id))

		runner.stopTask(id)
	}

	runner.running[id] = New(logger, spec, task)

	logger.Debug("starting task", zap.String("task", id))
	runner.running[id].Start(ctx)
}

// StopTask stop the running task.
func (runner *Runner[T, S]) StopTask(logger *zap.Logger, id string) {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	logger.Debug("stopping task", zap.String("task", id))

	runner.stopTask(id)
}

func (runner *Runner[T, S]) stopTask(id string) {
	if _, ok := runner.running[id]; !ok {
		return
	}

	runner.running[id].Stop()
	delete(runner.running, id)
}

// Reconcile running tasks.
func (runner *Runner[T, S]) Reconcile(ctx context.Context, logger *zap.Logger, shouldRun map[ID]S, in T) {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	// stop running tasks which shouldn't run
	for id := range runner.running {
		if _, exists := shouldRun[id]; !exists {
			logger.Debug("stopping task", zap.String("task", id))

			runner.stopTask(id)
		} else if !runner.equalityFunc(shouldRun[id], runner.running[id].Spec()) {
			logger.Debug("replacing task", zap.String("task", id))

			runner.stopTask(id)
		}
	}

	// start tasks which aren't running
	for id := range shouldRun {
		if _, exists := runner.running[id]; !exists {
			runner.running[id] = New(logger, shouldRun[id], in)

			logger.Debug("starting task", zap.String("task", id))
			runner.running[id].Start(ctx)
		}
	}
}
