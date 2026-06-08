// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package lb contains the logic of running health checks against a set of upstreams and picking a healthy one when requested.
// It takes this logic and the primitives from https://github.com/siderolabs/go-loadbalancer as-is.
package lb

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"
)

// LB is a minimal wrapper around [upstream.List] which provides a simpler interface for picking an upstream address.
//
// addresses and fallbackCounter are only accessed from Reconcile and PickAddress, both of which the workload
// proxy reconciler always calls while holding its own mutex, so they need no synchronization of their own.
type LB struct {
	logger          *zap.Logger
	upstreams       *upstream.List[node]
	addresses       []string
	fallbackCounter uint
}

// New creates a new load balancer with the given upstream addresses and logger.
func New(upstreamAddresses []string, logger *zap.Logger, options ...upstream.ListOption) (*LB, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	nodes := slices.Values(xslices.Map(upstreamAddresses, func(addr string) node {
		return node{address: addr, logger: logger}
	}))
	nodesEqual := func(a, b node) bool { return a.address == b.address }

	upstreams, err := upstream.NewListWithCmp(nodes, nodesEqual, options...)
	if err != nil {
		return nil, err
	}

	return &LB{
		logger:    logger,
		upstreams: upstreams,
		addresses: slices.Clone(upstreamAddresses),
	}, nil
}

// Reconcile updates the list of upstream addresses in the load balancer.
func (lb *LB) Reconcile(upstreamAddresses []string) error {
	nodes := slices.Values(xslices.Map(upstreamAddresses, func(addr string) node {
		return node{address: addr, logger: lb.logger}
	}))

	lb.upstreams.Reconcile(nodes)
	lb.addresses = slices.Clone(upstreamAddresses)

	return nil
}

// PickAddress picks a healthy upstream address, falling back to any known upstream when all are benched.
func (lb *LB) PickAddress() (string, error) {
	pickedNode, err := lb.upstreams.Pick()
	if err == nil {
		return pickedNode.address, nil
	}

	// Optimistic fallback: when every upstream is benched, Pick reports no upstreams available.
	// A cold load balancer (freshly created after an Omni restart, all scores starting at zero)
	// gets benched by a single failed health check, e.g. one that coincided with a brief upstream
	// blip, and would otherwise keep failing until the next health check restores a score, up to
	// one health check interval later. Hand out a known upstream anyway, round-robin: if it
	// actually serves, the caller recovers immediately, and the periodic health check brings the
	// score back up on its own.
	if errors.Is(err, upstream.ErrNoUpstreams) && len(lb.addresses) > 0 {
		addr := lb.addresses[lb.fallbackCounter%uint(len(lb.addresses))]
		lb.fallbackCounter++

		return addr, nil
	}

	return "", err
}

// Notify notifies the load balancer for it to refresh its internal state.
func (lb *LB) Notify() error {
	return nil
}

// Shutdown shuts down the load balancer and its upstream health checks.
func (lb *LB) Shutdown() {
	lb.upstreams.Shutdown()
}

// node is an implementation of an [upstream.Backend]. Taken as-is from the upstream package.
type node struct {
	logger  *zap.Logger
	address string // host:port
}

// HealthCheck implements the [upstream.Backend] interface for node.
//
// It is taken as-is from the upstream package.
func (upstream node) HealthCheck(ctx context.Context) (upstream.Tier, error) {
	start := time.Now()
	err := upstream.healthCheck(ctx)
	elapsed := time.Since(start)

	return calcTier(err, elapsed)
}

func (upstream node) healthCheck(ctx context.Context) error {
	d := probeDialer()

	c, err := d.DialContext(ctx, "tcp", upstream.address)
	if err != nil {
		upstream.logger.Warn("healthcheck failed", zap.String("address", upstream.address), zap.Error(err))

		return err
	}

	return c.Close()
}

var mins = []time.Duration{
	0,
	time.Millisecond / 10,
	time.Millisecond,
	10 * time.Millisecond,
	100 * time.Millisecond,
	1 * time.Second,
}

func calcTier(err error, elapsed time.Duration) (upstream.Tier, error) {
	if err != nil {
		// preserve old tier
		return -1, err
	}

	for i := range slices.Backward(mins) {
		if elapsed >= mins[i] {
			return upstream.Tier(i), nil
		}
	}

	// We should never get here, but there is no way to tell this to Go compiler.
	return upstream.Tier(len(mins)), err
}
