// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// ReleaseForMachine exposes releaseForMachine to external tests.
func (factory *ClientFactory) ReleaseForMachine(clusterID, machineID string) {
	factory.releaseForMachine(clusterID, machineID)
}

// CacheLen returns the number of cached clients.
func (factory *ClientFactory) CacheLen() int {
	return factory.cache.Len()
}

// ActiveClients returns the value of the active clients metric for the client type.
func (factory *ClientFactory) ActiveClients(typ string) int {
	return int(testutil.ToFloat64(factory.metricActiveClients.WithLabelValues(typ)))
}

// ReaderCertificate exposes the base64 encoded certificate of readerCredentials to external tests.
func (factory *ClientFactory) ReaderCertificate(ctx context.Context, clusterID string) (string, error) {
	_, crt, _, err := factory.readerCredentials(ctx, clusterID)

	return crt, err
}
