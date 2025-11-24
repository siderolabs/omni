// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlitelog

import (
	"slices"
	"sync"

	"github.com/siderolabs/gen/xslices"
)

// Manager defines a subscription manager.
type Manager struct {
	subscriptions []chan struct{}
	mu            sync.Mutex
}

type subscription struct {
	ch chan struct{}
	m  *Manager
}

// Subscription is an active subscription interface.
type Subscription interface {
	NotifyCh() <-chan struct{}
	TriggerNotify()
	Unsubscribe()
}

// NewManager creates a new subscription manager.
func NewManager() *Manager {
	return &Manager{}
}

// Subscribe creates a new subscription for the given resource kind.
func (m *Manager) Subscribe() Subscription {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan struct{}, 1)

	m.subscriptions = append(m.subscriptions, ch)

	return &subscription{
		ch: ch,
		m:  m,
	}
}

// Notify notifies all subscribers about an event for the given resource kind.
func (m *Manager) Notify() {
	m.mu.Lock()
	subs := slices.Clone(m.subscriptions)
	m.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Empty checks whether there are any subscriptions.
func (m *Manager) Empty() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.subscriptions) == 0
}

// NotifyCh implements Subscription interface.
func (s *subscription) NotifyCh() <-chan struct{} {
	return s.ch
}

// TriggerNotify implements Subscription interface.
func (s *subscription) TriggerNotify() {
	select {
	case s.ch <- struct{}{}:
	default:
	}
}

// Unsubscribe implements Subscription interface.
func (s *subscription) Unsubscribe() {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()

	s.m.subscriptions = xslices.FilterInPlace(s.m.subscriptions,
		func(ch chan struct{}) bool {
			return ch != s.ch
		},
	)
}
