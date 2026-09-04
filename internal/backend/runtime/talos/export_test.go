// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
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
