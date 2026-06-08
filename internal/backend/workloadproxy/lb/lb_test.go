// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lb_test

import (
	"testing"
	"time"

	"github.com/siderolabs/go-loadbalancer/upstream"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/workloadproxy/lb"
)

// TestPickAddressFallback verifies that when every upstream is benched, PickAddress still
// hands out a known upstream (round-robin) instead of failing. This is the recovery path
// for a cold load balancer right after an Omni restart, where a single failed health check
// benches every node and the proxy would otherwise return 502 until the next health check.
func TestPickAddressFallback(t *testing.T) {
	t.Parallel()

	addresses := []string{"127.0.0.1:1", "127.0.0.2:1"} // refused on connect, so every node stays benched

	// A negative initial score starts every upstream benched, and the failing health checks keep
	// them there, so Pick reports no upstreams available and the fallback is exercised.
	l, err := lb.New(
		addresses, zaptest.NewLogger(t),
		upstream.WithInitialScore(-1),
		upstream.WithHealthcheckTimeout(time.Second),
		upstream.WithHealthcheckInterval(time.Hour),
	)
	require.NoError(t, err)

	t.Cleanup(l.Shutdown)

	first, err := l.PickAddress()
	require.NoError(t, err)
	require.Contains(t, addresses, first)

	second, err := l.PickAddress()
	require.NoError(t, err)
	require.Contains(t, addresses, second)

	require.NotEqual(t, first, second, "fallback should round-robin across all known upstreams")
}
