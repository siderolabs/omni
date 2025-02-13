// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestReconciler(t *testing.T) {
	reconciler := workloadproxy.NewReconciler(zaptest.NewLogger(t), zapcore.InfoLevel)

	t.Cleanup(reconciler.Shutdown)

	t.Cleanup(func() {
		for dsc := range reconciler.All() {
			t.Logf("%s -> %s -> %v -> %s : inuse_port: %q", dsc.Alias, dsc.ClusterID, dsc.Upstream, dsc.Port, dsc.InUsePort)
		}
	})

	t.Run("group", func(t *testing.T) {
		t.Run("path1", func(t *testing.T) {
			t.Parallel()
			testPath1(t, reconciler)
		})

		t.Run("path2", func(t *testing.T) {
			t.Parallel()
			testPath2(t, reconciler)
		})
	})
}

func testPath1(t *testing.T, reconciler *workloadproxy.Reconciler) {
	tests := []struct {
		name    string
		actions []action
	}{
		{
			name: "init 1 cluster",
			actions: []action{
				addClusterData(
					"cluster1",
					[]string{"192.168.1.1", "192.168.1.2"},
					pair.MakePair("alias1", "8080"),
					pair.MakePair("alias2", "8081"),
					pair.MakePair("alias3", "8082"),
				),
				checkProxy("alias1", "cluster1"),
				checkProxy("alias2", "cluster1"),
				checkProxy("alias3", "cluster1"),
				checkProxy("alias10", ""),
				inUsePortIs("cluster1", "8080", "8081", "8082"),
			},
		},
		{
			name: "init 2 cluster, replace alias1 and alias2",
			actions: []action{
				failToAddClusterData(
					"cluster2",
					[]string{"192.168.2.1", "192.168.2.2"},
					pair.MakePair("alias1", "8080"),
					pair.MakePair("alias2", "8081"),
					pair.MakePair("alias4", "8083"),
				),
				dropAlias("alias1"),
				dropAlias("alias2"),
				addClusterData(
					"cluster2",
					[]string{"192.168.2.1", "192.168.2.2"},
					pair.MakePair("alias1", "8080"),
					pair.MakePair("alias2", "8081"),
					pair.MakePair("alias4", "8083"),
				),
				checkProxy("alias1", "cluster2"),
				checkProxy("alias2", "cluster2"),
				checkProxy("alias3", "cluster1"),
				checkProxy("alias4", "cluster2"),
				checkProxy("alias10", ""),
				inUsePortIs("cluster1", "8082"),
				inUsePortIs("cluster2", "8080", "8081", "8083"),
			},
		},
		{
			name: "remove 1 cluster",
			actions: []action{
				addClusterData("cluster1", nil),
				checkProxy("alias1", "cluster2"),
				checkProxy("alias2", "cluster2"),
				checkProxy("alias3", ""),
				checkProxy("alias4", "cluster2"),
				inUsePortIs("cluster2", "8080", "8081", "8083"),
			},
		},
		{
			name: "add 1 cluster back",
			actions: []action{
				dropAlias("alias1"),
				dropAlias("alias2"),
				addClusterData(
					"cluster1",
					[]string{"192.168.1.1", "192.168.1.2"},
					pair.MakePair("alias1", "8080"),
					pair.MakePair("alias2", "8081"),
					pair.MakePair("alias3", "8082"),
				),
				checkProxy("alias1", "cluster1"),
				checkProxy("alias2", "cluster1"),
				checkProxy("alias3", "cluster1"),
				checkProxy("alias4", "cluster2"),
				inUsePortIs("cluster2", "8083"),
				inUsePortIs("cluster1", "8080", "8081", "8082"),
			},
		},
		{
			name: "add 3 cluster, rewrite part of cluster 1 aliases",
			actions: []action{
				dropAlias("alias1"),
				dropAlias("alias2"),
				addClusterData(
					"cluster3",
					[]string{"192.168.3.1", "192.168.3.2"},
					pair.MakePair("alias1", "8080"),
					pair.MakePair("alias2", "8081"),
					pair.MakePair("alias5", "8084"),
				),
				checkProxy("alias1", "cluster3"),
				checkProxy("alias2", "cluster3"),
				checkProxy("alias3", "cluster1"),
				checkProxy("alias4", "cluster2"),
				checkProxy("alias5", "cluster3"),
				inUsePortIs("cluster2", "8083"),
				inUsePortIs("cluster1", "8082"),
				inUsePortIs("cluster3", "8080", "8081", "8084"),
			},
		},
		{
			name: "add 3 cluster, rewrite all of cluster 1 aliases",
			actions: []action{
				addClusterData(
					"cluster3",
					[]string{"192.168.3.1", "192.168.3.2"},
					pair.MakePair("alias1", "8080"),
					pair.MakePair("alias2", "8081"),
					pair.MakePair("alias3", "8083"),
				),
				checkProxy("alias1", "cluster3"),
				checkProxy("alias2", "cluster3"),
				checkProxy("alias3", "cluster3"),
				checkProxy("alias4", "cluster2"),
				checkProxy("alias5", ""),
				inUsePortIs("cluster2", "8083"),
				inUsePortIs("cluster3", "8080", "8081", "8083"),
			},
		},
	}

	for _, tt := range tests {
		if !t.Run(tt.name, func(t *testing.T) {
			for _, a := range tt.actions {
				a(t, reconciler)
			}
		}) {
			t.FailNow()
		}
	}
}

func dropAlias(als string) action {
	return func(t *testing.T, r *workloadproxy.Reconciler) {
		removed := r.DropAlias(als)
		require.True(t, removed)
	}
}

func testPath2(t *testing.T, reconciler *workloadproxy.Reconciler) {
	tests := []struct {
		name    string
		actions []action
	}{
		{
			name: "init 11 cluster",
			actions: []action{
				addClusterData(
					"cluster11",
					[]string{"192.168.11.1", "192.168.11.2", "192.168.11.3"},
					pair.MakePair("alias11", "8080"),
					pair.MakePair("alias12", "8081"),
				),
				checkProxy("alias11", "cluster11"),
				checkProxy("alias12", "cluster11"),
				checkProxy("alias13", ""),
				inUsePortIs("cluster11", "8080", "8081"),
			},
		},
		{
			name: "init 12 cluster",
			actions: []action{
				addClusterData(
					"cluster12",
					[]string{"192.168.12.1", "192.168.12.2"},
					pair.MakePair("alias13", "8080"),
					pair.MakePair("alias14", "8081"),
					pair.MakePair("alias15", "8083"),
				),
				checkProxy("alias11", "cluster11"),
				checkProxy("alias12", "cluster11"),
				checkProxy("alias13", "cluster12"),
				checkProxy("alias14", "cluster12"),
				checkProxy("alias15", "cluster12"),
				inUsePortIs("cluster11", "8080", "8081"),
				inUsePortIs("cluster12", "8080", "8081", "8083"),
			},
		},
		{
			name: "remove 11 cluster",
			actions: []action{
				dropAlias("alias11"),
				dropAlias("alias12"),
				addClusterData("cluster11", nil),
				checkProxy("alias11", ""),
				checkProxy("alias12", ""),
				checkProxy("alias13", "cluster12"),
				checkProxy("alias14", "cluster12"),
				checkProxy("alias15", "cluster12"),
				inUsePortIs("cluster12", "8080", "8081", "8083"),
			},
		},
		{
			name: "add 11 cluster back",
			actions: []action{
				dropAlias("alias13"),
				addClusterData(
					"cluster11",
					[]string{"192.168.11.1", "192.168.11.2", "192.168.11.9"},
					pair.MakePair("alias11", "8080"),
					pair.MakePair("alias12", "8081"),
					pair.MakePair("alias13", "8082"),
				),
				checkProxy("alias11", "cluster11"),
				checkProxy("alias12", "cluster11"),
				checkProxy("alias13", "cluster11"),
				checkProxy("alias14", "cluster12"),
				checkProxy("alias15", "cluster12"),
				inUsePortIs("cluster12", "8081", "8083"),
				inUsePortIs("cluster11", "8080", "8081", "8082"),
			},
		},
		{
			name: "add 13 cluster, rewrite part of cluster 1 and part of cluster 2 aliases",
			actions: []action{
				dropAlias("alias11"),
				dropAlias("alias15"),
				addClusterData(
					"cluster13",
					[]string{"192.168.13.1", "192.168.13.2"},
					pair.MakePair("alias11", "8080"),
					pair.MakePair("alias15", "8081"),
				),
				checkProxy("alias11", "cluster13"),
				checkProxy("alias12", "cluster11"),
				checkProxy("alias13", "cluster11"),
				checkProxy("alias14", "cluster12"),
				checkProxy("alias15", "cluster13"),
				inUsePortIs("cluster12", "8081"),
				inUsePortIs("cluster11", "8081", "8082"),
				inUsePortIs("cluster13", "8080", "8081"),
			},
		},
	}

	for _, tt := range tests {
		if !t.Run(tt.name, func(t *testing.T) {
			for _, a := range tt.actions {
				a(t, reconciler)
			}
		}) {
			t.FailNow()
		}
	}
}

type action func(t *testing.T, r *workloadproxy.Reconciler)

func addClusterData(clusterID resource.ID, upstreams []string, aliasPorts ...pair.Pair[string, string]) action {
	d := makeData(upstreams, aliasPorts...)

	return func(t *testing.T, r *workloadproxy.Reconciler) {
		require.NoError(t, r.Reconcile(clusterID, d))
	}
}

func failToAddClusterData(clusterID resource.ID, upstreams []string, aliasPorts ...pair.Pair[string, string]) action {
	d := makeData(upstreams, aliasPorts...)

	return func(t *testing.T, r *workloadproxy.Reconciler) {
		require.Error(t, r.Reconcile(clusterID, d))
	}
}

func makeData(upstreams []string, aliasPorts ...pair.Pair[string, string]) *workloadproxy.ReconcileData {
	data := &workloadproxy.ReconcileData{
		AliasPort: make(map[string]string, len(aliasPorts)),
		Hosts:     upstreams,
	}

	for _, ap := range aliasPorts {
		data.AliasPort[ap.F1] = ap.F2
	}

	return data
}

func checkProxy(alias string, expectedClusterID resource.ID) action {
	return func(t *testing.T, r *workloadproxy.Reconciler) {
		proxy, id, err := r.GetProxy(alias)
		require.NoError(t, err)

		if expectedClusterID == "" {
			require.Nilf(t, proxy, "unexpected proxy for alias %q", alias)
			require.Zerof(t, id, "unexpected cluster id for alias %q", alias)

			return
		}

		require.NotNilf(t, proxy, "nil proxy for alias %q and cluster %q", alias, expectedClusterID)
		require.Equalf(t, expectedClusterID, id, "unexpected cluster id for alias %q, expected %q, got %q", alias, expectedClusterID, id)
	}
}

func inUsePortIs(cluserID resource.ID, ports ...string) action {
	return func(t *testing.T, r *workloadproxy.Reconciler) {
		for d := range r.All() {
			if d.ClusterID == cluserID {
				require.Containsf(t, ports, d.InUsePort, "unexpected in use port for cluster %q, expected one of %v, got %q", cluserID, ports, d.InUsePort)

				return
			}
		}

		t.Fatalf("no data for cluster %q", cluserID)
	}
}

func TestReconcilerDoubles(t *testing.T) {
	reconciler := workloadproxy.NewReconciler(zaptest.NewLogger(t), zapcore.InfoLevel)

	t.Cleanup(reconciler.Shutdown)

	for _, a := range []struct { //nolint:govet
		name   string
		action action
	}{
		{
			"init 1 cluster",
			addClusterData(
				"integration-workload-proxy",
				[]string{"fdae:41e4:649b:9303:940a:e06e:2d3b:4ba2"},
				pair.MakePair("2cfvf4", "12345"),
			),
		},
		{
			"init 1 cluster again",
			addClusterData(
				"integration-workload-proxy",
				[]string{"fdae:41e4:649b:9303:940a:e06e:2d3b:4ba2"},
				pair.MakePair("2cfvf4", "12345"),
			),
		},
		{
			"drop alias",
			dropAlias("2cfvf4"),
		},
		{
			"remove 1 cluster",
			addClusterData(
				"integration-workload-proxy",
				[]string{"fdae:41e4:649b:9303:940a:e06e:2d3b:4ba2"},
			),
		},
		{
			"remove 1 cluster again",
			addClusterData(
				"integration-workload-proxy",
				[]string{"fdae:41e4:649b:9303:940a:e06e:2d3b:4ba2"},
			),
		},
		{
			"ensure no proxy",
			checkProxy("2cfvf4", ""),
		},
	} {
		t.Run(a.name, func(t *testing.T) {
			a.action(t, reconciler)
		})
	}
}

func TestReconcilerPath(t *testing.T) {
	ch := make(chan string, 10)

	server1 := httptest.Server{
		Listener: must.Value(net.Listen("tcp", "127.0.0.1:8085"))(t),
		Config: &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			ch <- "got request in server1"
		})},
	}
	t.Cleanup(server1.Close)

	server1.Start()

	server2 := httptest.Server{
		Listener: must.Value(net.Listen("tcp", "[::1]:8085"))(t),
		Config: &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			ch <- "got request in server2"
		})},
	}
	t.Cleanup(server2.Close)

	server2.Start()

	reconciler := workloadproxy.NewReconciler(zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())), zapcore.InfoLevel)
	t.Cleanup(reconciler.Shutdown)

	h1, p1 := parseHost(t, "http://127.0.0.1:8085")
	h2, _ := parseHost(t, "http://[::1]:8085")
	hosts := []string{h1, h2}

	rand.Shuffle(len(hosts), func(i, j int) { hosts[i], hosts[j] = hosts[j], hosts[i] })

	err := reconciler.Reconcile("cluster1", &workloadproxy.ReconcileData{
		AliasPort: map[string]string{
			"alias1": p1,
		},
		Hosts: hosts,
	})

	require.NoError(t, err)

	proxy, id, err := reconciler.GetProxy("alias1")
	require.NoError(t, err)
	require.NotNil(t, proxy)
	require.Equal(t, "cluster1", id)

	req := must.Value(http.NewRequestWithContext(t.Context(), http.MethodGet, server1.URL, nil))(t)
	res := httptest.NewRecorder()

	for range cap(ch) {
		time.Sleep(time.Second)
		proxy.ServeHTTP(res, req)
	}

	for range cap(ch) {
		select {
		case got := <-ch:
			t.Log(got)
		case <-t.Context().Done():
			t.Fatal("timeout waiting for request")
		}
	}
}

func parseHost(t *testing.T, u string) (string, string) {
	parse := must.Value(url.Parse(u))(t)

	return must.Values(net.SplitHostPort(parse.Host))(t)
}
