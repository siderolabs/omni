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
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"go.uber.org/zap"
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

// NewClient wraps a Talos client which is not managed by a ClientFactory. Close closes the wrapped client.
//
// clusterID is optional, and can be empty for maintenance clients. If it is set, the client will check cluster status to determine connectivity in Connected() method.
func NewClient(c *client.Client, clusterID, machineID string) *Client {
	cli := &Client{Client: c, clusterID: clusterID, machineID: machineID}

	if c != nil {
		cli.release = c.Close
	}

	return cli
}

// Client represents a Talos client handed out by a ClientFactory.
//
// The clients of the same cluster or machine share one connection. It stays open as long as any of them is open or the
// factory holds it in its cache. Therefore, the caller must close the client when it is done with it. Nothing obtained
// through the client (e.g., a stream) must be used after that.
type Client struct {
	*client.Client

	release func() error

	clusterID string
	machineID string

	cleanup runtime.Cleanup
	once    sync.Once
}

// ClusterID returns the cluster ID of the client. Empty for maintenance clients.
func (c *Client) ClusterID() string {
	return c.clusterID
}

// MachineID returns the machine ID of the client. Empty for cluster-wide clients.
func (c *Client) MachineID() string {
	return c.machineID
}

// Close releases the client. Only the first call has an effect.
func (c *Client) Close() error {
	var err error

	c.once.Do(func() {
		if c.release == nil {
			return
		}

		// the client is released properly, so the leak guard is stopped. Stop requires the client to stay reachable across the call.
		c.cleanup.Stop()
		runtime.KeepAlive(c)

		err = c.release()
	})

	return err
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

	clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](
		ctx, r,
		omni.NewClusterStatus(c.clusterID).Metadata(),
	)
	if err != nil {
		return false, fmt.Errorf("failed to get cluster status for cluster %q: %w", c.clusterID, err)
	}

	return clusterStatus.TypedSpec().Value.GetAvailable(), nil
}

// NewMaintenanceClient opens an insecure Talos client to a machine's maintenance API.
func NewMaintenanceClient(ctx context.Context, address string) (*client.Client, error) {
	opts := GetSocketOptions(address)
	opts = append(
		opts,
		client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), //nolint:gosec
		client.WithEndpoints(address),
	)

	return client.New(ctx, opts...)
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
	// talosClientIdleTimeout is how long a cached client without open leases is kept.
	talosClientIdleTimeout = 10 * time.Minute

	// talosClientSweepInterval is how often the idle clients are evicted.
	talosClientSweepInterval = time.Minute
)

// entry represents a cached Talos client shared by all the leases handed out for its key.
//
// refs counts the open leases plus one for the cache while the entry is cached. The connection is closed when it drops
// to zero. idleSince is set when the last lease is closed. Both are guarded by the factory mutex.
type entry struct {
	client    *client.Client
	idleSince time.Time
	key       string
	clusterID string
	machineID string
	refs      int
}

// ClientFactory creates client based on the resource state.
//
// See Client for the lifetime of the returned clients.
type ClientFactory struct {
	omniState state.State
	logger    *zap.Logger

	entries map[string]*entry

	// started is closed by StartCacheManager once all its watches are registered.
	started chan struct{}

	metricCacheSize     *prometheus.GaugeVec
	metricActiveClients *prometheus.GaugeVec
	metricCacheHits     *prometheus.CounterVec
	metricCacheMisses   *prometheus.CounterVec

	mu sync.Mutex

	// stopped is set when the cache manager exits, no new clients are cached after that.
	stopped bool
}

// NewClientFactory initializes a ClientFactory with a built-in cache.
// For the factory to do proper cache invalidation, the method StartCacheManager must be called.
func NewClientFactory(omniState state.State, logger *zap.Logger) *ClientFactory {
	typeLabel := []string{"type"}

	return &ClientFactory{
		omniState: omniState,
		logger:    logger,
		entries:   map[string]*entry{},
		started:   make(chan struct{}),
		metricCacheSize: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_talos_clientfactory_cache_size",
			Help: "Number of Talos clients in the cache of Talos client factory.",
		}, typeLabel),
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
		),
	}, nil
}

// GetForCluster constructs a client from resource configuration.
//
// The returned client must be closed by the caller.
func (factory *ClientFactory) GetForCluster(ctx context.Context, clusterID string) (*Client, error) {
	cacheKey := buildCacheKey(clusterID, "")

	factory.mu.Lock()
	defer factory.mu.Unlock()

	if cli, ok := factory.leaseLocked(cacheKey); ok {
		return cli, nil
	}

	c, err := factory.buildForCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	return factory.publishLocked(cacheKey, clusterID, "", c)
}

// leaseLocked returns a new lease on the cached client for the key, if there is one. Must be called with the factory mutex held.
func (factory *ClientFactory) leaseLocked(cacheKey string) (*Client, bool) {
	e, ok := factory.entries[cacheKey]
	if !ok {
		return nil, false
	}

	factory.logger.Debug("cache hit, returning cached Talos client", zap.String("key", cacheKey))

	factory.metricCacheHits.WithLabelValues(cacheKeyType(cacheKey)).Inc()

	return factory.newLeaseLocked(e), true
}

// publishLocked caches a freshly built client and returns the first lease on it. Must be called with the factory mutex held.
//
// The client is built and published under the mutex on purpose. This serializes it with the cache invalidation: either
// the state reads of the build see the change already, or the invalidation runs after the publish and evicts the client.
// This way, a client built from a stale state never survives in the cache. Building a client does not dial, so the mutex
// is held only for the state reads. A slow state stalls the lookups and the releases alike, we accept that.
func (factory *ClientFactory) publishLocked(cacheKey, clusterID, machineID string, c *client.Client) (*Client, error) {
	if factory.stopped {
		c.Close() //nolint:errcheck

		return nil, errors.New("talos client factory is stopped")
	}

	factory.logger.Debug("cache miss, caching new Talos client", zap.String("key", cacheKey))

	typ := cacheKeyType(cacheKey)

	factory.metricCacheMisses.WithLabelValues(typ).Inc()
	factory.metricCacheSize.WithLabelValues(typ).Inc()
	factory.metricActiveClients.WithLabelValues(typ).Inc()

	e := &entry{
		client:    c,
		key:       cacheKey,
		clusterID: clusterID,
		machineID: machineID,
		refs:      1, // the reference held by the cache
	}

	factory.entries[cacheKey] = e

	return factory.newLeaseLocked(e), nil
}

// newLeaseLocked adds a reference to the entry and returns a client which releases it on Close. Must be called with the
// factory mutex held.
func (factory *ClientFactory) newLeaseLocked(e *entry) *Client {
	e.refs++
	e.idleSince = time.Time{}

	cli := &Client{
		Client:    e.client,
		clusterID: e.clusterID,
		machineID: e.machineID,
		release: func() error {
			factory.release(e)

			return nil
		},
	}

	// the leak guard reports a lease which is garbage collected without being closed. It never releases the lease itself,
	// as the connection might still be in use through a stream. Its argument must not reference the lease.
	cli.cleanup = runtime.AddCleanup(cli, factory.reportLeak, e.key)

	return cli
}

func (factory *ClientFactory) reportLeak(key string) {
	factory.logger.Warn("Talos client was garbage collected without being closed", zap.String("key", key))
}

// release drops one reference to the entry and closes the connection when it was the last one.
//
// It never touches the cache map, the entry might have been evicted already.
func (factory *ClientFactory) release(e *entry) {
	factory.mu.Lock()

	e.refs--
	refs := e.refs

	if refs == 1 {
		// only the cache holds the entry now
		e.idleSince = time.Now()
	}

	factory.mu.Unlock()

	if refs > 0 {
		return
	}

	factory.logger.Debug("closing Talos client", zap.String("key", e.key))

	factory.metricActiveClients.WithLabelValues(cacheKeyType(e.key)).Dec()

	if err := e.client.Close(); err != nil {
		factory.logger.Warn("failed to close Talos client", zap.String("key", e.key), zap.Error(err))
	}
}

// evict removes the matching entries from the cache and releases the cache reference on each of them.
// The predicate is called with the factory mutex held.
func (factory *ClientFactory) evict(match func(e *entry) bool) {
	var evicted []*entry

	factory.mu.Lock()

	for key, e := range factory.entries {
		if !match(e) {
			continue
		}

		delete(factory.entries, key)

		factory.metricCacheSize.WithLabelValues(cacheKeyType(key)).Dec()

		evicted = append(evicted, e)
	}

	factory.mu.Unlock()

	for _, e := range evicted {
		factory.logger.Debug("evicted Talos client from cache", zap.String("key", e.key))

		factory.release(e)
	}
}

// sweep evicts the cached clients which had no open leases for longer than the idle timeout.
func (factory *ClientFactory) sweep(now time.Time) {
	factory.evict(func(e *entry) bool {
		return e.refs == 1 && now.Sub(e.idleSince) >= talosClientIdleTimeout
	})
}

// stop evicts all the cached clients and prevents new ones from being cached.
func (factory *ClientFactory) stop() {
	factory.mu.Lock()
	factory.stopped = true
	factory.mu.Unlock()

	factory.evict(func(*entry) bool { return true })
}

// releaseForCluster evicts all cached clients for the given cluster (cluster-wide and per-machine).
func (factory *ClientFactory) releaseForCluster(clusterID string) {
	clusterKey := buildCacheKey(clusterID, "")

	factory.evict(func(e *entry) bool {
		return strings.HasPrefix(e.key, clusterKey)
	})
}

func (factory *ClientFactory) buildForCluster(ctx context.Context, clusterID string) (*client.Client, error) {
	clusterEndpoint, err := safe.StateGet[*omni.ClusterEndpoint](
		ctx, factory.omniState,
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

	return client.New(ctx, options...)
}

// GetForMachine constructs a Talos client connected directly to a specific node's SideroLink address.
// It returns a maintenance (insecure) or a regular (secure) client depending on whether the machine is currently in
// maintenance mode or not, as reported by its MachineStatus.
//
// The returned client must be closed by the caller.
func (factory *ClientFactory) GetForMachine(ctx context.Context, machineID string) (*Client, error) {
	return factory.getForMachine(ctx, machineID, false)
}

// GetMaintenance constructs a Talos client connected directly to a specific node's SideroLink address over the insecure
// maintenance connection.
//
// It determines the machine mode solely from its MachineStatus: if the machine status does not exist yet, or the machine
// is not in maintenance mode, it returns an error instead of a client. This way a caller acting on a machine it believes
// to be in maintenance mode (based on a possibly stale view) can never accidentally reconfigure an allocated machine that
// has already left maintenance.
//
// The returned client must be closed by the caller.
func (factory *ClientFactory) GetMaintenance(ctx context.Context, machineID string) (*Client, error) {
	return factory.getForMachine(ctx, machineID, true)
}

func (factory *ClientFactory) getForMachine(ctx context.Context, machineID string, maintenanceOnly bool) (*Client, error) {
	_, clusterID, err := factory.resolveMachine(ctx, machineID, maintenanceOnly)
	if err != nil {
		return nil, err
	}

	cacheKey := buildCacheKey(clusterID, machineID)

	factory.mu.Lock()
	defer factory.mu.Unlock()

	if cli, ok := factory.leaseLocked(cacheKey); ok {
		return cli, nil
	}

	// cache miss: the machine status is read again under the mutex, as the one read above might be stale (see publishLocked)
	machineStatus, clusterID, err := factory.resolveMachine(ctx, machineID, maintenanceOnly)
	if err != nil {
		return nil, err
	}

	cacheKey = buildCacheKey(clusterID, machineID)

	if cli, ok := factory.leaseLocked(cacheKey); ok {
		return cli, nil
	}

	c, err := factory.buildForMachine(ctx, clusterID, machineStatus)
	if err != nil {
		return nil, err
	}

	return factory.publishLocked(cacheKey, clusterID, machineID, c)
}

// resolveMachine reads the machine status and returns the cluster the machine is reachable through: its own cluster over
// the secure connection, or none in maintenance mode.
func (factory *ClientFactory) resolveMachine(ctx context.Context, machineID string, maintenanceOnly bool) (*omni.MachineStatus, string, error) {
	machineStatus, err := safe.StateGet[*omni.MachineStatus](
		ctx, factory.omniState,
		omni.NewMachineStatus(machineID).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, "", NewClientNotReadyError(err)
		}

		return nil, "", err
	}

	spec := machineStatus.TypedSpec().Value

	// when only a maintenance client was asked for, refuse to build (or return a cached) cluster client. checked before
	// touching the cache so a concurrently cached cluster client can never leak through either.
	if maintenanceOnly && !spec.Maintenance {
		return nil, "", fmt.Errorf("machine %q is not in maintenance mode", machineID)
	}

	// A machine in maintenance mode is reachable only over the insecure maintenance connection, even when already
	// allocated to a cluster. Otherwise it is reachable over its cluster's secure connection. A machine that is in
	// neither state has no reachable client, so report it as not ready rather than caching a doomed one.
	switch {
	case spec.Maintenance:
		return machineStatus, "", nil
	case spec.Cluster == "":
		return nil, "", NewClientNotReadyError(fmt.Errorf("machine %q is neither in maintenance mode nor allocated to a cluster", machineID))
	default:
		return machineStatus, spec.Cluster, nil
	}
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

func (factory *ClientFactory) buildForMachine(ctx context.Context, clusterID string, machineStatus *omni.MachineStatus) (*client.Client, error) {
	machineID := machineStatus.Metadata().ID()

	managementAddress := machineStatus.TypedSpec().Value.ManagementAddress
	if managementAddress == "" {
		return nil, NewClientNotReadyError(fmt.Errorf("no management address for machine %q", machineID))
	}

	if clusterID != "" {
		options, err := factory.connectionOptions(ctx, clusterID, []string{managementAddress})
		if err != nil {
			return nil, err
		}

		return client.New(ctx, options...)
	}

	// Maintenance mode: encrypted but no certificate verification.
	return client.New(
		ctx,
		client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), //nolint:gosec
		client.WithEndpoints(managementAddress),
		client.WithGRPCDialOptions(
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(constants.GRPCMaxMessageSize)),
		),
	)
}

func (factory *ClientFactory) releaseForMachine(clusterID, machineID string) {
	cacheKey := buildCacheKey(clusterID, machineID)

	factory.evict(func(e *entry) bool {
		return e.key == cacheKey
	})
}

// WaitForCacheStart blocks until StartCacheManager has registered all its watches, or the context is done.
//
// A caller can use it to be sure the cache manager is live and will observe subsequent state changes before relying on
// its cache invalidation.
func (factory *ClientFactory) WaitForCacheStart(ctx context.Context) error {
	select {
	case <-factory.started:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StartCacheManager watches the relevant resources to invalidate the client cache and evicts the idle clients periodically.
// When it returns, the cache is emptied and no new clients are cached.
func (factory *ClientFactory) StartCacheManager(ctx context.Context) error {
	defer factory.stop()

	eventCh := make(chan state.Event)

	clusterEndpointMd := omni.NewClusterEndpoint("").Metadata()
	talosconfigMd := omni.NewTalosConfig("").Metadata()
	machineStatusMd := omni.NewMachineStatus("").Metadata()

	err := factory.omniState.WatchKind(ctx, clusterEndpointMd, eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch ClusterEndpoints: %w", err)
	}

	err = factory.omniState.WatchKind(ctx, talosconfigMd, eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch TalosConfigs: %w", err)
	}

	err = factory.omniState.WatchKind(ctx, machineStatusMd, eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch MachineStatuses: %w", err)
	}

	factory.logger.Debug("started Talos client cache manager")

	close(factory.started)

	ticker := time.NewTicker(talosClientSweepInterval)
	defer ticker.Stop()

	for {
		var event state.Event

		select {
		case <-ctx.Done():
			factory.logger.Debug("stopping Talos client cache manager")

			return nil
		case now := <-ticker.C:
			factory.sweep(now)

			continue
		case event = <-eventCh:
		}

		switch event.Type {
		case state.Bootstrapped, state.Noop: // do nothing
			continue
		case state.Errored:
			return fmt.Errorf("talos client cache manager received an error event: %w", event.Error)
		case state.Created, state.Updated, state.Destroyed: // handle below
		}

		switch event.Resource.Metadata().Type() {
		case omni.MachineStatusType:
			factory.handleMachineStatusEvent(event)
		default:
			// ClusterEndpoint or TalosConfig changed, invalidate the cluster with all its clients.
			clusterID := event.Resource.Metadata().ID()
			factory.releaseForCluster(clusterID)
		}
	}
}

// handleMachineStatusEvent evicts the now-stale clients of a machine whose maintenance mode or cluster changed.
//
// A machine is reachable over exactly one client: the insecure maintenance client while in maintenance mode, or the
// secure cluster client otherwise. On every change the clients the machine is no longer reachable through are evicted,
// which is idempotent and never drops the currently valid one.
//
// The previous cluster is read from the old version of the resource carried by the event, so the secure cluster client
// can be evicted even when the machine status has already cleared its cluster field as the machine leaves the cluster.
//
// The management address is not watched, as it never changes for an existing machine: SideroLink keeps the address of an
// existing link, and a machine gets a new one only after its link and status were destroyed, which evicts its clients.
func (factory *ClientFactory) handleMachineStatusEvent(event state.Event) {
	machineID := event.Resource.Metadata().ID()

	machineStatus, ok := event.Resource.(*omni.MachineStatus)
	if !ok {
		factory.logger.Error("unexpected resource type for machine status event", zap.String("id", machineID))

		return
	}

	// evictCluster drops the secure cluster client for a non-empty cluster (an empty cluster would target the
	// maintenance client instead, which must never be evicted as a side effect here).
	evictCluster := func(clusterID string) {
		if clusterID != "" {
			factory.releaseForMachine(clusterID, machineID)
		}
	}

	if event.Type == state.Destroyed {
		// the machine is gone: drop both its maintenance and its secure cluster client.
		factory.releaseForMachine("", machineID)
		evictCluster(machineStatus.TypedSpec().Value.Cluster)

		return
	}

	// evict the secure client of the previous cluster if the machine moved to a different one or left it entirely.
	if old, ok := event.Old.(*omni.MachineStatus); ok {
		if oldCluster := old.TypedSpec().Value.Cluster; oldCluster != machineStatus.TypedSpec().Value.Cluster {
			evictCluster(oldCluster)
		}
	}

	if machineStatus.TypedSpec().Value.Maintenance {
		// the machine is in maintenance mode: its current secure cluster client is stale.
		evictCluster(machineStatus.TypedSpec().Value.Cluster)

		return
	}

	// the machine is not in maintenance mode: the insecure maintenance client is stale.
	factory.releaseForMachine("", machineID)
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
