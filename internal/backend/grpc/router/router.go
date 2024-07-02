// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package router defines gRPC proxy helpers.
package router

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math"
	"net"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/siderolabs/grpc-proxy/proxy"
	"github.com/siderolabs/talos/pkg/machinery/client/resolver"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/memconn"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/certs"
)

const (
	talosBackendLRUSize = 32
	talosBackendTTL     = time.Hour
)

// Router wraps grpc-proxy StreamDirector.
type Router struct {
	talosBackends                        *expirable.LRU[string, proxy.Backend]
	sf                                   singleflight.Group
	metricCacheSize, metricActiveClients prometheus.Gauge
	metricCacheHits, metricCacheMisses   prometheus.Counter

	omniBackend  proxy.Backend
	nodeResolver NodeResolver
	verifier     grpc.UnaryServerInterceptor
	cosiState    state.State
	authEnabled  bool
}

// NewRouter builds new Router.
func NewRouter(
	transport *memconn.Transport,
	cosiState state.State,
	nodeResolver NodeResolver,
	authEnabled bool,
	verifier grpc.UnaryServerInterceptor,
) (*Router, error) {
	omniConn, err := grpc.NewClient(transport.Address(),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return transport.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithCodec(proxy.Codec()), //nolint:staticcheck
		// we are proxying requests to ourselves, so we don't need to impose a limit
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial omni backend: %w", err)
	}

	r := &Router{
		talosBackends: expirable.NewLRU[string, proxy.Backend](talosBackendLRUSize, nil, talosBackendTTL),
		omniBackend:   NewOmniBackend("omni", nodeResolver, omniConn),
		cosiState:     cosiState,
		nodeResolver:  nodeResolver,
		authEnabled:   authEnabled,
		verifier:      verifier,

		metricCacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_grpc_proxy_talos_backend_cache_size",
			Help: "Number of Talos clients in the cache of gRPC Proxy.",
		}),
		metricActiveClients: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_grpc_proxy_talos_backend_active_clients",
			Help: "Number of active Talos clients created by gRPC Proxy.",
		}),
		metricCacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_grpc_proxy_talos_backend_cache_hits_total",
			Help: "Number of gRPC Proxy Talos client cache hits.",
		}),
		metricCacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_grpc_proxy_talos_backend_cache_misses_total",
			Help: "Number of gRPC Proxy Talos client cache misses.",
		}),
	}

	return r, nil
}

// removeBackend clears cached client for a cluster.
func (r *Router) removeBackend(id string) {
	r.talosBackends.Remove(id)
}

// Director implements proxy.StreamDirector function.
func (r *Router) Director(ctx context.Context, fullMethodName string) (proxy.Mode, []proxy.Backend, error) {
	fullMethodName = strings.TrimLeft(fullMethodName, "/")

	// Proxy explicitly local APIs to the local backend.
	switch {
	case strings.HasPrefix(fullMethodName, "auth."),
		strings.HasPrefix(fullMethodName, "config."),
		strings.HasPrefix(fullMethodName, "management."),
		strings.HasPrefix(fullMethodName, "oidc."),
		strings.HasPrefix(fullMethodName, "omni."):
		return proxy.One2One, []proxy.Backend{r.omniBackend}, nil
	default:
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return proxy.One2One, []proxy.Backend{r.omniBackend}, nil
	}

	if runtime := md.Get(message.RuntimeHeaderHey); runtime != nil && runtime[0] == common.Runtime_Talos.String() {
		backends, err := r.getTalosBackend(ctx, md)
		if err != nil {
			return proxy.One2One, nil, err
		}

		return proxy.One2One, backends, nil
	}

	return proxy.One2One, []proxy.Backend{r.omniBackend}, nil
}

func (r *Router) getTalosBackend(ctx context.Context, md metadata.MD) ([]proxy.Backend, error) {
	clusterName := getClusterName(md)

	id := fmt.Sprintf("cluster-%s", clusterName)

	if clusterName == "" {
		id = fmt.Sprintf("machine-%s", getNodeID(md))
	}

	if backend, ok := r.talosBackends.Get(id); ok {
		r.metricCacheHits.Inc()

		return []proxy.Backend{backend}, nil
	}

	ch := r.sf.DoChan(id, func() (any, error) {
		ctx = actor.MarkContextAsInternalActor(ctx)

		r.metricCacheMisses.Inc()

		conn, err := r.getConn(ctx, clusterName)
		if err != nil {
			return nil, err
		}

		r.metricActiveClients.Inc()

		backend := NewTalosBackend(id, clusterName, r.nodeResolver, conn, r.authEnabled, r.verifier)
		r.talosBackends.Add(id, backend)

		runtime.SetFinalizer(backend, func(backend *TalosBackend) {
			r.metricActiveClients.Dec()

			backend.conn.Close() //nolint:errcheck
		})

		return backend, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		backend := res.Val.(proxy.Backend) //nolint:errcheck,forcetypeassert

		return []proxy.Backend{backend}, nil
	}
}

func (r *Router) getTransportCredentials(ctx context.Context, contextName string) (credentials.TransportCredentials, []string, error) {
	if contextName == "" {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, nil, fmt.Errorf("failed to get node ip from the request")
		}

		var endpoints []dns.Info

		info := resolveNodes(r.nodeResolver, md)

		endpoints = info.nodes

		if info.nodeOk {
			endpoints = []dns.Info{info.node}
		}

		return credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true,
			}), xslices.Map(endpoints, func(info dns.Info) string {
				return net.JoinHostPort(info.GetAddress(), strconv.FormatInt(talosconstants.ApidPort, 10))
			}), nil
	}

	clusterCredentials, err := r.getClusterCredentials(ctx, contextName)
	if err != nil {
		return nil, nil, err
	}

	tlsConfig := &tls.Config{}

	tlsConfig.RootCAs = x509.NewCertPool()

	if ok := tlsConfig.RootCAs.AppendCertsFromPEM(clusterCredentials.CAPEM); !ok {
		return nil, nil, errors.New("failed to append CA certificate to RootCAs pool")
	}

	clientCert, err := tls.X509KeyPair(clusterCredentials.CertPEM, clusterCredentials.KeyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create TLS client certificate: %w", err)
	}

	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.Certificates = append(tlsConfig.Certificates, clientCert)

	return credentials.NewTLS(tlsConfig), xslices.Map(clusterCredentials.Endpoints, func(endpoint string) string {
		return net.JoinHostPort(endpoint, strconv.FormatInt(talosconstants.ApidPort, 10))
	}), nil
}

func (r *Router) getConn(ctx context.Context, contextName string) (*grpc.ClientConn, error) {
	creds, endpoints, err := r.getTransportCredentials(ctx, contextName)
	if err != nil {
		return nil, err
	}

	backoffConfig := backoff.DefaultConfig
	backoffConfig.MaxDelay = 15 * time.Second

	endpoint := fmt.Sprintf("%s:///%s", resolver.RoundRobinResolverScheme, strings.Join(endpoints, ","))

	opts := []grpc.DialOption{
		grpc.WithInitialWindowSize(65535 * 32),
		grpc.WithInitialConnWindowSize(65535 * 16),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoffConfig,
			MinConnectTimeout: 20 * time.Second,
		}),
		grpc.WithTransportCredentials(creds),
		grpc.WithCodec(proxy.Codec()), //nolint:staticcheck
		grpc.WithSharedWriteBuffer(true),
	}

	return grpc.NewClient(
		endpoint,
		opts...,
	)
}

type talosClusterCredentials struct {
	CAPEM     []byte
	CertPEM   []byte
	KeyPEM    []byte
	Endpoints []string
}

func (r *Router) getClusterCredentials(ctx context.Context, clusterName string) (*talosClusterCredentials, error) {
	if clusterName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "cluster name is not set")
	}

	secrets, err := safe.StateGet[*omni.ClusterSecrets](ctx, r.cosiState, omni.NewClusterSecrets(resources.DefaultNamespace, clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "cluster %q is not registered", clusterName)
		}

		return nil, err
	}

	// use the `os:impersonator` role here, set the required role directly in router.TalosBackend.GetConnection.
	clientCert, CA, err := certs.TalosAPIClientCertificateFromSecrets(secrets, constants.CertificateValidityTime, role.MakeSet(role.Impersonator))
	if err != nil {
		return nil, err
	}

	clusterEndpoint, err := safe.StateGet[*omni.ClusterEndpoint](ctx, r.cosiState, omni.NewClusterEndpoint(resources.DefaultNamespace, clusterName).Metadata())
	if err != nil {
		return nil, err
	}

	endpoints := clusterEndpoint.TypedSpec().Value.ManagementAddresses

	return &talosClusterCredentials{
		Endpoints: endpoints,
		CAPEM:     CA,
		CertPEM:   clientCert.Crt,
		KeyPEM:    clientCert.Key,
	}, nil
}

// ResourceWatcher watches the resource state and removes cached Talos API connections.
func (r *Router) ResourceWatcher(ctx context.Context, s state.State, logger *zap.Logger) error {
	events := make(chan state.Event)

	if err := s.WatchKind(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, "", resource.VersionUndefined), events); err != nil {
		return err
	}

	if err := s.WatchKind(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterSecretsType, "", resource.VersionUndefined), events); err != nil {
		return err
	}

	if err := s.WatchKind(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterEndpointType, "", resource.VersionUndefined), events); err != nil {
		return err
	}

	if err := s.WatchKind(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.MachineType, "", resource.VersionUndefined), events); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-events:
			switch e.Type {
			case state.Errored:
				return fmt.Errorf("talos backend cluster watch failed: %w", e.Error)
			case state.Bootstrapped:
				// ignore
			case state.Created, state.Updated, state.Destroyed:
				if e.Resource.Metadata().Type() == omni.MachineType {
					if e.Type == state.Destroyed {
						id := fmt.Sprintf("machine-%s", e.Resource.Metadata().ID())
						r.removeBackend(id)

						logger.Info("remove machine talos backend", zap.String("id", id))
					}

					continue
				}

				id := fmt.Sprintf("cluster-%s", e.Resource.Metadata().ID())

				// all resources have cluster name as the ID, drop the backend to make sure we have new connection established
				r.removeBackend(id)

				logger.Info("remove cluster talos backend", zap.String("id", id))
			}
		}
	}
}

// ExtractContext reads cluster context from the supplied metadata.
func ExtractContext(ctx context.Context) *common.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}

	return &common.Context{Name: getClusterName(md)}
}

func getClusterName(md metadata.MD) string {
	get := func(key string) string {
		vals := md.Get(key)
		if vals == nil {
			return ""
		}

		return vals[0]
	}

	if clusterName := get(message.ClusterHeaderKey); clusterName != "" {
		return clusterName
	}

	return get(message.ContextHeaderKey)
}

func getNodeID(md metadata.MD) string {
	if nodes := md.Get(nodesHeaderKey); len(nodes) != 0 {
		slices.Sort(nodes)

		return strings.Join(nodes, ",")
	}

	return strings.Join(md.Get(nodeHeaderKey), ",")
}

// Describe implements prom.Collector interface.
func (r *Router) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(r, ch)
}

// Collect implements prom.Collector interface.
func (r *Router) Collect(ch chan<- prometheus.Metric) {
	r.metricActiveClients.Collect(ch)

	r.metricCacheSize.Set(float64(r.talosBackends.Len()))
	r.metricCacheSize.Collect(ch)

	r.metricCacheHits.Collect(ch)
	r.metricCacheMisses.Collect(ch)
}

var _ prometheus.Collector = &Router{}
