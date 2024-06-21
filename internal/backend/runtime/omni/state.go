// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/prometheus/client_golang/prometheus"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	resourceregistry "github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/cloudprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/external"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/migration"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/version"
)

// NewState creates a production Omni state.
//
//nolint:cyclop
func NewState(ctx context.Context, params *config.Params, logger *zap.Logger, metricsRegistry prometheus.Registerer, f func(context.Context, state.State, *virtual.State) error) error {
	stateFunc := func(ctx context.Context, persistentStateBuilder namespaced.StateBuilder) error {
		primaryStorageCoreState := persistentStateBuilder(resources.DefaultNamespace)

		secondaryStorageCoreState, secondaryStorageBackingStore, err := newBoltPersistentState(
			params.SecondaryStorage.Path, &bbolt.Options{
				NoSync: true, // we do not need fsync for the secondary storage
			}, true, logger)
		if err != nil {
			return fmt.Errorf("failed to create BoltDB state for secondary storage: %w", err)
		}

		defer secondaryStorageBackingStore.Close() //nolint:errcheck

		virtualState := virtual.NewState(state.WrapCore(primaryStorageCoreState))

		storeFactory, err := store.NewStoreFactory()
		if err != nil {
			return fmt.Errorf("failed to create etcd backup store: %w", err)
		}

		cloudProviderState := cloudprovider.NewState(primaryStorageCoreState, logger.With(logging.Component("cloudprovider_state")))

		namespacedState := namespaced.NewState(func(ns resource.Namespace) state.CoreState {
			switch ns {
			case resources.VirtualNamespace:
				return virtualState
			case resources.MetricsNamespace:
				return secondaryStorageCoreState
			case meta.NamespaceName, resources.EphemeralNamespace:
				return inmem.NewStateWithOptions(inmem.WithHistoryGap(20))(ns)
			case resources.ExternalNamespace:
				return &external.State{
					CoreState:    primaryStorageCoreState,
					StoreFactory: storeFactory,
					Logger:       logger,
				}
			case resources.CloudProviderNamespace:
				return cloudProviderState
			default:
				if strings.HasPrefix(ns, resources.CloudProviderSpecificNamespacePrefix) {
					return cloudProviderState
				}

				return primaryStorageCoreState
			}
		})

		measuredState := wrapStateWithMetrics(namespacedState)

		metricsRegistry.MustRegister(measuredState)

		resourceState := state.WrapCore(measuredState)

		namespaceRegistry := registry.NewNamespaceRegistry(resourceState)
		resourceRegistry := registry.NewResourceRegistry(resourceState)

		if err := namespaceRegistry.RegisterDefault(ctx); err != nil {
			return err
		}

		if err := resourceRegistry.RegisterDefault(ctx); err != nil {
			return err
		}

		// register Omni namespaces
		for _, ns := range []struct {
			name        string
			description string
		}{
			{resources.DefaultNamespace, "Default namespace for resources"},
			{resources.EphemeralNamespace, "Ephemeral namespace for resources"},
			{resources.VirtualNamespace, "Namespace for virtual resources"},
			{resources.ExternalNamespace, "Namespace for external resources"},
			{resources.MetricsNamespace, "Secondary storage namespace for resources"},
		} {
			if err := namespaceRegistry.Register(ctx, ns.name, ns.description); err != nil {
				return err
			}
		}

		// register Omni resources
		for _, r := range resourceregistry.Resources {
			if err := resourceRegistry.Register(ctx, r); err != nil {
				return err
			}
		}

		sysVersion := system.NewSysVersion(resources.EphemeralNamespace, system.SysVersionID)
		sysVersion.TypedSpec().Value.BackendVersion = version.Tag
		sysVersion.TypedSpec().Value.InstanceName = config.Config.Name
		sysVersion.TypedSpec().Value.BackendApiVersion = version.API

		if err := resourceState.Create(ctx, sysVersion); err != nil {
			return err
		}

		migrationsManager := migration.NewManager(resourceState, logger.With(logging.Component("migration")))
		if err := migrationsManager.Run(ctx); err != nil {
			return err
		}

		return f(ctx, resourceState, virtualState)
	}

	switch params.Storage.Kind {
	case "boltdb":
		return buildBoltPersistentState(ctx, params.Storage.Boltdb.Path, logger, stateFunc)
	case "etcd":
		return buildEtcdPersistentState(ctx, params, logger, stateFunc)
	default:
		return fmt.Errorf("unknown storage kind %q", params.Storage.Kind)
	}
}
