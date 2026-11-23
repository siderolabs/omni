// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package loadbalancer_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/loadbalancer"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestDefaultNew(t *testing.T) {
	t.Parallel()

	ipv6Loopback, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is not available: %v", err)
	}

	require.NoError(t, ipv6Loopback.Close())

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok")) //nolint:errcheck
	}))
	t.Cleanup(upstream.Close)

	lb, port := startLoadBalancer(t)

	require.NoError(t, lb.UpdateUpstreams([]string{upstream.Listener.Addr().String()}))

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		healthy, healthErr := lb.Healthy()
		assert.NoError(collect, healthErr)
		assert.True(collect, healthy)
	}, 10*time.Second, 100*time.Millisecond)

	for _, host := range []string{"::1", "127.0.0.1"} {
		assertProxied(t, net.JoinHostPort(host, strconv.Itoa(port)))
	}

	require.NoError(t, lb.Shutdown())

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	require.NoError(t, err, "the port should be free after shutdown")
	require.NoError(t, listener.Close())
}

func TestDefaultNewAddressNotAvailable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	start := time.Now()

	// 192.0.2.1 is a documentation address, it is not assigned to any interface
	_, err := loadbalancer.DefaultNew(ctx, "192.0.2.1", 0, zaptest.NewLogger(t), config.Default().Services.LoadBalancer)
	require.Error(t, err)

	assert.Less(t, time.Since(start), 5*time.Second, "the bind retry should stop when the context is canceled")
}

// startLoadBalancer starts a load balancer on a free port, and picks another port if the free one got taken in the meantime.
func startLoadBalancer(t *testing.T) (loadbalancer.LoadBalancer, int) {
	t.Helper()

	for range 3 {
		port := freePort(t)

		lb, err := loadbalancer.DefaultNew(t.Context(), "::1", port, zaptest.NewLogger(t), config.Default().Services.LoadBalancer)
		if errors.Is(err, syscall.EADDRINUSE) {
			continue
		}

		require.NoError(t, err)

		return lb, port
	}

	require.FailNow(t, "no free port found")

	return nil, 0
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	require.NoError(t, listener.Close())

	return addr.Port
}

func assertProxied(t *testing.T, address string) {
	t.Helper()

	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+address, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, "ok", string(body), "the load balancer on %s should proxy to the upstream", address)
}
