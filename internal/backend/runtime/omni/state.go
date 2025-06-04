// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/hooks"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/external"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/infraprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/migration"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/version"
)

// NewState creates a production Omni state.
func NewState(ctx context.Context, params *config.Params, logger *zap.Logger, metricsRegistry prometheus.Registerer, f func(context.Context, state.State, *virtual.State) error) error {
	stateFunc := func(ctx context.Context, persistentStateBuilder namespaced.StateBuilder) error {
		primaryStorageCoreState := persistentStateBuilder(resources.DefaultNamespace)
		virtualState := virtual.NewState(state.WrapCore(primaryStorageCoreState))

		namespacedState, closer, err := newNamespacedState(params, primaryStorageCoreState, virtualState, logger)
		if err != nil {
			return err
		}

		defer closer()

		measuredState := stateWithMetrics(namespacedState, metricsRegistry)
		resourceState := state.WrapCore(measuredState)

		if err = initResources(ctx, resourceState, logger); err != nil {
			return err
		}

		return f(
			ctx,
			resourceState,
			virtualState,
		)
	}

	switch params.Storage.Default.Kind {
	case "boltdb":
		return buildBoltPersistentState(ctx, params.Storage.Default.Boltdb.Path, logger, stateFunc)
	case "etcd":
		return buildEtcdPersistentState(ctx, params, logger, stateFunc)
	default:
		return fmt.Errorf("unknown storage kind %q", params.Storage.Default.Kind)
	}
}

func newNamespacedState(params *config.Params, primaryStorageCoreState state.CoreState, virtualState *virtual.State, logger *zap.Logger) (*namespaced.State, func(), error) {
	secondaryStorageCoreState, secondaryStorageBackingStore, err := newBoltPersistentState(
		params.Storage.Secondary.Path, &bbolt.Options{
			NoSync: true, // we do not need fsync for the secondary storage
		}, true, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create BoltDB state for secondary storage: %w", err)
	}

	storeFactory, err := store.NewStoreFactory()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create etcd backup store: %w", err)
	}

	buildEphemeralState := inmem.NewStateWithOptions(inmem.WithHistoryGap(20), inmem.WithHistoryMaxCapacity(10000))
	ephemeralState := buildEphemeralState(resources.EphemeralNamespace)
	metaEphemeralState := buildEphemeralState(meta.NamespaceName)
	infraProviderState := infraprovider.NewState(primaryStorageCoreState, ephemeralState, logger.With(logging.Component("infraprovider_state")))

	namespacedState := namespaced.NewState(func(ns resource.Namespace) state.CoreState {
		switch ns {
		case resources.VirtualNamespace:
			return virtualState
		case resources.MetricsNamespace:
			return secondaryStorageCoreState
		case meta.NamespaceName:
			return metaEphemeralState
		case resources.EphemeralNamespace:
			return ephemeralState
		case resources.ExternalNamespace:
			return &external.State{
				CoreState:    primaryStorageCoreState,
				StoreFactory: storeFactory,
				Logger:       logger,
			}
		case resources.InfraProviderNamespace, resources.InfraProviderEphemeralNamespace:
			return infraProviderState
		default:
			if strings.HasPrefix(ns, resources.InfraProviderSpecificNamespacePrefix) {
				return infraProviderState
			}

			return primaryStorageCoreState
		}
	})

	return namespacedState, func() {
		if err := secondaryStorageBackingStore.Close(); err != nil {
			logger.Warn("failed to close secondary storage backing store", zap.Error(err))
		}
	}, nil
}

func initResources(ctx context.Context, resourceState state.State, logger *zap.Logger) error {
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
	sysVersion.TypedSpec().Value.InstanceName = config.Config.Account.Name
	sysVersion.TypedSpec().Value.BackendApiVersion = version.API

	if err := resourceState.Create(ctx, sysVersion); err != nil {
		return err
	}

	if err := migration.NewManager(resourceState, logger.With(logging.Component("migration"))).Run(ctx); err != nil {
		return err
	}

	return nil
}

func stateWithMetrics(namespacedState *namespaced.State, metricsRegistry prometheus.Registerer) *stateMetrics {
	measuredState := wrapStateWithMetrics(namespacedState)

	metricsRegistry.MustRegister(measuredState)

	return measuredState
}

// NewAuditWrap creates a new audit wrap.
func NewAuditWrap(resState state.State, params *config.Params, logger *zap.Logger) (*AuditWrap, error) {
	if params.Logs.Audit.Path == "" {
		logger.Info("audit log disabled")

		return &AuditWrap{state: resState}, nil
	}

	logger.Info("audit log enabled", zap.String("dir", params.Logs.Audit.Path))

	a, err := audit.NewLog(params.Logs.Audit.Path, logger)
	if err != nil {
		return nil, err
	}

	hooks.Init(a)

	return &AuditWrap{state: resState, log: a, dir: params.Logs.Audit.Path}, nil
}

// AuditWrap is builder/wrapper for creating logged access to Omni and Talos nodes.
type AuditWrap struct {
	state state.State
	log   *audit.Log
	dir   string
}

// ReadAuditLog reads the audit log file by file, oldest to newest.
func (w *AuditWrap) ReadAuditLog(start, end time.Time) (io.ReadCloser, error) {
	if w.log == nil {
		return nil, errors.New("audit log is disabled")
	}

	return w.log.ReadAuditLog(start, end)
}

// RunCleanup runs wrapped [audit.Log.RunCleanup] if the audit log is enabled. Otherwise, blocks until context is
// canceled.
func (w *AuditWrap) RunCleanup(ctx context.Context) error {
	if w.log == nil {
		<-ctx.Done()

		return nil
	}

	return w.log.RunCleanup(ctx)
}

// Wrap implements [k8sproxy.MiddlewareWrapper].
func (w *AuditWrap) Wrap(handler http.Handler) http.Handler {
	if w.log == nil {
		return handler
	}

	return w.log.Wrap(handler)
}

// AuditTalosAccess logs a Talos access event. It does nothing if the audit log is disabled.
func (w *AuditWrap) AuditTalosAccess(ctx context.Context, fullMethodName, clusterID, nodeID string) error {
	if w.log == nil {
		return nil
	}

	return w.log.AuditTalosAccess(ctx, fullMethodName, clusterID, nodeID)
}

// WrapState wraps the state with audit logging. It does nothing if the audit log is disabled.
func (w *AuditWrap) WrapState(resourceState state.State) state.State {
	if w.log == nil {
		return resourceState
	}

	return audit.WrapState(resourceState, w.log)
}
