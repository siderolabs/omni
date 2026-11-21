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

	"github.com/prometheus/client_golang/prometheus"
)

var (
	ErrNoRoute    = errors.New("no in-process load balancer for the address")
	ErrNotRunning = errors.New("load balancer is not running")
)

type DialFunc func(ctx context.Context) (net.Conn, error)

type Dialer struct {
	routes map[string]DialFunc

	metricOpenConnections prometheus.Gauge
	metricDialErrors      *prometheus.CounterVec
	metricAcceptRetries   prometheus.Counter

	mu sync.RWMutex
}

func New() *Dialer {
	return &Dialer{
		routes: map[string]DialFunc{},

		metricOpenConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_lb_inprocess_open_connections",
			Help: "Number of open in-process connections to the cluster load balancers.",
		}),
		metricDialErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_lb_inprocess_dial_errors_total",
			Help: "Number of failed in-process dials to the cluster load balancers, by reason.",
		}, []string{"reason"}),
		metricAcceptRetries: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_lb_accept_retries_total",
			Help: "Number of retried network accept errors of the cluster load balancers.",
		}),
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
		d.metricDialErrors.WithLabelValues(dialErrorReason(ErrNoRoute)).Inc()

		return nil, fmt.Errorf("%w: %s", ErrNoRoute, address)
	}

	conn, err := dial(ctx)
	if err != nil {
		d.metricDialErrors.WithLabelValues(dialErrorReason(err)).Inc()

		return nil, err
	}

	d.metricOpenConnections.Inc()

	return &countedConn{Conn: conn, release: d.metricOpenConnections.Dec}, nil
}

func (d *Dialer) RecordAcceptRetry() {
	d.metricAcceptRetries.Inc()
}

func (d *Dialer) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *Dialer) Collect(ch chan<- prometheus.Metric) {
	d.metricOpenConnections.Collect(ch)
	d.metricDialErrors.Collect(ch)
	d.metricAcceptRetries.Collect(ch)
}

var _ prometheus.Collector = &Dialer{}

func dialErrorReason(err error) string {
	switch {
	case errors.Is(err, ErrNoRoute):
		return "no_route"
	case errors.Is(err, ErrNotRunning):
		return "not_running"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "other"
	}
}

type countedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *countedConn) Close() error {
	c.once.Do(c.release)

	return c.Conn.Close()
}
