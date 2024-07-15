// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/singleflight"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/client-go/util/connrotation"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// multiplexer provides an http.RoundTripper which selects the cluster based on the request context.
type multiplexer struct {
	connectors *expirable.LRU[string, *clusterConnector]
	sf         singleflight.Group

	metricCacheSize                    prometheus.Gauge
	metricCacheHits, metricCacheMisses prometheus.Counter
}

const (
	k8sConnectorLRUSize = 128
	k8sConnectorTTL     = time.Hour
)

type clusterConnector struct {
	dialer    *connrotation.Dialer
	transport *http.Transport
	rt        http.RoundTripper
	apiHost   string
}

func newMultiplexer() *multiplexer {
	return &multiplexer{
		connectors: expirable.NewLRU[string, *clusterConnector](
			k8sConnectorLRUSize,
			func(_ string, connector *clusterConnector) {
				connector.transport.CloseIdleConnections()
				connector.dialer.CloseAll()
			},
			k8sConnectorTTL,
		),
		metricCacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_k8sproxy_cache_size",
			Help: "Number of Kubernetes proxy connections in the cache.",
		}),
		metricCacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_k8sproxy_cache_hits_total",
			Help: "Number of Kubebernetes proxy connection cache hits.",
		}),
		metricCacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_k8sproxy_cache_misses_total",
			Help: "Number of Kubernetes proxy connection cache misses.",
		}),
	}
}

// RoundTrip implements http.RoundTripper interface.
func (m *multiplexer) RoundTrip(req *http.Request) (*http.Response, error) {
	clusterNameVal, ok := ctxstore.Value[clusterContextKey](req.Context())
	if !ok {
		return nil, errors.New("cluster name not found in request context")
	}

	rt, err := m.getRT(req.Context(), clusterNameVal.ClusterName)
	if err != nil {
		return nil, err
	}

	return rt.RoundTrip(req)
}

func (m *multiplexer) getRT(ctx context.Context, clusterName string) (http.RoundTripper, error) {
	clusterInfo, err := m.getClusterConnector(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	return clusterInfo.rt, nil
}

func (m *multiplexer) removeClusterConnector(clusterName string) {
	m.connectors.Remove(clusterName)
	m.sf.Forget(clusterName)
}

func (m *multiplexer) getClusterConnector(ctx context.Context, clusterName string) (*clusterConnector, error) {
	if connector, ok := m.connectors.Get(clusterName); ok {
		// refresh the TTL
		m.connectors.Add(clusterName, connector)

		m.metricCacheHits.Inc()

		return connector, nil
	}

	ch := m.sf.DoChan(clusterName, func() (any, error) {
		type kubeRuntime interface {
			GetKubeconfig(ctx context.Context, cluster *common.Context) (*rest.Config, error)
		}

		k8s, err := runtime.LookupInterface[kubeRuntime](kubernetes.Name)
		if err != nil {
			return nil, err
		}

		restConfig, err := k8s.GetKubeconfig(ctx, &common.Context{Name: clusterName})
		if err != nil {
			return nil, fmt.Errorf("error getting kubeconfig: %w", err)
		}

		tlsConfig, err := rest.TLSConfigFor(restConfig)
		if err != nil {
			return nil, err
		}

		clientTransport := cleanhttp.DefaultPooledTransport()
		clientTransport.TLSClientConfig = tlsConfig

		dialer := connrotation.NewDialer(clientTransport.DialContext)
		clientTransport.DialContext = dialer.DialContext

		// disable HTTP/2:
		//  * request path is `kubectl` -> `nginx` -> Omni -> kube-apiserver
		//  * nginx does not support HTTP/2 for backend connections
		//  * we need to have same HTTP version all the way from `kubectl` to kube-apiserver
		//  * so nginx should disable HTTP/2 for the external connection, and we should same for Omni -> kube-apiserver
		clientTransport.ForceAttemptHTTP2 = false

		restTransportConfig, err := restConfig.TransportConfig()
		if err != nil {
			return nil, err
		}

		rt, err := transport.HTTPWrappersForConfig(restTransportConfig, clientTransport)
		if err != nil {
			return nil, err
		}

		u, err := url.Parse(restConfig.Host)
		if err != nil {
			return nil, err
		}

		connector := &clusterConnector{
			dialer:    dialer,
			transport: clientTransport,
			rt:        rt,
			apiHost:   u.Host,
		}

		m.metricCacheMisses.Inc()
		m.connectors.Add(clusterName, connector)

		return connector, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		return res.Val.(*clusterConnector), nil //nolint:forcetypeassert
	}
}

// Describe implements prom.Collector interface.
func (m *multiplexer) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(m, ch)
}

// Collect implements prom.Collector interface.
func (m *multiplexer) Collect(ch chan<- prometheus.Metric) {
	m.metricCacheSize.Set(float64(m.connectors.Len()))
	m.metricCacheSize.Collect(ch)

	m.metricCacheHits.Collect(ch)
	m.metricCacheMisses.Collect(ch)
}

var _ prometheus.Collector = &multiplexer{}
