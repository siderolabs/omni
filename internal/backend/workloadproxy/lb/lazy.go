// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"
)

// ErrShutdown is returned when the load balancer is already shut down and cannot be used anymore.
var ErrShutdown = fmt.Errorf("load balancer is shut down")

// Lazy wraps the regular LB and adds lazy initialization and automatic shutdown
// after a specified duration of inactivity functionalities.
type Lazy struct {
	lastPickAttempt   time.Time
	log               *zap.Logger
	lb                *LB
	upstreamAddresses []string
	options           []upstream.ListOption
	stopAfter         time.Duration
	mu                sync.Mutex
	isShutdown        bool
}

// NewLazy creates a new Lazy load balancer.
func NewLazy(upstreamAddresses []string, stopAfter time.Duration, logger *zap.Logger, options ...upstream.ListOption) (*Lazy, error) {
	if stopAfter <= 0 {
		return nil, fmt.Errorf("invalid stopAfter value: %s", stopAfter)
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Lazy{
		upstreamAddresses: upstreamAddresses,
		options:           options,
		stopAfter:         stopAfter,
		log:               logger,
	}, nil
}

// Reconcile updates the upstream addresses and notifies the load balancer to reconcile them.
func (l *Lazy) Reconcile(upstreamAddresses []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger().Debug("reconcile")

	if l.isShutdown {
		return ErrShutdown
	}

	l.upstreamAddresses = upstreamAddresses

	if l.lb != nil {
		if err := l.lb.Reconcile(upstreamAddresses); err != nil {
			return err
		}
	}

	return nil
}

// PickAddress picks an upstream address from the load balancer, initializing it if necessary.
func (l *Lazy) PickAddress(ctx context.Context) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger().Debug("pick address")

	l.lastPickAttempt = time.Now()

	if l.isShutdown {
		return "", ErrShutdown
	}

	l.stopIfNeeded()

	if len(l.upstreamAddresses) == 0 {
		return "", upstream.ErrNoUpstreams
	}

	if err := l.init(); err != nil {
		return "", err
	}

	return l.lb.PickAddress(ctx)
}

// Notify notifies the load balancer to refresh its internal state.
func (l *Lazy) Notify() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger().Debug("notify")

	if l.isShutdown {
		return ErrShutdown
	}

	l.stopIfNeeded()

	return nil
}

func (l *Lazy) stopIfNeeded() {
	if len(l.upstreamAddresses) == 0 {
		l.logger().Debug("no upstream addresses, stop")

		l.stop()

		return
	}

	if !l.lastPickAttempt.IsZero() && time.Since(l.lastPickAttempt) > l.stopAfter {
		l.logger().Debug("inactive, stop")

		l.stop()
	}
}

// Shutdown stops the load balancer and cleans up resources.
func (l *Lazy) Shutdown() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger().Debug("shutdown")

	if l.isShutdown {
		return
	}

	l.stop()

	l.isShutdown = true
}

func (l *Lazy) init() error {
	l.logger().Debug("init")

	if l.lb != nil {
		return nil
	}

	var err error

	l.logger().Debug("create new LB")

	l.lb, err = New(l.upstreamAddresses, l.log, l.options...)
	if err != nil {
		return err
	}

	return nil
}

func (l *Lazy) stop() {
	l.logger().Debug("stop")

	if l.lb == nil {
		return
	}

	l.lb.Shutdown()

	l.lb = nil
}

func (l *Lazy) logger() *zap.Logger {
	return l.log.With(
		zap.Strings("upstream_addresses", l.upstreamAddresses),
		zap.Time("last_pick", l.lastPickAttempt),
		zap.Bool("is_running", l.lb != nil),
		zap.Duration("stop_after", l.stopAfter),
		zap.Bool("is_shutdown", l.isShutdown),
	)
}
