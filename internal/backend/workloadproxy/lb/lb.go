// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package lb contains the logic of running health checks against a set of upstreams and picking a healthy one when requested.
// It takes this logic and the primitives from https://github.com/siderolabs/go-loadbalancer as-is.
package lb

import (
	"context"
	"net"
	"slices"
	"syscall"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"
)

// LB is a minimal wrapper around [upstream.List] which provides a simpler interface for picking an upstream address.
type LB struct {
	logger    *zap.Logger
	upstreams *upstream.List[node]
}

// New creates a new load balancer with the given upstream addresses and logger.
func New(upstreamAddresses []string, logger *zap.Logger, options ...upstream.ListOption) (*LB, error) {
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
	}, nil
}

// Reconcile updates the list of upstream addresses in the load balancer.
func (lb *LB) Reconcile(upstreamAddresses []string) {
	nodes := slices.Values(xslices.Map(upstreamAddresses, func(addr string) node {
		return node{address: addr, logger: lb.logger}
	}))

	lb.upstreams.Reconcile(nodes)
}

// PickAddress picks a healthy upstream address from the load balancer.
func (lb *LB) PickAddress() (string, error) {
	pickedNode, err := lb.upstreams.Pick()
	if err != nil {
		return "", err
	}

	return pickedNode.address, nil
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

	for i := len(mins) - 1; i >= 0; i-- {
		if elapsed >= mins[i] {
			return upstream.Tier(i), nil
		}
	}

	// We should never get here, but there is no way to tell this to Go compiler.
	return upstream.Tier(len(mins)), err
}

func probeDialer() *net.Dialer {
	return &net.Dialer{
		// The dialer reduces the TIME-WAIT period to 1 seconds instead of the OS default of 60 seconds.
		Control: func(_, _ string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptLinger(int(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{Onoff: 1, Linger: 1}) //nolint: errcheck
			})
		},
	}
}
