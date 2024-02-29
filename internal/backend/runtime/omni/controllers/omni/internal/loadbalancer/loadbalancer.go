// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package loadbalancer provides wrappers to run controlplane Kubernetes API loadbalancers.
package loadbalancer

import (
	"github.com/siderolabs/go-loadbalancer/controlplane"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// LoadBalancer is the interface needed by the Manager.
type LoadBalancer interface {
	Start(upstreamCh <-chan []string) error
	Shutdown() error
	Healthy() (bool, error)
}

// NewFunc is a function type whose implementation should create a new load balancer.
type NewFunc func(bindAddress string, bindPort int, logger *zap.Logger) (LoadBalancer, error)

// DefaultNew returns a new load balancer with default settings.
func DefaultNew(bindAddress string, bindPort int, logger *zap.Logger) (LoadBalancer, error) { //nolint:ireturn
	return controlplane.NewLoadBalancer(
		bindAddress,
		bindPort,
		logger.WithOptions(zap.IncreaseLevel(zap.ErrorLevel)), // silence the load balancer logs
		controlplane.WithDialTimeout(config.Config.LoadBalancer.DialTimeout),
		controlplane.WithKeepAlivePeriod(config.Config.LoadBalancer.KeepAlivePeriod),
		controlplane.WithTCPUserTimeout(config.Config.LoadBalancer.TCPUserTimeout),
		controlplane.WithHealthCheckOptions(
			upstream.WithHealthcheckInterval(config.Config.LoadBalancer.HealthCheckInterval),
			upstream.WithHealthcheckTimeout(config.Config.LoadBalancer.HealthCheckTimeout),
		),
	)
}
