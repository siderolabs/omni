// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ClientNotReadyError is returned when building the client fails because cluster endpoints list is empty
// or Talos config resource doesn't exist.
type ClientNotReadyError struct {
	wrappedError error
}

// NewClientNotReadyError creates a new ClientNotReadyError wrapping the given error.
func NewClientNotReadyError(wrapped error) ClientNotReadyError {
	return ClientNotReadyError{wrappedError: wrapped}
}

func (e ClientNotReadyError) Error() string {
	return fmt.Sprintf("talos API client is not available: %s", e.wrappedError)
}

func (e ClientNotReadyError) Unwrap() error {
	return e.wrappedError
}

// IsClientNotReadyError checks if the error is ClientNotReadyError.
func IsClientNotReadyError(e error) bool {
	var w ClientNotReadyError

	return errors.As(e, &w)
}

// NewClient creates a new Talos client.
//
// clusterID is optional, and can be empty for maintenance clients. If it is set, the client will check cluster status to determine connectivity in Connected() method.
func NewClient(c *client.Client, clusterID, machineID string) *Client {
	return &Client{Client: c, clusterID: clusterID, machineID: machineID}
}

// Client wraps Talos client.
type Client struct {
	*client.Client

	clusterID string
	machineID string
}

// ClusterID returns the cluster ID of the client. Empty for maintenance clients.
func (c *Client) ClusterID() string {
	return c.clusterID
}

// MachineID returns the machine ID of the client. Empty for cluster-wide clients.
func (c *Client) MachineID() string {
	return c.machineID
}

// Close closes the Talos client.
//
// Deprecated: Clients are cached, so this is no-op and must not be called.
func (c *Client) Close() error {
	return nil
}

// Connected provides informational flag about cluster being reachable which is computed from the resources.
func (c *Client) Connected(ctx context.Context, r controller.Reader) (bool, error) {
	if c == nil {
		return false, errors.New("client is nil")
	}

	if c.clusterID == "" && c.machineID == "" {
		return false, errors.New("both clusterID and machineID are empty")
	}

	if len(c.GetEndpoints()) == 0 {
		return false, nil
	}

	if c.clusterID == "" { // this is a machine client, check machine connectivity
		machine, err := safe.ReaderGetByID[*omni.Machine](ctx, r, c.machineID)
		if err != nil {
			return false, fmt.Errorf("failed to get machine %q for Talos client: %w", c.machineID, err)
		}

		return machine.TypedSpec().Value.Connected, nil
	}

	// this is a cluster client, check cluster connectivity

	clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](ctx, r,
		omni.NewClusterStatus(c.clusterID).Metadata(),
	)
	if err != nil {
		return false, fmt.Errorf("failed to get cluster status for cluster %q: %w", c.clusterID, err)
	}

	return clusterStatus.TypedSpec().Value.GetAvailable(), nil
}

// GetSocketOptions adds unix socket parameters to the client configuration
// if the address has unix:// schema.
func GetSocketOptions(address string) []client.OptionFunc {
	// we are not going to use unix sockets for management,
	// but it's handy to have it when running unit tests
	if strings.HasPrefix(address, "unix://") {
		_, address, _ = strings.Cut(address, "//")

		return []client.OptionFunc{
			client.WithUnixSocket(address),
			client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
		}
	}

	return nil
}

const (
	// Talos client cache holds both cluster-wide and per-machine clients.
	// Cluster-wide clients are used by controllers (e.g. etcd machine audit).
	// Per-machine clients are created on demand for frontend resource requests.
	// TTL is kept short to avoid holding many idle per-machine connections.
	// Active invalidation via StartCacheManager handles state-change evictions.

	talosClientLRUSize = 1024
	talosClientTTL     = 10 * time.Minute
)

// ClientFactory creates client based on the resource state.
type ClientFactory struct {
	omniState state.State
	logger    *zap.Logger

	cache *expirable.LRU[string, *Client]
	sf    singleflight.Group

	metricCacheSize     *prometheus.GaugeVec
	metricActiveClients *prometheus.GaugeVec
	metricCacheHits     *prometheus.CounterVec
	metricCacheMisses   *prometheus.CounterVec
}

// NewClientFactory initializes a ClientFactory with a built-in cache.
// For the factory to do proper cache invalidation, the method StartCacheManager must be called.
func NewClientFactory(omniState state.State, logger *zap.Logger) *ClientFactory {
	typeLabel := []string{"type"}

	cacheSize := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "omni_talos_clientfactory_cache_size",
		Help: "Number of Talos clients in the cache of Talos client factory.",
	}, typeLabel)

	f := &ClientFactory{
		omniState:       omniState,
		logger:          logger,
		metricCacheSize: cacheSize,
		metricActiveClients: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_talos_clientfactory_active_clients",
			Help: "Number of active Talos clients created by Talos client factory.",
		}, typeLabel),
		metricCacheHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_talos_clientfactory_cache_hits_total",
			Help: "Number of Talos client factory cache hits.",
		}, typeLabel),
		metricCacheMisses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_talos_clientfactory_cache_misses_total",
			Help: "Number of Talos client factory cache misses.",
		}, typeLabel),
	}

	f.cache = expirable.NewLRU[string, *Client](talosClientLRUSize, func(key string, _ *Client) {
		cacheSize.WithLabelValues(cacheKeyType(key)).Dec()
	}, talosClientTTL)

	return f
}

// connectionOptions returns client configuration generated from the TalosConfig resource.
func (factory *ClientFactory) connectionOptions(ctx context.Context, id string, endpoints []string) ([]client.OptionFunc, error) {
	if len(endpoints) > 0 {
		opts := GetSocketOptions(endpoints[0])

		if opts != nil {
			return opts, nil
		}
	}

	res, err := safe.StateGet[*omni.TalosConfig](ctx, factory.omniState, resource.NewMetadata(resources.DefaultNamespace, omni.TalosConfigType, id, resource.VersionUndefined))
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, NewClientNotReadyError(err)
		}

		return nil, err
	}

	spec := res.TypedSpec().Value

	config := &clientconfig.Config{
		Context: id,
		Contexts: map[string]*clientconfig.Context{
			id: {
				Endpoints: endpoints,
				CA:        spec.Ca,
				Crt:       spec.Crt,
				Key:       spec.Key,
			},
		},
	}

	return []client.OptionFunc{
		client.WithConfig(config),
		client.WithGRPCDialOptions(
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(constants.GRPCMaxMessageSize),
			),
			grpc.WithSharedWriteBuffer(true),
		),
	}, nil
}

// GetForCluster constructs a client from resource configuration.
// Returned client is cached and must not be closed by the consumer.
func (factory *ClientFactory) GetForCluster(ctx context.Context, clusterID string) (*Client, error) {
	cacheKey := buildCacheKey(clusterID, "")
	typ := cacheKeyType(cacheKey)

	if cli, ok := factory.cache.Get(cacheKey); ok {
		factory.logger.Debug("cache hit, returning cached Talos client", zap.String("key", cacheKey))

		factory.metricCacheHits.WithLabelValues(typ).Inc()

		return cli, nil
	}

	ch := factory.sf.DoChan(cacheKey, func() (any, error) {
		factory.logger.Debug("cache miss, creating new Talos client", zap.String("key", cacheKey))

		factory.metricCacheMisses.WithLabelValues(typ).Inc()

		cli, err := factory.buildForCluster(ctx, clusterID)
		if err != nil {
			return nil, err
		}

		activeGauge := factory.metricActiveClients.WithLabelValues(typ)
		activeGauge.Inc()

		runtime.AddCleanup(cli, func(c *client.Client) {
			factory.logger.Debug("finalizing Talos client", zap.String("key", cacheKey))

			activeGauge.Dec()

			c.Close() //nolint:errcheck
		}, cli.Client)

		factory.cache.Add(cacheKey, cli)

		factory.metricCacheSize.WithLabelValues(typ).Inc()

		return cli, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		cli := res.Val.(*Client) //nolint:forcetypeassert,errcheck

		return cli, nil
	}
}

// releaseForCluster evicts all cached clients for the given cluster (cluster-wide and per-machine).
//
// The cluster-wide key ("clusterID/") is removed last to avoid a window where
// stale per-machine entries remain in the cache after the cluster key is gone.
func (factory *ClientFactory) releaseForCluster(clusterID string) {
	clusterKey := buildCacheKey(clusterID, "")

	for _, key := range factory.cache.Keys() {
		if key == clusterKey {
			continue // remove last
		}

		if !strings.HasPrefix(key, clusterKey) {
			continue
		}

		if factory.cache.Remove(key) {
			factory.logger.Debug("deleted Talos client from cache", zap.String("key", key))
		}
	}

	if factory.cache.Remove(clusterKey) {
		factory.logger.Debug("deleted Talos client from cache", zap.String("key", clusterKey))
	}
}

func (factory *ClientFactory) buildForCluster(ctx context.Context, clusterID string) (*Client, error) {
	clusterEndpoint, err := safe.StateGet[*omni.ClusterEndpoint](ctx, factory.omniState,
		omni.NewClusterEndpoint(clusterID).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, NewClientNotReadyError(err)
		}

		return nil, err
	}

	endpoints := clusterEndpoint.TypedSpec().Value.ManagementAddresses
	if len(endpoints) == 0 {
		return nil, NewClientNotReadyError(errors.New("no management addresses on cluster endpoint"))
	}

	options, err := factory.connectionOptions(ctx, clusterID, endpoints)
	if err != nil {
		return nil, err
	}

	c, err := client.New(ctx, options...)
	if err != nil {
		return nil, err
	}

	return NewClient(c, clusterID, ""), nil
}

// GetForMachine constructs a Talos client connected directly to a specific node's SideroLink address.
// It returns a maintenance or a regular client depending on whether the node is currently part of a cluster or not.
// Returned client is cached and must not be closed by the consumer.
func (factory *ClientFactory) GetForMachine(ctx context.Context, machineID string) (*Client, error) {
	clusterID, idErr := factory.lookupClusterID(ctx, machineID)
	if idErr != nil {
		return nil, idErr
	}

	cacheKey := buildCacheKey(clusterID, machineID)
	typ := cacheKeyType(cacheKey)

	if cli, ok := factory.cache.Get(cacheKey); ok {
		factory.logger.Debug("cache hit, returning cached Talos node client", zap.String("key", cacheKey))

		factory.metricCacheHits.WithLabelValues(typ).Inc()

		return cli, nil
	}

	ch := factory.sf.DoChan(cacheKey, func() (any, error) {
		factory.logger.Debug("cache miss, creating new Talos node client", zap.String("key", cacheKey))

		factory.metricCacheMisses.WithLabelValues(typ).Inc()

		cli, err := factory.buildForMachine(ctx, clusterID, machineID)
		if err != nil {
			return nil, err
		}

		activeGauge := factory.metricActiveClients.WithLabelValues(typ)
		activeGauge.Inc()

		runtime.AddCleanup(cli, func(c *client.Client) {
			factory.logger.Debug("finalizing Talos node client", zap.String("machine", machineID))

			activeGauge.Dec()

			c.Close() //nolint:errcheck
		}, cli.Client)

		factory.cache.Add(cacheKey, cli)

		factory.metricCacheSize.WithLabelValues(typ).Inc()

		return cli, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		cli := res.Val.(*Client) //nolint:forcetypeassert,errcheck

		return cli, nil
	}
}

// lookupClusterID returns the cluster name for the given machine, or an empty string if it's not in a cluster.
func (factory *ClientFactory) lookupClusterID(ctx context.Context, machineID string) (string, error) {
	clusterMachine, err := safe.StateGetByID[*omni.ClusterMachine](ctx, factory.omniState, machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get ClusterMachine for machine %q: %w", machineID, err)
	}

	cluster, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return "", fmt.Errorf("cluster machine %q has no cluster label", machineID)
	}

	return cluster, nil
}

// buildCacheKey constructs a cache key for a client based on cluster and machine IDs.
//
// If no machine is specified, this is a cluster-client, and its key will be "clusterID/".
// If a machine is specified, this is a machine client:
// - If the machine is part of a cluster, the key will be "clusterID/machineID".
// - If the machine is not part of any cluster (maintenance mode), the key will be "machine-machineID".
func buildCacheKey(clusterID, machineID string) string {
	if clusterID == "" {
		return "machine-" + machineID
	}

	return clusterID + "/" + machineID
}

func (factory *ClientFactory) buildForMachine(ctx context.Context, clusterID, machineID string) (*Client, error) {
	machineStatus, getErr := safe.StateGet[*omni.MachineStatus](ctx, factory.omniState,
		omni.NewMachineStatus(machineID).Metadata(),
	)
	if getErr != nil {
		if state.IsNotFoundError(getErr) {
			return nil, NewClientNotReadyError(getErr)
		}

		return nil, getErr
	}

	managementAddress := machineStatus.TypedSpec().Value.ManagementAddress
	if managementAddress == "" {
		return nil, NewClientNotReadyError(fmt.Errorf("no management address for machine %q", machineID))
	}

	if clusterID != "" {
		options, err := factory.connectionOptions(ctx, clusterID, []string{managementAddress})
		if err != nil {
			return nil, err
		}

		c, err := client.New(ctx, options...)
		if err != nil {
			return nil, err
		}

		return NewClient(c, clusterID, machineID), nil
	}

	// Maintenance mode: encrypted but no certificate verification.
	c, err := client.New(ctx,
		client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), //nolint:gosec
		client.WithEndpoints(managementAddress),
		client.WithGRPCDialOptions(
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(constants.GRPCMaxMessageSize)),
			grpc.WithSharedWriteBuffer(true),
		),
	)
	if err != nil {
		return nil, err
	}

	return NewClient(c, "", machineID), nil
}

func (factory *ClientFactory) releaseForMachine(clusterID, machineID string) {
	cacheKey := buildCacheKey(clusterID, machineID)

	factory.logger.Debug("deleting Talos machine client from cache", zap.String("key", cacheKey))

	factory.cache.Remove(cacheKey)
}

// StartCacheManager starts watching the relevant resources to do the client cache invalidation.
func (factory *ClientFactory) StartCacheManager(ctx context.Context) error {
	eventCh := make(chan state.Event)

	clusterEndpointMd := omni.NewClusterEndpoint("").Metadata()
	talosconfigMd := omni.NewTalosConfig("").Metadata()
	clusterMachineMd := omni.NewClusterMachine("").Metadata()

	err := factory.omniState.WatchKind(ctx, clusterEndpointMd, eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch ClusterEndpoints: %w", err)
	}

	err = factory.omniState.WatchKind(ctx, talosconfigMd, eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch TalosConfigs: %w", err)
	}

	err = factory.omniState.WatchKind(ctx, clusterMachineMd, eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch ClusterMachines: %w", err)
	}

	factory.logger.Debug("started Talos client cache manager")

	for {
		var event state.Event

		select {
		case <-ctx.Done():
			factory.logger.Debug("stopping Talos client cache manager")

			return nil
		case event = <-eventCh:
		}

		switch event.Type {
		case state.Bootstrapped, state.Noop: // do nothing
			continue
		case state.Errored:
			return fmt.Errorf("talos client cache manager received an error event: %w", event.Error)
		case state.Created, state.Updated, state.Destroyed: // handle below
		}

		if event.Resource.Metadata().Type() == omni.ClusterMachineType {
			factory.handleClusterMachineEvent(event)

			continue
		}

		// ClusterEndpoint or TalosConfig changed — invalidate the cluster with all its clients.
		clusterID := event.Resource.Metadata().ID()
		factory.releaseForCluster(clusterID)
	}
}

func (factory *ClientFactory) handleClusterMachineEvent(event state.Event) {
	switch event.Type { //nolint:exhaustive // event type is already filtered at call site
	case state.Created:
		factory.releaseForMachine("", event.Resource.Metadata().ID())
	case state.Destroyed:
		clusterID, ok := event.Resource.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			factory.logger.Error("cluster machine has no cluster label, cannot evict from cache", zap.String("id", event.Resource.Metadata().ID()))

			return
		}

		factory.releaseForMachine(clusterID, event.Resource.Metadata().ID())
	}
}

// Describe implements prom.Collector interface.
func (factory *ClientFactory) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(factory, ch)
}

// Collect implements prom.Collector interface.
func (factory *ClientFactory) Collect(ch chan<- prometheus.Metric) {
	factory.metricCacheSize.Collect(ch)
	factory.metricActiveClients.Collect(ch)
	factory.metricCacheHits.Collect(ch)
	factory.metricCacheMisses.Collect(ch)
}

var _ prometheus.Collector = &ClientFactory{}

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
