// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
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
	"go.uber.org/zap"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	resourceregistry "github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/hooks"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/external"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/infraprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/migration"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/version"
)

// PersistentState keeps the state and the close function.
type PersistentState struct {
	State  state.CoreState
	Close  func() error
	errors <-chan error
}

// State wraps virtual and default cosi states.
type State struct {
	virtualState  *virtual.State
	defaultState  state.State
	auditWrap     *AuditWrap
	storeFactory  store.FactoryWithMetrics
	sqliteMetrics *sqlite.Metrics

	logger *zap.Logger

	defaultPersistentState   *PersistentState
	secondaryPersistentState *PersistentState
	secondaryStorageDB       *sqlitex.Pool
}

// Default returns the default state.
func (s *State) Default() state.State {
	return s.defaultState
}

// SecondaryStorageDB returns the secondary storage database.
func (s *State) SecondaryStorageDB() *sqlitex.Pool {
	return s.secondaryStorageDB
}

// SecondaryPersistentState returns the secondary persistent state.
func (s *State) SecondaryPersistentState() *PersistentState {
	return s.secondaryPersistentState
}

// SQLiteMetrics returns the SQLite metrics collector.
func (s *State) SQLiteMetrics() *sqlite.Metrics {
	return s.sqliteMetrics
}

// Virtual returns the virtual state.
func (s *State) Virtual() *virtual.State {
	return s.virtualState
}

// Auditor returns the state auditor.
func (s *State) Auditor() *AuditWrap {
	return s.auditWrap
}

// StoreFactory returns the etcd backup store factory.
func (s *State) StoreFactory() store.FactoryWithMetrics {
	return s.storeFactory
}

// RunAuditCleanup runs the audit cleanup.
func (s *State) RunAuditCleanup(ctx context.Context) error {
	return s.auditWrap.RunCleanup(ctx)
}

// DefaultCore returns the default core state.
func (s *State) DefaultCore() state.CoreState {
	return s.defaultPersistentState.State
}

// Close closes the state.
func (s *State) Close() error {
	var secondaryPersistentStateErr, defaultPersistentStateErr, secondaryStorageDBErr error

	if err := s.secondaryPersistentState.Close(); err != nil {
		secondaryPersistentStateErr = fmt.Errorf("failed to close secondary persistent state: %w", err)
	}

	if err := s.defaultPersistentState.Close(); err != nil {
		defaultPersistentStateErr = fmt.Errorf("failed to close default persistent state: %w", err)
	}

	if err := sqlite.CloseDB(s.secondaryStorageDB, 5*time.Second); err != nil {
		secondaryStorageDBErr = fmt.Errorf("failed to close secondary storage database: %w", err)
	}

	return errors.Join(secondaryPersistentStateErr, defaultPersistentStateErr, secondaryStorageDBErr)
}

// HandleErrors must be called by the Omni server to be able to handle state errors that happen asynchronously.
func (s *State) HandleErrors(ctx context.Context) error {
	select {
	case err := <-s.defaultPersistentState.errors:
		s.logger.Error("state failed", zap.Error(err))

		return err
	case <-ctx.Done():
	}

	return nil
}

// NewState creates a production Omni state.
func NewState(ctx context.Context, params *config.Params, logger *zap.Logger, metricsRegistry prometheus.Registerer) (*State, error) {
	logger.Info("using sqlite", zap.String("path", params.Storage.Sqlite.GetPath()))

	secondaryStorageDB, err := sqlite.OpenDB(params.Storage.Sqlite)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database for secondary storage: %w", err)
	}

	var defaultPersistentState *PersistentState

	switch params.Storage.Default.GetKind() {
	case config.StorageDefaultKindBoltdb:
		defaultPersistentState, err = newBoltPersistentState(params.Storage.Default.Boltdb.GetPath(), nil, false, logger)
	case config.StorageDefaultKindEtcd:
		defaultPersistentState, err = newEtcdPersistentState(ctx, params, logger)
	default:
		return nil, fmt.Errorf("unknown storage kind %q", params.Storage.Default.GetKind())
	}

	if err != nil {
		return nil, err
	}

	virtualState := virtual.NewState(state.WrapCore(defaultPersistentState.State), params.Services.Api.URL())

	secondaryPersistentState, err := newSQLitePersistentState(ctx, secondaryStorageDB, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite state for secondary storage: %w", err)
	}

	storeFactory, err := store.NewStoreFactory(params.EtcdBackup)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd backup store factory: %w", err)
	}

	namespacedState := newNamespacedState(
		defaultPersistentState.State,
		secondaryPersistentState.State,
		virtualState,
		logger,
		storeFactory,
	)

	measuredState := stateWithMetrics(namespacedState, metricsRegistry)
	defaultState := state.WrapCore(measuredState)

	if err = initResources(ctx, defaultState, logger, params.Account.GetName()); err != nil {
		return nil, err
	}

	sqliteMetrics := sqlite.NewMetrics(secondaryStorageDB, secondaryPersistentState.State, logger)
	metricsRegistry.MustRegister(sqliteMetrics)

	auditWrap, err := NewAuditWrap(ctx, defaultState, params, secondaryStorageDB, logger, sqliteMetrics.CleanupCallback(sqlite.SubsystemAuditLogs))
	if err != nil {
		return nil, err
	}

	defaultState = auditWrap.WrapState(defaultState)

	return &State{
		defaultState:  defaultState,
		virtualState:  virtualState,
		auditWrap:     auditWrap,
		storeFactory:  storeFactory,
		sqliteMetrics: sqliteMetrics,

		defaultPersistentState:   defaultPersistentState,
		secondaryPersistentState: secondaryPersistentState,
		secondaryStorageDB:       secondaryStorageDB,
	}, nil
}

func newNamespacedState(
	defaultCoreState state.CoreState,
	secondaryCoreState state.CoreState,
	virtualState *virtual.State,
	logger *zap.Logger,
	storeFactory store.Factory,
) *namespaced.State {
	buildEphemeralState := inmem.NewStateWithOptions(inmem.WithHistoryGap(20), inmem.WithHistoryMaxCapacity(10000))
	ephemeralState := buildEphemeralState(resources.EphemeralNamespace)
	metaEphemeralState := buildEphemeralState(meta.NamespaceName)
	infraProviderState := infraprovider.NewState(defaultCoreState, ephemeralState, logger.With(logging.Component("infraprovider_state")))

	return namespaced.NewState(func(ns resource.Namespace) state.CoreState {
		switch ns {
		case resources.VirtualNamespace:
			return virtualState
		case resources.MetricsNamespace:
			return secondaryCoreState
		case meta.NamespaceName:
			return metaEphemeralState
		case resources.EphemeralNamespace:
			return ephemeralState
		case resources.ExternalNamespace:
			return &external.State{
				CoreState:    defaultCoreState,
				StoreFactory: storeFactory,
				Logger:       logger,
			}
		case resources.InfraProviderNamespace, resources.InfraProviderEphemeralNamespace:
			return infraProviderState
		default:
			if strings.HasPrefix(ns, resources.InfraProviderSpecificNamespacePrefix) {
				return infraProviderState
			}

			return defaultCoreState
		}
	})
}

func initResources(ctx context.Context, resourceState state.State, logger *zap.Logger, accountName string) error {
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
		{resources.InfraProviderNamespace, "Infra providers persistent namespace"},
		{resources.InfraProviderEphemeralNamespace, "Infra providers ephemeral namespace"},
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

	sysVersion := system.NewSysVersion(system.SysVersionID)
	sysVersion.TypedSpec().Value.BackendVersion = version.Tag
	sysVersion.TypedSpec().Value.InstanceName = accountName
	sysVersion.TypedSpec().Value.BackendApiVersion = version.API

	if err := resourceState.Create(ctx, sysVersion); err != nil {
		return err
	}

	if _, err := migration.NewManager(resourceState, logger.With(logging.Component("migration"))).Run(ctx); err != nil {
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
func NewAuditWrap(ctx context.Context, resState state.State, params *config.Params, auditLogDB *sqlitex.Pool, logger *zap.Logger, onCleanup func(int)) (*AuditWrap, error) {
	if !params.Logs.Audit.GetEnabled() {
		logger.Info("audit log disabled")

		return &AuditWrap{state: resState}, nil
	}

	logger.Info("audit log enabled")

	var logOpts []audit.LogOption
	if onCleanup != nil {
		logOpts = append(logOpts, audit.WithCleanupCallback(onCleanup))
	}

	a, err := audit.NewLog(ctx, params.Logs.Audit, auditLogDB, logger, logOpts...)
	if err != nil {
		return nil, err
	}

	hooks.Init(a)

	return &AuditWrap{state: resState, log: a}, nil
}

// AuditWrap is builder/wrapper for creating logged access to Omni and Talos nodes.
type AuditWrap struct {
	state state.State
	log   *audit.Log
}

// Reader reads the audit log file by file, oldest to newest.
func (w *AuditWrap) Reader(ctx context.Context, start, end time.Time) (auditlog.Reader, error) {
	if w.log == nil {
		return nil, errors.New("audit log is disabled")
	}

	return w.log.Reader(ctx, start, end)
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

// TestStateBuilder returns an inmem state builder with history options suitable for testing.
// The history buffer prevents watch event loss when controller goroutines are slow under parallel test load.
func TestStateBuilder() namespaced.StateBuilder {
	build := inmem.NewStateWithOptions(
		inmem.WithHistoryMaxCapacity(1024),
		inmem.WithHistoryGap(20),
	)

	return func(ns resource.Namespace) state.CoreState {
		return build(ns)
	}
}

// NewTestState creates a new test state using the in-memory storage and no virtual one.
func NewTestState(logger *zap.Logger) (*State, error) {
	defaultPersistentState := &PersistentState{
		State: namespaced.NewState(TestStateBuilder()),
	}

	storeFactory, err := store.NewStoreFactory(config.EtcdBackup{})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd backup store factory: %w", err)
	}

	return &State{
		defaultState:           state.WrapCore(defaultPersistentState.State),
		defaultPersistentState: defaultPersistentState,
		storeFactory:           storeFactory,
		logger:                 logger,
	}, nil
}
