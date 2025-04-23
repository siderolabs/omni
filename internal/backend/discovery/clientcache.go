// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/omni/internal/backend/discovery/internal/discovery"
)

// ClientCache is a discovery client cache.
type ClientCache struct {
	cache *expirable.LRU[string, *discovery.Client]
	sf    singleflight.Group

	metricCacheSize, metricActiveClients prometheus.Gauge
	metricCacheHits, metricCacheMisses   prometheus.Counter
	logger                               *zap.Logger
}

// NewClientCache creates a new client cache.
func NewClientCache(logger *zap.Logger) *ClientCache {
	metricActiveClients := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "omni_discovery_clientcache_active_clients",
		Help: "Number of active Discovery clients created by Discovery client cache.",
	})

	return &ClientCache{
		logger: logger,
		cache: expirable.NewLRU[string, *discovery.Client](16, func(endpoint string, client *discovery.Client) {
			if err := client.Close(); err != nil {
				logger.Error("failed to close discovery client", zap.String("endpoint", endpoint), zap.Error(err))
			}

			metricActiveClients.Dec()
			logger.Debug("evicted cached discovery client", zap.String("endpoint", endpoint))
		}, 1*time.Hour),
		metricCacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_discovery_clientcache_cache_size",
			Help: "Number of Discovery clients in the cache of Discovery client cache.",
		}),
		metricActiveClients: metricActiveClients,
		metricCacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_discovery_clientcache_cache_hits_total",
			Help: "Number of Discovery client cache hits.",
		}),
		metricCacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_discovery_clientcache_cache_misses_total",
			Help: "Number of Discovery client cache misses.",
		}),
	}
}

// get constructs a discovery service client for the given endpoint.
func (cache *ClientCache) get(ctx context.Context, endpoint string) (*discovery.Client, error) {
	if cli, ok := cache.cache.Get(endpoint); ok {
		cache.logger.Debug("cache hit, returning cached Discovery client", zap.String("endpoint", endpoint))

		cache.metricCacheHits.Inc()

		return cli, nil
	}

	ch := cache.sf.DoChan(endpoint, func() (any, error) {
		cache.logger.Debug("cache miss, creating new Discovery client", zap.String("endpoint", endpoint))

		cache.metricCacheMisses.Inc()

		cli, err := discovery.NewClient(endpoint)
		if err != nil {
			return nil, err
		}

		cache.metricActiveClients.Inc()
		cache.cache.Add(endpoint, cli)

		return cli, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		cli := res.Val.(*discovery.Client) //nolint:forcetypeassert,errcheck

		return cli, nil
	}
}

// Close purges the discovery client cache, closing and releasing all cached clients.
func (cache *ClientCache) Close() {
	cache.cache.Purge()
	cache.metricCacheSize.Set(0)
	cache.metricActiveClients.Set(0)
}

// AffiliateDelete deletes the given affiliate from the given cluster from the discovery service running on the given endpoint.
func (cache *ClientCache) AffiliateDelete(ctx context.Context, endpoint, cluster, affiliate string) error {
	cli, err := cache.get(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("failed to get discovery client for endpoint %q: %w", endpoint, err)
	}

	if err = cli.AffiliateDelete(ctx, cluster, affiliate); err != nil {
		return fmt.Errorf("failed to delete affiliate %q for cluster %q from %q: %w", affiliate, cluster, endpoint, err)
	}

	return nil
}

// Describe implements prom.Collector interface.
func (cache *ClientCache) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(cache, ch)
}

// Collect implements prom.Collector interface.
func (cache *ClientCache) Collect(ch chan<- prometheus.Metric) {
	cache.metricActiveClients.Collect(ch)

	cache.metricCacheSize.Set(float64(cache.cache.Len()))
	cache.metricCacheSize.Collect(ch)

	cache.metricCacheHits.Collect(ch)
	cache.metricCacheMisses.Collect(ch)
}

var _ prometheus.Collector = &ClientCache{}
