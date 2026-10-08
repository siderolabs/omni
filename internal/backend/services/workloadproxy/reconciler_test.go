// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
)

func TestReconciler(t *testing.T) {
	t.Parallel()

	reconciler := workloadproxy.NewReconciler(zaptest.NewLogger(t), zapcore.InfoLevel, 30*time.Second)

	err := reconciler.Reconcile("cluster1", map[string][]string{
		"alias1": {
			"upstream1",
			"upstream2",
		},
		"alias2": {
			"upstream3",
			"upstream4",
		},
	})
	require.NoError(t, err)

	proxy, id, err := reconciler.GetProxy("alias1")
	require.NoError(t, err)

	require.NotNil(t, proxy)
	require.Equal(t, "cluster1", id)

	proxy, id, err = reconciler.GetProxy("alias2")
	require.NoError(t, err)

	require.NotNil(t, proxy)
	require.Equal(t, "cluster1", id)

	err = reconciler.Reconcile("cluster2", nil)
	require.NoError(t, err)

	proxy, id, err = reconciler.GetProxy("alias1")
	require.NoError(t, err)

	require.NotNil(t, proxy)
	require.Equal(t, "cluster1", id)

	proxy, id, err = reconciler.GetProxy("alias3")
	require.NoError(t, err)

	require.Nil(t, proxy)
	require.Zero(t, id)
}

// TestReconcilerProxyCookies checks that the workload proxy cookies are not forwarded to the workload, while the
// cookies of the workload itself are forwarded untouched.
func TestReconcilerProxyCookies(t *testing.T) {
	t.Parallel()

	var (
		upstreamHost    string
		upstreamQuery   string
		upstreamHeader  http.Header
		upstreamCookies []string
	)

	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		upstreamHost = r.Host
		upstreamQuery = r.URL.RawQuery
		upstreamHeader = r.Header.Clone()
		upstreamCookies = r.Header.Values("Cookie")
	}))
	t.Cleanup(upstream.Close)

	reconciler := workloadproxy.NewReconciler(zaptest.NewLogger(t), zapcore.InfoLevel, 0)

	require.NoError(t, reconciler.Reconcile("cluster", map[string][]string{"alias": {upstream.Listener.Addr().String()}}))

	proxy, _, err := reconciler.GetProxy("alias")
	require.NoError(t, err)

	serve := func(cookieLines ...string) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://alias.proxy.example.com/path?filter=a;b", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
		req.Header.Set("X-Forwarded-Host", "app.example.com")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("Forwarded", "proto=https;host=app.example.com")

		for _, line := range cookieLines {
			req.Header.Add("Cookie", line)
		}

		rr := httptest.NewRecorder()

		proxy.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, "alias.proxy.example.com", upstreamHost)
		require.Equal(t, "filter=a;b", upstreamQuery)
		require.Equal(t, "203.0.113.9, 192.0.2.1", upstreamHeader.Get("X-Forwarded-For"))
		require.Equal(t, "app.example.com", upstreamHeader.Get("X-Forwarded-Host"))
		require.Equal(t, "https", upstreamHeader.Get("X-Forwarded-Proto"))
		require.Equal(t, "proto=https;host=app.example.com", upstreamHeader.Get("Forwarded"))
	}

	serve(workloadproxy.PublicKeyIDCookie+"=key-id; session=abc; "+workloadproxy.PublicKeyIDSignatureBase64Cookie+"=c2ln", "theme=dark")
	require.Equal(t, []string{"session=abc", "theme=dark"}, upstreamCookies)

	serve(workloadproxy.PublicKeyIDCookie + "=key-id; " + workloadproxy.PublicKeyIDSignatureBase64Cookie + "=c2ln;")
	require.Empty(t, upstreamCookies)
}
