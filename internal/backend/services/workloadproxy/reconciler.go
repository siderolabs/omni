// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/omni/internal/backend/workloadproxy/lb"
)

type loadBalancer interface {
	Reconcile(upstreamAddresses []string) error
	PickAddress() (string, error)
	Notify() error
	Shutdown()
}

const aliasClusterIDSeparator = ":"

// Reconciler reconciles the load balancers for a cluster.
type Reconciler struct {
	clusterToAliasToLB map[resource.ID]map[string]loadBalancer
	aliasToCluster     map[string]resource.ID

	logger      *zap.Logger
	lbLogger    *zap.Logger
	proxyDialer *net.Dialer

	logLevel zapcore.Level
	mu       sync.Mutex

	lazyStopAfter time.Duration
}

// NewReconciler creates a new Reconciler.
func NewReconciler(logger *zap.Logger, logLevel zapcore.Level, lazyStopAfter time.Duration) *Reconciler {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Reconciler{
		clusterToAliasToLB: map[resource.ID]map[string]loadBalancer{},
		aliasToCluster:     map[string]resource.ID{},
		logger:             logger,
		lbLogger:           logger.WithOptions(zap.IncreaseLevel(zapcore.ErrorLevel)),
		logLevel:           logLevel,
		proxyDialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		lazyStopAfter: lazyStopAfter,
	}
}

// Run starts the reconciler, periodically notifying all load balancers to refresh their state.
func (registry *Reconciler) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			registry.shutdown()

			return nil
		case <-ticker.C:
			if err := registry.notifyAll(); err != nil {
				return fmt.Errorf("failed to notify all LBs: %w", err)
			}
		}
	}
}

func (registry *Reconciler) shutdown() {
	registry.logger.Debug("shutting down reconciler")

	registry.mu.Lock()
	defer registry.mu.Unlock()

	for cluster, aliasToLB := range registry.clusterToAliasToLB {
		for alias := range aliasToLB {
			registry.removeLB(cluster, alias)
		}
	}
}

func (registry *Reconciler) notifyAll() error {
	registry.mu.Lock()
	clusterToAliasToLB := maps.Clone(registry.clusterToAliasToLB)
	registry.mu.Unlock()

	count := 0

	for _, aliasToLB := range clusterToAliasToLB {
		for _, aliasLB := range aliasToLB {
			if aliasLB != nil {
				if err := aliasLB.Notify(); err != nil {
					if errors.Is(err, lb.ErrShutdown) {
						registry.logger.Warn("load balancer is already shut down", zap.Error(err))

						continue
					}

					return err
				}

				count++
			}
		}
	}

	registry.logger.Debug("notified all LBs", zap.Int("count", count))

	return nil
}

// Reconcile reconciles the workload proxy load balancers for a cluster.
func (registry *Reconciler) Reconcile(cluster resource.ID, aliasToUpstreamAddresses map[string][]string) error {
	registry.logger.Log(registry.logLevel, "reconcile LBs", zap.String("cluster", cluster))

	registry.mu.Lock()
	defer registry.mu.Unlock()

	var errs error

	// drop removed LBs
	for alias := range registry.clusterToAliasToLB[cluster] {
		if _, ok := aliasToUpstreamAddresses[alias]; ok { // still present
			continue
		}

		// not present anymore, remove
		registry.removeLB(cluster, alias)
	}

	// ensure new LBs
	for alias, upstreamAddresses := range aliasToUpstreamAddresses {
		if err := registry.ensureLB(cluster, alias, upstreamAddresses); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("failed to register LB: %w", err))
		}
	}

	return errs
}

// ensureLB ensures that a load balancer exists and started for the given cluster and alias, targeting the given upstream addresses.
func (registry *Reconciler) ensureLB(cluster resource.ID, alias string, upstreamAddresses []string) error {
	registry.logger.Log(registry.logLevel, "ensure LB", zap.String("cluster", cluster), zap.String("alias", alias), zap.Strings("upstream_addresses", upstreamAddresses))

	aliasLB := registry.clusterToAliasToLB[cluster][alias]

	if aliasLB == nil { // no LB yet, create it
		var err error
		if aliasLB, err = registry.newLB(upstreamAddresses); err != nil {
			return fmt.Errorf("failed to create LB for %q/%q: %w", cluster, alias, err)
		}
	}

	if err := aliasLB.Reconcile(upstreamAddresses); err != nil {
		return fmt.Errorf("failed to reconcile LB for %q/%q: %w", cluster, alias, err)
	}

	registry.aliasToCluster[alias] = cluster

	if aliasToLB := registry.clusterToAliasToLB[cluster]; aliasToLB == nil {
		registry.clusterToAliasToLB[cluster] = map[string]loadBalancer{}
	}

	registry.clusterToAliasToLB[cluster][alias] = aliasLB

	return nil
}

func (registry *Reconciler) newLB(upstreamAddresses []string) (loadBalancer, error) {
	opts := []upstream.ListOption{
		upstream.WithHealthcheckTimeout(time.Second),
		upstream.WithHealthcheckInterval(time.Minute),
		upstream.WithHealthCheckJitter(0.1),
		// Start with a zero score - we wait for the first health check to complete on .Pick(),
		// after that is done, the score of non-healthy remotes will be negative, so it won't be picked.
		upstream.WithInitialScore(0),
	}

	if registry.lazyStopAfter > 0 {
		lazyLB, err := lb.NewLazy(upstreamAddresses, registry.lazyStopAfter, registry.lbLogger, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create lazy LB: %w", err)
		}

		return lazyLB, nil
	}

	newLB, err := lb.New(upstreamAddresses, registry.lbLogger, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create LB: %w", err)
	}

	return newLB, nil
}

// GetProxy returns a proxy for the exposed service, targeting the load balancer for the given alias.
func (registry *Reconciler) GetProxy(alias string) (http.Handler, resource.ID, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	clusterID, ok := registry.aliasToCluster[alias]
	if !ok {
		return nil, "", nil
	}

	aliasLB := registry.clusterToAliasToLB[clusterID][alias]
	if aliasLB == nil {
		return nil, clusterID, nil
	}

	hostPort := alias + aliasClusterIDSeparator + clusterID + ":80"

	targetURL := &url.URL{
		Scheme: "http",
		Host:   hostPort,
	}

	proxyErrorLogger, err := zap.NewStdLogAt(registry.logger.With(zap.String("sub_component", "reverse_proxy")), zapcore.InfoLevel)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create proxy error logger: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorLog = proxyErrorLogger
	proxy.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return registry.dialProxy(ctx, network, address)
		},
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return proxy, clusterID, nil
}

func (registry *Reconciler) dialProxy(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("failed to split host and port from address %s: %w", address, err)
	}

	parts := strings.SplitN(host, aliasClusterIDSeparator, 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid address format: %s", address)
	}

	alias := parts[0]
	clusterID := parts[1]

	destAddress, err := registry.pickDestAddress(clusterID, alias)
	if err != nil {
		return nil, fmt.Errorf("failed to pick destination address for alias %s in cluster %s: %w", alias, clusterID, err)
	}

	return registry.proxyDialer.DialContext(ctx, network, destAddress)
}

func (registry *Reconciler) pickDestAddress(cluster resource.ID, alias string) (string, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	aliasLB := registry.clusterToAliasToLB[cluster][alias]
	if aliasLB == nil {
		return "", fmt.Errorf("no load balancer found for cluster %s and alias %s", cluster, alias)
	}

	destAddress, err := aliasLB.PickAddress()
	if err != nil {
		return "", fmt.Errorf("failed to pick address for alias %s: %w", alias, err)
	}

	return destAddress, nil
}

func (registry *Reconciler) removeLB(cluster resource.ID, alias string) {
	registry.logger.Log(registry.logLevel, "remove LB", zap.String("cluster", cluster), zap.String("alias", alias))

	aliasToLB := registry.clusterToAliasToLB[cluster]
	aliasLB := aliasToLB[alias]

	if aliasLB != nil {
		aliasLB.Shutdown()
	}

	delete(aliasToLB, alias)
	delete(registry.aliasToCluster, alias)

	if len(aliasToLB) == 0 {
		delete(registry.clusterToAliasToLB, cluster)
	}
}
