// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package loadbalancer provides wrappers to run controlplane Kubernetes API loadbalancers.
package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/siderolabs/go-loadbalancer/loadbalancer"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"github.com/siderolabs/go-retry/retry"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// LoadBalancer is the interface needed by the Manager.
type LoadBalancer interface {
	UpdateUpstreams(upstreams []string) error
	Shutdown() error
	Healthy() (bool, error)
}

// NewFunc is a function type whose implementation should create and start a new load balancer.
type NewFunc func(ctx context.Context, bindAddress string, bindPort int, logger *zap.Logger, lbConfig config.LoadBalancerService) (LoadBalancer, error)

// DefaultNew creates and starts a new load balancer with default settings.
//
// It listens on the bind address, which the nodes connect to, and on 127.0.0.1, which Omni itself connects to.
func DefaultNew(ctx context.Context, bindAddress string, bindPort int, logger *zap.Logger, lbConfig config.LoadBalancerService) (LoadBalancer, error) { //nolint:ireturn
	port := strconv.Itoa(bindPort)

	lb := &controlPlaneLB{
		endpoints: []string{
			net.JoinHostPort(bindAddress, port),
			net.JoinHostPort("127.0.0.1", port),
		},
	}

	lb.tcp.DialTimeout = lbConfig.GetDialTimeout()
	lb.tcp.KeepAlivePeriod = lbConfig.GetKeepAlivePeriod()
	lb.tcp.TCPUserTimeout = lbConfig.GetTcpUserTimeout()
	lb.tcp.Logger = logging.IncreaseLevel(logger, zap.ErrorLevel).Named("controlplane-lb").With(zap.Strings("endpoints", lb.endpoints)) // silence the load balancer logs

	// tcpproxy binds every route through ListenFunc in Start
	lb.tcp.ListenFunc = func(network, address string) (net.Listener, error) {
		return listen(ctx, network, address)
	}

	for _, endpoint := range lb.endpoints {
		if err := lb.tcp.AddRoute(endpoint, nil,
			upstream.WithHealthcheckInterval(lbConfig.GetHealthCheckInterval()),
			upstream.WithHealthcheckTimeout(lbConfig.GetHealthCheckTimeout()),
			upstream.WithLowHighScores(-1, 3),
			upstream.WithUnhealthyHealthcheckInterval(time.Second),
		); err != nil {
			lb.tcp.Close() //nolint:errcheck

			return nil, err
		}
	}

	if err := lb.tcp.Start(); err != nil {
		lb.tcp.Close() //nolint:errcheck

		return nil, err
	}

	return lb, nil
}

// listen retries while the address is not available, as the SideroLink address is only there once the WireGuard interface is up.
func listen(ctx context.Context, network, address string) (net.Listener, error) {
	var listener net.Listener

	if err := retry.Constant(20*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(ctx, func(ctx context.Context) error {
		var err error

		listener, err = (&net.ListenConfig{}).Listen(ctx, network, address)
		if errors.Is(err, syscall.EADDRNOTAVAIL) {
			return retry.ExpectedError(err)
		}

		return err
	}); err != nil {
		return nil, fmt.Errorf("error listening on %s: %w", address, err)
	}

	return listener, nil
}

type controlPlaneLB struct {
	endpoints []string
	tcp       loadbalancer.TCP
}

func (lb *controlPlaneLB) UpdateUpstreams(upstreams []string) error {
	for _, endpoint := range lb.endpoints {
		if err := lb.tcp.ReconcileRoute(endpoint, slices.Values(upstreams)); err != nil {
			return err
		}
	}

	return nil
}

func (lb *controlPlaneLB) Healthy() (bool, error) {
	for _, endpoint := range lb.endpoints {
		healthy, err := lb.tcp.IsRouteHealthy(endpoint)
		if err != nil || !healthy {
			return false, err
		}
	}

	return true, nil
}

func (lb *controlPlaneLB) Shutdown() error {
	if err := lb.tcp.Close(); err != nil {
		return err
	}

	lb.tcp.Wait() //nolint:errcheck

	return nil
}
