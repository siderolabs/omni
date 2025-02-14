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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
)

// Reconciler reconciles the load balancers for a cluster.
//
//nolint:govet
type Reconciler struct {
	logger    *zap.Logger
	logLevel  zapcore.Level
	transport *http.Transport

	mu                      sync.Mutex
	clusterToAliasToBackend map[resource.ID]map[string]*upstream.List[*backend]
	aliasToCluster          map[string]resource.ID
}

// NewReconciler creates a new Reconciler.
func NewReconciler(logger *zap.Logger, logLevel zapcore.Level) *Reconciler {
	if logger == nil {
		logger = zap.NewNop()
	}

	trans := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if err := http2.ConfigureTransport(trans); err != nil {
		panic(fmt.Errorf("failed to configure HTTP/2 transport: %w", err))
	}

	rec := &Reconciler{
		logger:                  logger,
		logLevel:                logLevel,
		transport:               trans,
		clusterToAliasToBackend: map[resource.ID]map[string]*upstream.List[*backend]{},
		aliasToCluster:          map[string]resource.ID{},
	}

	rec.setTransportDialer(trans)

	return rec
}

// Reconcile reconciles the workload proxy load balancers for a cluster.
func (registry *Reconciler) Reconcile(cluster resource.ID, aliasToUpstreamAddresses map[string][]string) error {
	registry.logger.Log(registry.logLevel, "reconcile LBs", zap.String("cluster", cluster))

	registry.mu.Lock()
	defer registry.mu.Unlock()

	var errs error

	// drop removed LBs
	for alias := range registry.clusterToAliasToBackend[cluster] {
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

	if aliasToB := registry.clusterToAliasToBackend[cluster]; aliasToB == nil {
		registry.clusterToAliasToBackend[cluster] = map[string]*upstream.List[*backend]{}
	}

	u := registry.clusterToAliasToBackend[cluster][alias]

	if u == nil {
		var err error

		if u, err = upstream.NewListWithCmp(
			xiter.Map(func(addr string) *backend { return &backend{Addr: addr} }, slices.Values(upstreamAddresses)),
			func(b *backend, b2 *backend) bool { return b.Addr == b2.Addr },
		); err != nil {
			return err
		}
	} else {
		u.Reconcile(xiter.Map(func(addr string) *backend { return &backend{Addr: addr} }, slices.Values(upstreamAddresses)))
	}

	registry.clusterToAliasToBackend[cluster][alias] = u

	registry.aliasToCluster[alias] = cluster

	return nil
}

func (registry *Reconciler) removeLB(cluster resource.ID, alias string) {
	registry.logger.Log(registry.logLevel, "remove LB", zap.String("cluster", cluster), zap.String("alias", alias))

	aliasToLB := registry.clusterToAliasToBackend[cluster]

	if val, ok := aliasToLB[alias]; ok {
		val.Shutdown()
	}

	delete(aliasToLB, alias)
	delete(registry.aliasToCluster, alias)

	if len(aliasToLB) == 0 {
		delete(registry.clusterToAliasToBackend, cluster)
	}
}

// GetProxy returns a proxy for the exposed service, targeting the load balancer for the given alias.
func (registry *Reconciler) GetProxy(alias string) (http.Handler, resource.ID, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	clusterID, ok := registry.aliasToCluster[alias]
	if !ok {
		return nil, "", nil
	}

	upstreams := registry.clusterToAliasToBackend[clusterID][alias]
	if upstreams == nil {
		return nil, "", nil
	}

	return &httputil.ReverseProxy{
		Director:  func(req *http.Request) { req.URL.Host = hostPortForAlias(clusterID, alias) },
		Transport: registry.transport,
	}, clusterID, nil
}

// hostPortForAlias returns a unique IP:port for the given cluster and alias.
func hostPortForAlias(clusterID resource.ID, alias string) string {
	return fmt.Sprintf("%s_%s:4242", clusterID, alias)
}

func (registry *Reconciler) setTransportDialer(trans *http.Transport) {
	d := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	trans.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		clusterID, alias, found := strings.Cut(addr, "_")
		if !found {
			return nil, fmt.Errorf("invalid proxy dst address: %s", addr)
		}

		upstreams := registry.clusterToAliasToBackend[clusterID][alias]
		if upstreams == nil {
			return nil, fmt.Errorf("no upstreams for cluster %s with alias %s", clusterID, alias)
		}

		pick, err := upstreams.Pick()
		if err != nil {
			return nil, fmt.Errorf("failed to pick upstream: %w", err)
		}

		return d.DialContext(ctx, network, pick.Addr)
	}
}

type backend struct {
	Addr string
}

func (b *backend) HealthCheck(context.Context) (upstream.Tier, error) { return 0, nil }
