// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
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
func NewClient(c *client.Client, clusterName string) *Client {
	return &Client{Client: c, clusterName: clusterName}
}

// Client wraps Talos client.
type Client struct {
	*client.Client

	clusterName string
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

	if len(c.GetEndpoints()) == 0 {
		return false, nil
	}

	if c.clusterName == "" {
		return false, errors.New("cluster name is empty")
	}

	clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](ctx, r,
		omni.NewClusterStatus(resources.DefaultNamespace, c.clusterName).Metadata(),
	)
	if err != nil {
		return false, fmt.Errorf("failed to get cluster status for cluster %q: %w", c.GetClusterName(), err)
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
	// Talos client cache is used for customer access to the clusters and
	// for the some controllers which go through Talos controlplane (vs. connecting to the node directly),
	// e.g. etcd machine audit.
	//
	// No controllers at the moment of writing this comment hold reference to all cluster clients.
	talosClientLRUSize = 256
	talosClientTTL     = time.Hour
)

// ClientFactory creates client based on the resource state.
type ClientFactory struct {
	omniState state.State
	logger    *zap.Logger

	cache *expirable.LRU[string, *Client]
	sf    singleflight.Group

	metricCacheSize, metricActiveClients prometheus.Gauge
	metricCacheHits, metricCacheMisses   prometheus.Counter
}

// NewClientFactory initializes a ClientFactory with a built-in cache.
// For the factory to do proper cache invalidation, the method StartCacheManager must be called.
func NewClientFactory(omniState state.State, logger *zap.Logger) *ClientFactory {
	return &ClientFactory{
		omniState: omniState,
		logger:    logger,
		cache:     expirable.NewLRU[string, *Client](talosClientLRUSize, nil, talosClientTTL),

		metricCacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_talos_clientfactory_cache_size",
			Help: "Number of Talos clients in the cache of Talos client factory.",
		}),
		metricActiveClients: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_talos_clientfactory_active_clients",
			Help: "Number of active Talos clients created by Talos client factory.",
		}),
		metricCacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_talos_clientfactory_cache_hits_total",
			Help: "Number of Talos client factory cache hits.",
		}),
		metricCacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_talos_clientfactory_cache_misses_total",
			Help: "Number of Talos client factory cache misses.",
		}),
	}
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

// Get constructs a client from resource configuration.
// Returned client is cached and must not be closed by the consumer.
func (factory *ClientFactory) Get(ctx context.Context, clusterName string) (*Client, error) {
	if cli, ok := factory.cache.Get(clusterName); ok {
		factory.logger.Debug("cache hit, returning cached Talos client", zap.String("cluster", clusterName))

		factory.metricCacheHits.Inc()

		return cli, nil
	}

	ch := factory.sf.DoChan(clusterName, func() (any, error) {
		factory.logger.Debug("cache miss, creating new Talos client", zap.String("cluster", clusterName))

		factory.metricCacheMisses.Inc()

		cli, err := factory.build(ctx, clusterName)
		if err != nil {
			return nil, err
		}

		factory.metricActiveClients.Inc()

		runtime.AddCleanup(cli, func(c *client.Client) {
			factory.logger.Debug("finalizing Talos client", zap.String("cluster", clusterName))

			factory.metricActiveClients.Dec()

			c.Close() //nolint:errcheck
		}, cli.Client)

		factory.cache.Add(clusterName, cli)

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

// release releases the Talos cluster client from the client cache.
// The client will only be closed when it is garbage collected.
func (factory *ClientFactory) release(clusterName string) {
	factory.logger.Debug("deleting Talos client from cache", zap.String("cluster", clusterName), zap.Stack("stack"))

	factory.cache.Remove(clusterName)
}

func (factory *ClientFactory) build(ctx context.Context, clusterName string) (*Client, error) {
	clusterEndpoint, err := safe.StateGet[*omni.ClusterEndpoint](ctx, factory.omniState,
		omni.NewClusterEndpoint(resources.DefaultNamespace, clusterName).Metadata(),
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

	options, err := factory.connectionOptions(ctx, clusterName, endpoints)
	if err != nil {
		return nil, err
	}

	c, err := client.New(ctx, options...)
	if err != nil {
		return nil, err
	}

	return NewClient(c, clusterName), nil
}

// StartCacheManager starts watching the relevant resources to do the client cache invalidation.
func (factory *ClientFactory) StartCacheManager(ctx context.Context) error {
	eventCh := make(chan state.Event)
	clusterEndpointMd := omni.NewClusterEndpoint(resources.DefaultNamespace, "").Metadata()
	talosconfigMd := omni.NewTalosConfig(resources.DefaultNamespace, "").Metadata()

	err := factory.omniState.WatchKind(ctx, clusterEndpointMd, eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch ClusterEndpoints: %w", err)
	}

	err = factory.omniState.WatchKind(ctx, talosconfigMd, eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch TalosConfigs: %w", err)
	}

	factory.logger.Debug("started Talos client cache manager")

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				factory.logger.Debug("stopping Talos client cache manager")

				return nil
			}

			return fmt.Errorf("talos client cache manager context error: %w", ctx.Err())
		case ev := <-eventCh:
			if ev.Error != nil {
				return fmt.Errorf("talos client cache manager received an error event: %w", ev.Error)
			}

			if ev.Resource == nil {
				continue
			}

			factory.release(ev.Resource.Metadata().ID())
		}
	}
}

// Describe implements prom.Collector interface.
func (factory *ClientFactory) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(factory, ch)
}

// Collect implements prom.Collector interface.
func (factory *ClientFactory) Collect(ch chan<- prometheus.Metric) {
	factory.metricActiveClients.Collect(ch)

	factory.metricCacheSize.Set(float64(factory.cache.Len()))
	factory.metricCacheSize.Collect(ch)

	factory.metricCacheHits.Collect(ch)
	factory.metricCacheMisses.Collect(ch)
}

var _ prometheus.Collector = &ClientFactory{}
