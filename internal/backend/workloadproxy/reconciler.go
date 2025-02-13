// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"fmt"
	"iter"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/jellydator/ttlcache/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/omni/client/pkg/panichandler"
)

// Reconciler reconciles the load balancers for a list of clusters.
//
// Two data strcutures are introduced to esnure internal consistency. Those will be explained below.
// For now, it's important to know two things:
// 1. The Reconciler will automatically WILL NOT delete the cluster which doesn't have any alias pointing to it.
// 2. The Reconciler will automatically delete all aliases to the cluster if the cluster was requisted to be removed.
// 3. The Reconciler will automatically update probe upstream/port or cluster alias changes.
//
// For now, we use external ttlcache which supports callbacks on eviction. This is used to shut down the active load
// balancer probe. The expiration time is set to 5 minutes, and reset each time the cluster is accessed directly or via
// an alias.
//
// It starts goroutine to manage the cache, and it will be stopped when the Reconciler is GC'd.
//
//nolint:govet
type Reconciler struct {
	logger    *zap.Logger
	logLevel  zapcore.Level
	transport http.RoundTripper

	mu                   sync.Mutex
	shutdown             func()
	activeProbes         *ttlcache.Cache[resource.ID, remoteList]
	aliasToCluster       aliasToCluster
	connectionLatency    prometheus.Histogram
	requestStartLatency  prometheus.Histogram
	responseStartLatency prometheus.Histogram
	inFlightRequests     prometheus.Gauge
	workingProbes        prometheus.GaugeFunc
}

// NewReconciler creates a new Reconciler.
func NewReconciler(logger *zap.Logger, logLevel zapcore.Level) *Reconciler {
	if logger == nil {
		logger = zap.NewNop()
	}

	activeProbes := ttlcache.New[resource.ID, remoteList](
		ttlcache.WithTTL[resource.ID, remoteList](5 * time.Minute),
	)
	stopped := make(chan struct{})

	panichandler.Go(func() { defer close(stopped); activeProbes.Start() }, logger)

	waitForEvicts := activeProbes.OnEviction(func(
		_ context.Context,
		_ ttlcache.EvictionReason,
		i *ttlcache.Item[resource.ID, remoteList],
	) {
		logger.Log(logLevel, "shutting down active probe LB", zap.String("cluster", i.Key()))

		i.Value().Shutdown()
	})

	connectionLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "omni_workload_proxy_connection_latency",
		Help:    "Latency of the connection to the upstream.",
		Buckets: []float64{.05, .1, .5, 1, 5, 10},
	})
	responseStartLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "omni_workload_proxy_response_start_latency",
		Help:    "Latency of the response start from the upstream.",
		Buckets: []float64{.1, .5, 1, 3, 5, 10},
	})
	requestStartLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "omni_workload_proxy_request_start_latency",
		Help:    "Latency of the request start to the upstream.",
		Buckets: []float64{.05, .1, .5, 1, 5, 10},
	})
	inFlightRequests := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "omni_workload_proxy_inflight_requests",
		Help: "Number of active requests in the workload proxy.",
	})
	workingProbes := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "omni_workload_proxy_active_probes",
		Help: "Number of active probes in the workload proxy.",
	}, func() float64 { return float64(activeProbes.Len()) })

	var rec Reconciler

	rt := promhttp.InstrumentRoundTripperInFlight(
		inFlightRequests,
		promhttp.InstrumentRoundTripperTrace(
			&promhttp.InstrumentTrace{
				GotConn:              connectionLatency.Observe,
				GotFirstResponseByte: responseStartLatency.Observe,
				WroteHeaders:         requestStartLatency.Observe,
			},
			buildTransportDialer(&rec),
		),
	)

	cleanup := runtime.AddCleanup(
		&rec,
		func(t *ttlcache.Cache[resource.ID, remoteList]) { t.Stop(); t.DeleteAll() },
		activeProbes,
	)

	rec = Reconciler{
		logger:    logger,
		logLevel:  logLevel,
		transport: rt,
		shutdown: func() {
			cleanup.Stop()
			activeProbes.Stop()
			activeProbes.DeleteAll()
			waitForEvicts()
			<-stopped
		},
		activeProbes: activeProbes,
		aliasToCluster: aliasToCluster{
			aliases:  map[alias]aliasData{},
			clusters: map[resource.ID]*clusterData{},
		},
		connectionLatency:    connectionLatency,
		requestStartLatency:  requestStartLatency,
		responseStartLatency: responseStartLatency,
		inFlightRequests:     inFlightRequests,
		workingProbes:        workingProbes,
	}

	return &rec
}

// Reconcile reconciles the workload proxy load balancers for a cluster. If the ReconcileData is nil or doesn't contain
// any aliases, the cluster will be removed from the load balancer.
func (rec *Reconciler) Reconcile(clusterID resource.ID, rd *ReconcileData) error {
	logger := rec.logger.With(zap.String("cluster", clusterID), zap.Strings("upstreams", rd.GetHosts()))

	rec.mu.Lock()
	defer rec.mu.Unlock()

	err := rec.aliasToCluster.ReplaceCluster(clusterID, rd)
	if err != nil {
		return fmt.Errorf("failed to insert/remove cluster %q: %w", clusterID, err)
	}

	if rd == nil || len(rd.AliasPort) == 0 {
		_, ok := rec.activeProbes.GetAndDelete(clusterID)
		if ok {
			rec.logger.Info("removed cluster from active probes")
		}

		return nil
	}

	aliases := slices.Collect(rd.AliasesData())
	ports := xslices.Map(aliases, rd.PortForAlias)

	logger = logger.With(zap.Strings("aliases", aliases), zap.Strings("ports", ports))

	logger.Info("reconciled cluster in LB")

	h, p := rec.aliasToCluster.ActiveHostsPort(clusterID)
	if p == "" {
		rec.activeProbes.Delete(clusterID)

		return nil
	}

	l := rec.logger.With(
		zap.String("cluster", clusterID),
		zap.Strings("upstreams", rd.GetHosts()),
		zap.Stringer("probe_port", p),
	)

	_, _, err = rec.setAndGetActiveProbes(clusterID, h, p, l, true)
	if err != nil {
		return fmt.Errorf("failed to get and set active probes for cluster %q: %w", clusterID, err)
	}

	return nil
}

func (rec *Reconciler) setAndGetActiveProbes(clusterID resource.ID, h []string, p port, l *zap.Logger, needReconcile bool) (remoteList, bool, error) {
	reconciled := false

	got := rec.activeProbes.Get(clusterID)
	if got == nil {
		l, listErr := upstream.NewListWithCmp(asRemotes(h, p, l), (*remote).Equal)
		if listErr != nil {
			return nil, false, fmt.Errorf("failed to create upstream list for cluster %q: %w", clusterID, listErr)
		}

		got = rec.activeProbes.Set(clusterID, l, ttlcache.DefaultTTL)

		needReconcile = false
		reconciled = true
	}

	if needReconcile {
		got.Value().Reconcile(asRemotes(h, p, l))

		reconciled = true
	}

	return got.Value(), reconciled, nil
}

// DropAlias drops the alias from the load balancer. If the alias doesn't exist, the function does nothing.
func (rec *Reconciler) DropAlias(als string) bool {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	clData := rec.aliasToCluster.DropAlias(alias(als))
	if clData == nil {
		return false
	}

	clData.aliases = slices.DeleteFunc(clData.aliases, func(a alias) bool { return a == alias(als) })

	got := rec.activeProbes.Get(clData.clusterID)
	if got == nil {
		return true
	}

	hosts, p := rec.aliasToCluster.ActiveHostsPort(clData.clusterID)
	if p == "" {
		rec.activeProbes.Delete(clData.clusterID)

		return true
	}

	got.Value().Reconcile(asRemotes(hosts, p, rec.logger.With(
		zap.String("cluster", clData.clusterID),
		zap.Strings("upstreams", clData.hosts),
		zap.Stringer("probe_port", p),
	)))

	return true
}

// GetProxy returns a proxy for the exposed service, targeting the load balancer for the given alias.
func (rec *Reconciler) GetProxy(als string) (http.Handler, resource.ID, error) {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	c, p, ok := rec.aliasToCluster.ClusterPort(alias(als))
	logger := rec.logger.With(zap.String("cluster", c), zap.String("alias", als), zap.Stringer("port", p))

	logger.Debug("got proxy")

	if !ok {
		return nil, "", nil
	}

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: net.JoinHostPort(c, p.String())})
	proxy.Transport = rec.transport
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		logger.Error(
			"proxy error",
			zap.Error(err),
			zap.String("cluster", c),
			zap.String("alias", als),
			zap.String("alias_host", req.Host),
		)

		http.Error(w, "workload proxy error", http.StatusBadGateway)
	}

	return proxy, c, nil
}

// All returns all the info about the load balancers. More info in the [Description] struct.
func (rec *Reconciler) All() iter.Seq[Description] {
	return func(yield func(Description) bool) {
		rec.mu.Lock()
		defer rec.mu.Unlock()

		for alsPort, data := range rec.aliasToCluster.All() {
			if !yield(Description{
				Alias:     alsPort.F1.String(),
				ClusterID: data.clusterID,
				Port:      alsPort.F2.String(),
				InUsePort: data.inUsePort.String(),
				Upstream:  data.hosts,
			}) {
				return
			}
		}
	}
}

// Description descibes an alias, its cluster, port and attached upstreams. It also shows if the current port is used
// active probing.
type Description struct {
	Alias     string
	ClusterID string
	Port      string
	InUsePort string
	Upstream  []string
}

func buildTransportDialer(rec *Reconciler) http.RoundTripper {
	d := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := cleanhttp.DefaultPooledTransport()

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		rec.mu.Lock()
		defer rec.mu.Unlock()

		clusterID, rawPort, splitErr := net.SplitHostPort(addr)
		if splitErr != nil {
			return nil, fmt.Errorf("invalid proxy dst address: %w", splitErr)
		}

		p := port(rawPort)

		ad := rec.aliasToCluster.ClusterData(clusterID)
		if ad == nil {
			return nil, fmt.Errorf("cluster %q not found", clusterID)
		}

		got, reconciled, err := rec.setAndGetActiveProbes(clusterID, ad.hosts, p, rec.logger.With(
			zap.String("cluster", clusterID),
			zap.Strings("upstreams", ad.hosts),
			zap.Stringer("probe_port", p),
		), ad.inUsePort == "")
		if err != nil {
			return nil, fmt.Errorf("failed to set and get active probes for cluster %q: %w", clusterID, err)
		}

		if reconciled {
			if err = rec.aliasToCluster.SetActivePort(clusterID, p); err != nil {
				return nil, fmt.Errorf("failed to set active lb port %q: %w", p, err)
			}
		}

		switch rem, err := got.Pick(); {
		case err != nil:
			return nil, fmt.Errorf("failed to pick upstream for cluster %q: %w", clusterID, err)
		case rem == nil:
			return nil, fmt.Errorf("failed to pick upstream for cluster %q", clusterID)
		default:
			conn, connErr := d.DialContext(ctx, network, net.JoinHostPort(rem.Addr, p.String()))
			if connErr != nil {
				return nil, connErr
			}

			return conn, nil
		}
	}

	return transport
}

// Describe implements the [prometheus.Collector] interface.
func (rec *Reconciler) Describe(ch chan<- *prometheus.Desc) { prometheus.DescribeByCollect(rec, ch) }

// Collect implements the [prometheus.Collector] interface.
func (rec *Reconciler) Collect(mcs chan<- prometheus.Metric) {
	rec.workingProbes.Collect(mcs)
}

func asRemotes(hosts []string, p port, l *zap.Logger) iter.Seq[*remote] {
	return func(yield func(*remote) bool) {
		for _, h := range hosts {
			if !yield(&remote{Logger: l, Addr: h, AddrPort: net.JoinHostPort(h, string(p))}) {
				return
			}
		}
	}
}

// Shutdown stops the Reconciler and all the active probes.
func (rec *Reconciler) Shutdown() {
	rec.shutdown()
}

type remoteList = *upstream.List[*remote]

// Alias is an alias for the cluster specific port.
type alias string

// String implements the Stringer interface.
func (a alias) String() string { return string(a) }

type port string

func (p port) String() string { return string(p) }
