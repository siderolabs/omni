// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"fmt"
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

const aliasClusterIDSeparator = ":"

// Reconciler reconciles the load balancers for a cluster.
type Reconciler struct {
	clusterToAliasToLB map[resource.ID]map[string]*lb.LB
	aliasToCluster     map[string]resource.ID

	logger      *zap.Logger
	lbLogger    *zap.Logger
	proxyDialer *net.Dialer

	logLevel zapcore.Level
	mu       sync.Mutex
}

// NewReconciler creates a new Reconciler.
func NewReconciler(logger *zap.Logger, logLevel zapcore.Level) *Reconciler {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Reconciler{
		clusterToAliasToLB: map[resource.ID]map[string]*lb.LB{},
		aliasToCluster:     map[string]resource.ID{},
		logger:             logger,
		lbLogger:           logger.WithOptions(zap.IncreaseLevel(zapcore.ErrorLevel)),
		logLevel:           logLevel,
		proxyDialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}
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

		aliasLB, err = lb.New(upstreamAddresses, registry.lbLogger,
			upstream.WithHealthcheckTimeout(time.Second),
			upstream.WithHealthcheckInterval(time.Minute),
		)
		if err != nil {
			return fmt.Errorf("failed to create LB for %q/%q: %w", cluster, alias, err)
		}
	}

	aliasLB.Reconcile(upstreamAddresses)

	registry.aliasToCluster[alias] = cluster

	if aliasToLB := registry.clusterToAliasToLB[cluster]; aliasToLB == nil {
		registry.clusterToAliasToLB[cluster] = map[string]*lb.LB{}
	}

	registry.clusterToAliasToLB[cluster][alias] = aliasLB

	return nil
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

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
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
