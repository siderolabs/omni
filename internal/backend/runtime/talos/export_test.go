// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"runtime/pprof"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

// IdleTimeout is exported for testing.
const IdleTimeout = talosClientIdleTimeout

// Sweep exposes sweep to external tests.
func (factory *ClientFactory) Sweep(now time.Time) {
	factory.sweep(now)
}

// Stop exposes stop to external tests.
func (factory *ClientFactory) Stop() {
	factory.stop()
}

// ReleaseForMachine exposes releaseForMachine to external tests.
func (factory *ClientFactory) ReleaseForMachine(clusterID, machineID string) {
	factory.releaseForMachine(clusterID, machineID)
}

// SetCacheSize sets the maximum number of cached clients.
func (factory *ClientFactory) SetCacheSize(size int) {
	factory.mu.Lock()
	defer factory.mu.Unlock()

	factory.cacheSize = size
}

// Cached returns whether a client for the machine is in the cache.
func (factory *ClientFactory) Cached(clusterID, machineID string) bool {
	factory.mu.Lock()
	defer factory.mu.Unlock()

	_, ok := factory.entries[buildCacheKey(clusterID, machineID)]

	return ok
}

// CacheSizeMetric returns the value of the cache size gauge for the given client type.
func (factory *ClientFactory) CacheSizeMetric(typ string) int {
	return int(testutil.ToFloat64(factory.metricCacheSize.WithLabelValues(typ)))
}

// CacheLen returns the number of cached clients.
func (factory *ClientFactory) CacheLen() int {
	factory.mu.Lock()
	defer factory.mu.Unlock()

	return len(factory.entries)
}

// ActiveClients returns the value of the active clients metric for the client type.
func (factory *ClientFactory) ActiveClients(typ string) int {
	return int(testutil.ToFloat64(factory.metricActiveClients.WithLabelValues(typ)))
}

// LeakedClients returns the value of the leaked clients metric.
func (factory *ClientFactory) LeakedClients() int {
	return int(testutil.ToFloat64(factory.metricLeakedClients))
}

// NewClientFactoryWithProfile creates a ClientFactory which records its open clients in the given profile, so a test
// can look at its own clients only.
func NewClientFactoryWithProfile(omniState state.State, logger *zap.Logger, openClients *pprof.Profile) *ClientFactory {
	return newClientFactory(omniState, logger, openClients)
}
