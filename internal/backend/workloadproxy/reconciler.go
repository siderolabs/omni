// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/akutz/memconn"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-loadbalancer/loadbalancer"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"github.com/siderolabs/tcpproxy"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type lbStatus struct {
	lb *loadbalancer.TCP
}

// Close closes the load balancer and waits for it to terminate.
//
// It should be called only if it was successfully started, otherwise it blocks indefinitely on the Wait call.
func (lbSts *lbStatus) Close() error {
	var lb *loadbalancer.TCP

	lb, lbSts.lb = lbSts.lb, nil

	if lb == nil {
		return nil
	}

	if err := lb.Close(); err != nil {
		return fmt.Errorf("failed to close LB: %w", err)
	}

	if err := lb.Wait(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("failed to wait for load balancer: %w", err)
	}

	return nil
}

// Reconciler reconciles the load balancers for a cluster.
type Reconciler struct {
	clusterToAliasToLBStatus map[resource.ID]map[string]*lbStatus
	aliasToCluster           map[string]resource.ID

	connProvider *memconn.Provider
	logger       *zap.Logger
	lbLogger     *zap.Logger
	logLevel     zapcore.Level
	mu           sync.Mutex
}

// NewReconciler creates a new Reconciler.
func NewReconciler(logger *zap.Logger, logLevel zapcore.Level) *Reconciler {
	// use an in-memory transport for the connections to the load balancer
	provider := &memconn.Provider{}

	provider.MapNetwork("tcp", "memu")

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Reconciler{
		clusterToAliasToLBStatus: map[resource.ID]map[string]*lbStatus{},
		aliasToCluster:           map[string]resource.ID{},
		connProvider:             provider,
		logger:                   logger,
		lbLogger:                 logger.WithOptions(zap.IncreaseLevel(zapcore.ErrorLevel)),
		logLevel:                 logLevel,
	}
}

// Reconcile reconciles the workload proxy load balancers for a cluster.
func (registry *Reconciler) Reconcile(cluster resource.ID, aliasToUpstreamAddresses map[string][]string) error {
	registry.logger.Log(registry.logLevel, "reconcile LBs", zap.String("cluster", cluster))

	registry.mu.Lock()
	defer registry.mu.Unlock()

	var errs error

	// drop removed LBs
	for alias := range registry.clusterToAliasToLBStatus[cluster] {
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

	hostPort := registry.hostPortForAlias(cluster, alias)
	lbSts := registry.clusterToAliasToLBStatus[cluster][alias]

	if lbSts == nil { // no LB yet, create and start it
		tcpLB := &loadbalancer.TCP{
			Logger: registry.lbLogger,
			Proxy: tcpproxy.Proxy{
				ListenFunc: registry.connProvider.Listen,
			},
			DialTimeout:    1 * time.Second,
			TCPUserTimeout: 5 * time.Second,
		}

		if err := tcpLB.AddRoute(
			hostPort, upstreamAddresses,
			upstream.WithHealthcheckTimeout(time.Second),
			upstream.WithHealthcheckInterval(time.Minute),
		); err != nil {
			return fmt.Errorf("failed to add route for %q/%q: %w", cluster, alias, err)
		}

		if err := tcpLB.Start(); err != nil {
			registry.logger.Log(registry.logLevel, "failed to start LB, attempt to stop it")

			startErr := fmt.Errorf("failed to start LB for %q/%q: %w", cluster, alias, err)

			// we still need to close the loadbalancer, so that the health checks goroutines get terminated
			if closeErr := tcpLB.Close(); closeErr != nil {
				return errors.Join(startErr, fmt.Errorf("failed to close LB: %w", closeErr))
			}

			return startErr
		}

		lbSts = &lbStatus{
			lb: tcpLB,
		}
	}

	if err := lbSts.lb.ReconcileRoute(hostPort, upstreamAddresses); err != nil {
		return fmt.Errorf("failed to reconcile route for %q/%q: %w", cluster, alias, err)
	}

	registry.aliasToCluster[alias] = cluster

	if aliasToLB := registry.clusterToAliasToLBStatus[cluster]; aliasToLB == nil {
		registry.clusterToAliasToLBStatus[cluster] = map[string]*lbStatus{}
	}

	registry.clusterToAliasToLBStatus[cluster][alias] = lbSts

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

	lbSts := registry.clusterToAliasToLBStatus[clusterID][alias]
	if lbSts == nil || lbSts.lb == nil {
		return nil, "", nil
	}

	hostPort := registry.hostPortForAlias(clusterID, alias)

	targetURL := &url.URL{
		Scheme: "http",
		Host:   hostPort,
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = &http.Transport{
		DialContext: registry.connProvider.DialContext,
	}

	return proxy, clusterID, nil
}

func (registry *Reconciler) removeLB(cluster resource.ID, alias string) {
	registry.logger.Log(registry.logLevel, "remove LB", zap.String("cluster", cluster), zap.String("alias", alias))

	aliasToLB := registry.clusterToAliasToLBStatus[cluster]
	lbSts := aliasToLB[alias]

	if lbSts != nil {
		if err := lbSts.Close(); err != nil {
			registry.logger.Error("failed to close LB", zap.String("cluster", cluster), zap.String("alias", alias), zap.Error(err))
		}
	}

	delete(aliasToLB, alias)
	delete(registry.aliasToCluster, alias)

	if len(aliasToLB) == 0 {
		delete(registry.clusterToAliasToLBStatus, cluster)
	}
}

// hostPortForAlias returns a unique IP:port for the given cluster and alias.
//
// The value is arbitrary, as it uses in-memory transport, never reached via the network.
func (registry *Reconciler) hostPortForAlias(clusterID resource.ID, alias string) string {
	return fmt.Sprintf("%s_%s:4242", clusterID, alias)
}
