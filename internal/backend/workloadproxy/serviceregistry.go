// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

type serviceEntry struct {
	clusterID resource.ID
	port      uint32
}

type clusterEntry struct {
	healthyTargetAddressSet map[string]struct{}
	enabled                 bool
}

// ServiceRegistry a registry to store Services that are exposed by the workload proxy.
type ServiceRegistry struct {
	state state.State

	logger *zap.Logger

	clusterIDToEntry    map[resource.ID]*clusterEntry
	aliasToServiceEntry map[string]*serviceEntry

	lock sync.Mutex
}

// NewServiceRegistry creates a new service registry. It needs to be started before use.
func NewServiceRegistry(state state.State, logger *zap.Logger) (*ServiceRegistry, error) {
	if state == nil {
		return nil, errors.New("state is nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &ServiceRegistry{
		state:               state,
		clusterIDToEntry:    make(map[resource.ID]*clusterEntry),
		aliasToServiceEntry: make(map[string]*serviceEntry),
		logger:              logger,
	}, nil
}

// Start starts the service registry.
func (s *ServiceRegistry) Start(ctx context.Context) error {
	if s.state == nil {
		return errors.New("state is nil")
	}

	ch := make(chan state.Event)

	if err := s.state.WatchKind(ctx, omni.NewCluster(resources.DefaultNamespace, "").Metadata(), ch, state.WithBootstrapContents(true)); err != nil {
		return fmt.Errorf("failed to watch clusters: %w", err)
	}

	if err := s.state.WatchKind(ctx, omni.NewClusterMachineStatus(resources.DefaultNamespace, "").Metadata(), ch, state.WithBootstrapContents(true)); err != nil {
		return fmt.Errorf("failed to watch cluster machine statuses: %w", err)
	}

	if err := s.state.WatchKind(ctx, omni.NewExposedService(resources.DefaultNamespace, "").Metadata(), ch, state.WithBootstrapContents(true)); err != nil {
		return fmt.Errorf("failed to watch exposed services: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				s.logger.Debug("stopping service registry")

				return nil
			}

			return fmt.Errorf("service registry context error: %w", ctx.Err())
		case ev := <-ch:
			if err := s.handleEvent(ev); err != nil {
				return fmt.Errorf("failed to handle resource: %w", err)
			}
		}
	}
}

func (s *ServiceRegistry) handleEvent(ev state.Event) error {
	deleted := false

	switch ev.Type {
	case state.Bootstrapped:
		return nil
	case state.Created, state.Updated:
		deleted = false
	case state.Destroyed:
		deleted = true
	case state.Errored:
		return fmt.Errorf("service registry received an error event: %w", ev.Error)
	}

	switch r := ev.Resource.(type) {
	case *omni.Cluster:
		s.handleCluster(r, deleted)
	case *omni.ClusterMachineStatus:
		s.handleClusterMachineStatus(r, deleted)
	case *omni.ExposedService:
		s.handleExposedService(r, deleted)
	default:
		s.logger.Warn(
			"service registry received an event with an unexpected resource type",
			zap.String("event_type", ev.Type.String()),
			zap.String("type", fmt.Sprintf("%T", r)),
		)
	}

	return nil
}

func (s *ServiceRegistry) handleClusterMachineStatus(res *omni.ClusterMachineStatus, deleted bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	clusterID, ok := res.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return
	}

	managementAddress := res.TypedSpec().Value.GetManagementAddress()
	if managementAddress == "" {
		return
	}

	cluster := s.getOrCreateClusterEntryNoLock(clusterID)

	if deleted || !res.TypedSpec().Value.GetReady() {
		delete(cluster.healthyTargetAddressSet, managementAddress)

		return
	}

	cluster.healthyTargetAddressSet[managementAddress] = struct{}{}
}

func (s *ServiceRegistry) handleCluster(res *omni.Cluster, deleted bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	clusterID := res.Metadata().ID()

	if deleted {
		delete(s.clusterIDToEntry, clusterID)

		return
	}

	cluster := s.getOrCreateClusterEntryNoLock(clusterID)

	cluster.enabled = res.TypedSpec().Value.GetFeatures().GetEnableWorkloadProxy()
}

func (s *ServiceRegistry) handleExposedService(res *omni.ExposedService, deleted bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	clusterID, ok := res.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		s.logger.Warn("exposed service is missing cluster label", zap.String("id", res.Metadata().ID()))

		return
	}

	alias, ok := res.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	if !ok {
		s.logger.Warn("exposed service is missing alias label", zap.String("id", res.Metadata().ID()))

		return
	}

	if deleted {
		delete(s.aliasToServiceEntry, alias)

		return
	}

	s.aliasToServiceEntry[alias] = &serviceEntry{
		clusterID: clusterID,
		port:      res.TypedSpec().Value.GetPort(),
	}
}

func (s *ServiceRegistry) getOrCreateClusterEntryNoLock(clusterID resource.ID) *clusterEntry {
	cluster, ok := s.clusterIDToEntry[clusterID]
	if !ok {
		cluster = &clusterEntry{
			healthyTargetAddressSet: make(map[string]struct{}),
		}

		s.clusterIDToEntry[clusterID] = cluster
	}

	return cluster
}

// GetProxy returns a proxy for the given cluster and the alias of the service.
func (s *ServiceRegistry) GetProxy(alias string) (http.Handler, resource.ID, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	service, ok := s.aliasToServiceEntry[alias]
	if !ok {
		return nil, "", nil
	}

	cluster, ok := s.clusterIDToEntry[service.clusterID]
	if !ok || !cluster.enabled {
		return nil, "", nil
	}

	if len(cluster.healthyTargetAddressSet) == 0 {
		return nil, "", errors.New("no healthy target addresses")
	}

	getRandomHealthyTargetAddress := func() string {
		i := rand.Intn(len(cluster.healthyTargetAddressSet))

		for key := range cluster.healthyTargetAddressSet {
			if i == 0 {
				return key
			}

			i--
		}

		return "" // unreachable
	}

	targetURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(getRandomHealthyTargetAddress(), strconv.Itoa(int(service.port))),
	}

	return httputil.NewSingleHostReverseProxy(targetURL), service.clusterID, nil
}
