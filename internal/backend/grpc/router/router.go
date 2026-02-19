// Copyright (c) 2026 Sidero Labs, Inc.
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

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
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
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/memconn"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/certs"
)

const (
	// Talos backend cache holds per-node gRPC proxy connections (one per cluster-node pair).
	// TTL is kept short to avoid holding many idle per-node connections.
	// Active invalidation via ResourceWatcher handles state-change evictions.

	talosBackendLRUSize = 1024
	talosBackendTTL     = 10 * time.Minute
)

// TalosAuditor is an interface for auditing Talos access.
type TalosAuditor interface {
	AuditTalosAccess(context.Context, string, string, string) error
}

// Router wraps grpc-proxy StreamDirector.
type Router struct {
	talosBackends *expirable.LRU[string, proxy.Backend]
	sf            singleflight.Group

	metricCacheSize     *prometheus.GaugeVec
	metricActiveClients *prometheus.GaugeVec
	metricCacheHits     *prometheus.CounterVec
	metricCacheMisses   *prometheus.CounterVec

	omniBackend  proxy.Backend
	nodeResolver NodeResolver
	verifier     grpc.UnaryServerInterceptor
	cosiState    state.State
	talosAuditor TalosAuditor
	authEnabled  bool
}

// NewRouter builds new Router.
func NewRouter(
	transport *memconn.Transport,
	cosiState state.State,
	nodeResolver NodeResolver,
	authEnabled bool,
	talosAuditor TalosAuditor,
	verifier grpc.UnaryServerInterceptor,
) (*Router, error) {
	omniConn, err := grpc.NewClient(transport.Address(),
		grpc.WithContextDialer(func(dctx context.Context, _ string) (net.Conn, error) {
			return transport.DialContext(dctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			// we are proxying requests to ourselves, so we don't need to impose a limit
			grpc.MaxCallRecvMsgSize(math.MaxInt32),
			grpc.ForceCodecV2(proxy.Codec()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial omni backend: %w", err)
	}

	typeLabel := []string{"type"}

	cacheSize := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "omni_grpc_proxy_talos_backend_cache_size",
		Help: "Number of Talos backends in the cache of gRPC Proxy.",
	}, typeLabel)

	r := &Router{
		metricCacheSize: cacheSize,
		metricActiveClients: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_grpc_proxy_talos_backend_active_clients",
			Help: "Number of active Talos backends created by gRPC Proxy.",
		}, typeLabel),
		metricCacheHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_grpc_proxy_talos_backend_cache_hits_total",
			Help: "Number of gRPC Proxy Talos backend cache hits.",
		}, typeLabel),
		metricCacheMisses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_grpc_proxy_talos_backend_cache_misses_total",
			Help: "Number of gRPC Proxy Talos backend cache misses.",
		}, typeLabel),
		omniBackend:  NewOmniBackend("omni", nodeResolver, omniConn),
		nodeResolver: nodeResolver,
		verifier:     verifier,
		cosiState:    cosiState,
		talosAuditor: talosAuditor,
		authEnabled:  authEnabled,
	}

	r.talosBackends = expirable.NewLRU[string, proxy.Backend](talosBackendLRUSize, func(key string, _ proxy.Backend) {
		cacheSize.WithLabelValues(cacheKeyType(key)).Dec()
	}, talosBackendTTL)

	return r, nil
}

// releaseForCluster evicts all cached backends for the given cluster (both cluster-scoped and per-node).
func (r *Router) releaseForCluster(clusterID string) {
	prefix := clusterID + "/"

	for _, key := range r.talosBackends.Keys() {
		if strings.HasPrefix(key, prefix) {
			r.talosBackends.Remove(key)
		}
	}
}

// releaseForMachine evicts a single cached backend by cluster and machine ID.
func (r *Router) releaseForMachine(clusterID, machineID string) {
	r.talosBackends.Remove(buildCacheKey(clusterID, machineID))
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

	if runtime := md.Get(message.RuntimeHeaderKey); runtime != nil && runtime[0] == common.Runtime_Talos.String() {
		backend, err := r.getTalosBackend(ctx, md)
		if err != nil {
			return proxy.One2One, nil, err
		}

		if err = r.talosAuditor.AuditTalosAccess(ctx, fullMethodName, getClusterName(md), getNodeID(md)); err != nil {
			return proxy.One2One, nil, err
		}

		return proxy.One2One, []proxy.Backend{backend}, nil
	}

	return proxy.One2One, []proxy.Backend{r.omniBackend}, nil
}

func (r *Router) getTalosBackend(ctx context.Context, md metadata.MD) (proxy.Backend, error) {
	clusterID := getClusterName(md)

	nodes, err := resolveNodes(r.nodeResolver, md)
	if err != nil {
		code := codes.InvalidArgument
		if errors.Is(err, dns.ErrNotFound) {
			code = codes.NotFound
		}

		return nil, status.Errorf(code, "resolve nodes: %v", err)
	}

	// No specific nodes targeted: return a cluster-scoped backend with round-robin across CP nodes.
	if len(nodes) == 0 {
		if clusterID == "" {
			return nil, status.Error(codes.InvalidArgument, "at least one of cluster, node, or nodes must be specified")
		}

		return r.getForCluster(ctx, clusterID)
	}

	// If cluster name was not provided via gRPC metadata, infer it from the resolved nodes.
	// They are checked earlier for being in the same cluster, so we can simply pick it from the first one.
	if clusterID == "" {
		clusterID = nodes[0].Cluster
	}

	// Multiple nodes: route through a CP node and preserve the "nodes" header
	// so that Talos apid handles the One2Many fan-out.
	// Omni cannot do this itself — One2Many merges multiple responses (including per-node errors)
	// into a single gRPC response, and some APIs explicitly reject One2Many calls.
	if len(nodes) > 1 {
		return r.getForCluster(ctx, clusterID)
	}

	// Single node: connect directly via SideroLink, strip all routing headers.
	return r.getForMachine(ctx, clusterID, nodes[0])
}

// getForCluster returns a backend that load-balances across all healthy CP nodes of the cluster.
func (r *Router) getForCluster(ctx context.Context, clusterID string) (proxy.Backend, error) {
	cacheKey := buildCacheKey(clusterID, "")
	typ := cacheKeyType(cacheKey)

	if backend, ok := r.talosBackends.Get(cacheKey); ok {
		r.metricCacheHits.WithLabelValues(typ).Inc()

		return backend, nil
	}

	ch := r.sf.DoChan(cacheKey, func() (any, error) {
		innerCtx := actor.MarkContextAsInternalActor(ctx)

		r.metricCacheMisses.WithLabelValues(typ).Inc()

		conn, err := r.getClusterConn(innerCtx, clusterID)
		if err != nil {
			return nil, err
		}

		activeGauge := r.metricActiveClients.WithLabelValues(typ)
		activeGauge.Inc()

		backend := NewTalosBackend(cacheKey, clusterID, r.nodeResolver, conn, r.authEnabled, r.verifier, r.cosiState)
		r.talosBackends.Add(cacheKey, backend)

		r.metricCacheSize.WithLabelValues(typ).Inc()

		runtime.AddCleanup(backend, func(m prometheus.Gauge) { m.Dec() }, activeGauge)
		runtime.AddCleanup(backend, func(conn *grpc.ClientConn) { conn.Close() }, backend.conn) //nolint:errcheck

		return backend, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		return res.Val.(proxy.Backend), nil //nolint:errcheck,forcetypeassert
	}
}

// getForMachine returns a backend connected directly to a specific node's SideroLink address.
func (r *Router) getForMachine(ctx context.Context, clusterID string, node dns.Info) (proxy.Backend, error) {
	if node.ManagementEndpoint == "" {
		return nil, status.Errorf(codes.Unavailable, "node %q has no management endpoint", node.Name)
	}

	cacheKey := buildCacheKey(clusterID, node.ID)
	typ := cacheKeyType(cacheKey)

	if backend, ok := r.talosBackends.Get(cacheKey); ok {
		r.metricCacheHits.WithLabelValues(typ).Inc()

		return backend, nil
	}

	ch := r.sf.DoChan(cacheKey, func() (any, error) {
		innerCtx := actor.MarkContextAsInternalActor(ctx)

		r.metricCacheMisses.WithLabelValues(typ).Inc()

		conn, err := r.getMachineConn(innerCtx, clusterID, node.ManagementEndpoint)
		if err != nil {
			return nil, err
		}

		activeGauge := r.metricActiveClients.WithLabelValues(typ)
		activeGauge.Inc()

		backend := NewTalosBackend(cacheKey, clusterID, r.nodeResolver, conn, r.authEnabled, r.verifier, r.cosiState)
		r.talosBackends.Add(cacheKey, backend)

		r.metricCacheSize.WithLabelValues(typ).Inc()

		runtime.AddCleanup(backend, func(m prometheus.Gauge) { m.Dec() }, activeGauge)
		runtime.AddCleanup(backend, func(conn *grpc.ClientConn) { conn.Close() }, backend.conn) //nolint:errcheck

		return backend, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		return res.Val.(proxy.Backend), nil //nolint:errcheck,forcetypeassert
	}
}

func buildCacheKey(clusterID, machineID string) string {
	if clusterID == "" {
		return "machine-" + machineID
	}

	return clusterID + "/" + machineID
}

// getClusterConn builds a gRPC connection that load-balances across all healthy CP nodes of a cluster.
// It reads the ClusterEndpoint resource to get the management addresses and uses gRPC round-robin.
func (r *Router) getClusterConn(ctx context.Context, clusterID string) (*grpc.ClientConn, error) {
	clusterEndpoint, err := safe.StateGet[*omni.ClusterEndpoint](ctx, r.cosiState,
		omni.NewClusterEndpoint(clusterID).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.Unavailable, "cluster %q endpoint not found", clusterID)
		}

		return nil, err
	}

	mgmtAddresses := clusterEndpoint.TypedSpec().Value.ManagementAddresses
	if len(mgmtAddresses) == 0 {
		return nil, status.Errorf(codes.Unavailable, "cluster %q has no management addresses", clusterID)
	}

	endpoints := make([]string, 0, len(mgmtAddresses))

	for _, addr := range mgmtAddresses {
		endpoints = append(endpoints, net.JoinHostPort(addr, strconv.FormatInt(talosconstants.ApidPort, 10)))
	}

	return r.dialTalos(ctx, clusterID, endpoints)
}

// getMachineConn builds a gRPC connection directly to a single machine's SideroLink management endpoint.
// For maintenance nodes (clusterID="") it uses insecure TLS (no cert verification).
// For nodes that belong to a cluster it uses the cluster's mTLS credentials.
func (r *Router) getMachineConn(ctx context.Context, clusterID, mgmtEndpoint string) (*grpc.ClientConn, error) {
	endpoint := net.JoinHostPort(mgmtEndpoint, strconv.FormatInt(talosconstants.ApidPort, 10))

	if clusterID == "" {
		return r.dialTalosMaintenance(endpoint)
	}

	return r.dialTalos(ctx, clusterID, []string{endpoint})
}

// dialTalos creates a gRPC connection to one or more Talos endpoints using cluster mTLS credentials.
// When multiple endpoints are provided, gRPC round-robin load balancing distributes calls across them.
func (r *Router) dialTalos(ctx context.Context, clusterID string, endpoints []string) (*grpc.ClientConn, error) {
	clusterCreds, err := r.getClusterCredentials(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		RootCAs: x509.NewCertPool(),
	}

	if ok := tlsConfig.RootCAs.AppendCertsFromPEM(clusterCreds.CAPEM); !ok {
		return nil, errors.New("failed to append CA certificate to RootCAs pool")
	}

	clientCert, err := tls.X509KeyPair(clusterCreds.CertPEM, clusterCreds.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS client certificate: %w", err)
	}

	tlsConfig.Certificates = append(tlsConfig.Certificates, clientCert)

	return r.dialTalosWithCreds(credentials.NewTLS(tlsConfig), endpoints)
}

// dialTalosMaintenance creates a gRPC connection with insecure TLS (no cert verification) for maintenance-mode nodes.
func (r *Router) dialTalosMaintenance(endpoint string) (*grpc.ClientConn, error) {
	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec

	return r.dialTalosWithCreds(creds, []string{endpoint})
}

func (r *Router) dialTalosWithCreds(creds credentials.TransportCredentials, endpoints []string) (*grpc.ClientConn, error) {
	target := fmt.Sprintf("%s:///%s",
		resolver.RoundRobinResolverScheme,
		strings.Join(endpoints, ","),
	)

	backoffConfig := backoff.DefaultConfig
	backoffConfig.MaxDelay = 15 * time.Second

	return grpc.NewClient(
		target,
		grpc.WithInitialWindowSize(65535*32),
		grpc.WithInitialConnWindowSize(65535*16),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoffConfig,
			MinConnectTimeout: 20 * time.Second,
		}),
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(proxy.Codec())),
		grpc.WithSharedWriteBuffer(true),
	)
}

type talosClusterCredentials struct {
	CAPEM   []byte
	CertPEM []byte
	KeyPEM  []byte
}

func (r *Router) getClusterCredentials(ctx context.Context, clusterID string) (*talosClusterCredentials, error) {
	if clusterID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "cluster name is not set")
	}

	secrets, err := safe.StateGet[*omni.ClusterSecrets](ctx, r.cosiState, omni.NewClusterSecrets(clusterID).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "cluster %q is not registered", clusterID)
		}

		return nil, err
	}

	// use the `os:impersonator` role here, set the required role directly in router.TalosBackend.GetConnection.
	clientCert, CA, err := certs.TalosAPIClientCertificateFromSecrets(secrets, constants.CertificateValidityTime, role.MakeSet(role.Impersonator))
	if err != nil {
		return nil, err
	}

	return &talosClusterCredentials{
		CAPEM:   CA,
		CertPEM: clientCert.Crt,
		KeyPEM:  clientCert.Key,
	}, nil
}

// ResourceWatcher watches the resource state and removes cached Talos API connections.
func (r *Router) ResourceWatcher(ctx context.Context, s state.State, logger *zap.Logger) error {
	events := make(chan state.Event)

	if err := s.WatchKind(ctx, omni.NewCluster("").Metadata(), events); err != nil {
		return fmt.Errorf("failed to watch Clusters: %w", err)
	}

	if err := s.WatchKind(ctx, omni.NewClusterSecrets("").Metadata(), events); err != nil {
		return fmt.Errorf("failed to watch ClusterSecrets: %w", err)
	}

	if err := s.WatchKind(ctx, omni.NewClusterEndpoint("").Metadata(), events); err != nil {
		return fmt.Errorf("failed to watch ClusterEndpoints: %w", err)
	}

	if err := s.WatchKind(ctx, omni.NewMachine("").Metadata(), events); err != nil {
		return fmt.Errorf("failed to watch Machines: %w", err)
	}

	if err := s.WatchKind(ctx, omni.NewClusterMachine("").Metadata(), events); err != nil {
		return fmt.Errorf("failed to watch ClusterMachines: %w", err)
	}

	for {
		var event state.Event

		select {
		case <-ctx.Done():
			return nil
		case event = <-events:
		}

		switch event.Type {
		case state.Bootstrapped, state.Noop:
			continue
		case state.Errored:
			return fmt.Errorf("talos backend resource watch failed: %w", event.Error)
		case state.Created, state.Updated, state.Destroyed: // handle below
		}

		switch event.Resource.Metadata().Type() {
		case omni.MachineType:
			if event.Type == state.Destroyed {
				machineID := event.Resource.Metadata().ID()
				r.releaseForMachine("", machineID)

				logger.Info("remove machine talos backend", zap.String("machine", machineID))
			}
		case omni.ClusterMachineType:
			r.handleClusterMachineEvent(logger, event)
		default: // Cluster, ClusterSecrets, or ClusterEndpoint changed: remove all backends for the cluster
			clusterID := event.Resource.Metadata().ID()
			r.releaseForCluster(clusterID)

			logger.Info("remove cluster talos backends", zap.String("cluster", clusterID))
		}
	}
}

func (r *Router) handleClusterMachineEvent(logger *zap.Logger, event state.Event) {
	machineID := event.Resource.Metadata().ID()

	switch event.Type { //nolint:exhaustive // event type is already filtered at call site
	case state.Created: // machine joined cluster: evict stale maintenance backend
		r.releaseForMachine("", machineID)

		logger.Info("remove maintenance talos backend", zap.String("machine", machineID))
	case state.Destroyed: // machine left cluster: evict its cluster backend
		clusterID, ok := event.Resource.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			logger.Warn("ClusterMachine has no cluster label, skipping backend cleanup", zap.String("machine", machineID))

			return
		}

		r.releaseForMachine(clusterID, machineID)

		logger.Info("remove node talos backend", zap.String("cluster", clusterID), zap.String("machine", machineID))
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
	r.metricCacheSize.Collect(ch)
	r.metricActiveClients.Collect(ch)
	r.metricCacheHits.Collect(ch)
	r.metricCacheMisses.Collect(ch)
}

var _ prometheus.Collector = &Router{}

// cacheKeyType returns the client type label for a cache key.
func cacheKeyType(key string) string {
	if strings.HasPrefix(key, "machine-") {
		return "maintenance"
	}

	if strings.HasSuffix(key, "/") {
		return "cluster"
	}

	return "machine"
}
