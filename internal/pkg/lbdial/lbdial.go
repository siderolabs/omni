// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package lbdial lets the Omni process reach its own Kubernetes load balancers without going through the network.
//
// The load balancers listen on the SideroLink address, which the Omni process cannot always reach: macOS does not
// route traffic to a local WireGuard address back to the host. Clients inside Omni dial through a Dialer instead,
// which hands them an in-memory connection to the load balancer.
package lbdial

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

var ErrNoRoute = errors.New("no in-process load balancer for the address")

type DialFunc func(ctx context.Context) (net.Conn, error)

type Dialer struct {
	routes map[string]DialFunc
	mu     sync.RWMutex
}

func New() *Dialer {
	return &Dialer{
		routes: map[string]DialFunc{},
	}
}

func (d *Dialer) Register(address string, dial DialFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.routes[address] = dial
}

func (d *Dialer) Unregister(address string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.routes, address)
}

func (d *Dialer) DialContext(ctx context.Context, _, address string) (net.Conn, error) {
	d.mu.RLock()
	dial, ok := d.routes[address]
	d.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoRoute, address)
	}

	return dial(ctx)
}
