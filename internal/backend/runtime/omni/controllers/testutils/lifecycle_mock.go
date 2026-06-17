// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testutils

import (
	"context"
	"sync"

	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

// LifecycleManagerMock is a configurable test double for the maintenance lifecycle manager used by
// ClusterMachineConfigStatusController. It implements the controller's LifecycleManager interface.
type LifecycleManagerMock struct {
	// CheckAliveFunc, if set, runs on each CheckAlive call. Nil reports the machine alive.
	CheckAliveFunc func(ctx context.Context, address string) error
	// RunFunc, if set, runs on each Run call. Nil reports the operation successful.
	RunFunc func(ctx context.Context, op lifecycle.Operation, opts ...lifecycle.Option) error

	checkAliveCalls []string
	runOps          []lifecycle.Operation

	mu sync.Mutex
}

// NewLifecycleManagerMock returns a no-op LifecycleManagerMock.
func NewLifecycleManagerMock() *LifecycleManagerMock {
	return &LifecycleManagerMock{}
}

// CheckAlive records the call and delegates to CheckAliveFunc, reporting the machine alive if unset.
func (m *LifecycleManagerMock) CheckAlive(ctx context.Context, address string) error {
	m.mu.Lock()
	m.checkAliveCalls = append(m.checkAliveCalls, address)
	fn := m.CheckAliveFunc
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, address)
	}

	return nil
}

// Run records the operation and delegates to RunFunc, reporting success if unset.
func (m *LifecycleManagerMock) Run(ctx context.Context, op lifecycle.Operation, opts ...lifecycle.Option) error {
	m.mu.Lock()
	m.runOps = append(m.runOps, op)
	fn := m.RunFunc
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, op, opts...)
	}

	return nil
}

// CheckAliveCalls returns the machine addresses CheckAlive was called with, in order.
func (m *LifecycleManagerMock) CheckAliveCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.checkAliveCalls...)
}

// RunOperations returns the operations Run was called with, in order.
func (m *LifecycleManagerMock) RunOperations() []lifecycle.Operation {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]lifecycle.Operation(nil), m.runOps...)
}
