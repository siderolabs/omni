// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package loadbalancer provides wrappers to run controlplane Kubernetes API loadbalancers.
package loadbalancer

import (
	"context"
	"errors"
	"net"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/siderolabs/go-loadbalancer/loadbalancer"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// LoadBalancer is the interface needed by the Manager.
type LoadBalancer interface {
	Start(upstreamCh <-chan []string) error
	Shutdown() error
	Healthy() (bool, error)
	DialContext(ctx context.Context) (net.Conn, error)
}

// NewFunc is a function type whose implementation should create a new load balancer.
type NewFunc func(bindAddress string, bindPort int, logger *zap.Logger, lbConfig config.LoadBalancerService) (LoadBalancer, error)

var ErrNotRunning = errors.New("load balancer is not running")

// DefaultNew returns a new load balancer with default settings.
func DefaultNew(bindAddress string, bindPort int, logger *zap.Logger, lbConfig config.LoadBalancerService) (LoadBalancer, error) { //nolint:ireturn
	endpoint := net.JoinHostPort(bindAddress, strconv.Itoa(bindPort))

	lb := &controlPlaneLB{
		endpoint:    endpoint,
		done:        make(chan struct{}),
		dialTimeout: lbConfig.GetDialTimeout(),
	}

	lb.tcp.DialTimeout = lbConfig.GetDialTimeout()
	lb.tcp.KeepAlivePeriod = lbConfig.GetKeepAlivePeriod()
	lb.tcp.TCPUserTimeout = lbConfig.GetTcpUserTimeout()
	lb.tcp.Logger = logging.IncreaseLevel(logger, zap.ErrorLevel).Named("controlplane-lb").With(zap.String("endpoint", endpoint)) // silence the load balancer logs

	lb.tcp.ListenFunc = func(network, addr string) (net.Listener, error) {
		inner, err := net.Listen(network, addr)
		if err != nil {
			return nil, err
		}

		l := newListener(inner)
		lb.listener.Store(l)

		return l, nil
	}

	healthCheckOptions := []upstream.ListOption{
		upstream.WithHealthcheckInterval(lbConfig.GetHealthCheckInterval()),
		upstream.WithHealthcheckTimeout(lbConfig.GetHealthCheckTimeout()),
		upstream.WithLowHighScores(-1, 3),
		upstream.WithUnhealthyHealthcheckInterval(time.Second),
	}

	if err := lb.tcp.AddRoute(endpoint, nil, healthCheckOptions...); err != nil {
		return nil, err
	}

	return lb, nil
}

type controlPlaneLB struct {
	tcp         loadbalancer.TCP
	listener    atomic.Pointer[listener]
	done        chan struct{}
	endpoint    string
	dialTimeout time.Duration
}

func (lb *controlPlaneLB) Start(upstreamCh <-chan []string) error {
	if err := lb.tcp.Start(); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case upstreams := <-upstreamCh:
				if err := lb.tcp.ReconcileRoute(lb.endpoint, slices.Values(upstreams)); err != nil {
					lb.tcp.Logger.Warn("failed reconciling list of upstreams",
						zap.Strings("upstreams", upstreams),
						zap.Error(err),
					)
				}
			case <-lb.done:
				return
			}
		}
	}()

	return nil
}

func (lb *controlPlaneLB) Healthy() (bool, error) {
	return lb.tcp.IsRouteHealthy(lb.endpoint)
}

func (lb *controlPlaneLB) DialContext(ctx context.Context) (net.Conn, error) {
	l := lb.listener.Load()
	if l == nil {
		return nil, ErrNotRunning
	}

	if _, ok := ctx.Deadline(); !ok && lb.dialTimeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, lb.dialTimeout)
		defer cancel()
	}

	return l.dial(ctx)
}

func (lb *controlPlaneLB) Shutdown() error {
	if err := lb.tcp.Close(); err != nil {
		return err
	}

	close(lb.done)

	lb.tcp.Wait() //nolint:errcheck

	return nil
}
