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

// PersistentState keeps the state and the close function.
type PersistentState struct {
	State  state.CoreState
	Close  func() error
	errors <-chan error
}

// State wraps virtual and default cosi states.
type State struct {
	virtualState *virtual.State
	defaultState state.State
	auditWrap    *AuditWrap

	logger *zap.Logger

	defaultPersistentState   *PersistentState
	secondaryPersistentState *PersistentState
}

// Default returns the default state.
func (s *State) Default() state.State {
	return s.defaultState
}

// Virtual returns the virtual state.
func (s *State) Virtual() *virtual.State {
	return s.virtualState
}

// Auditor returns the state auditor.
func (s *State) Auditor() *AuditWrap {
	return s.auditWrap
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
	if err := s.secondaryPersistentState.Close(); err != nil {
		s.logger.Warn("failed to close secondary storage backing store", zap.Error(err))
	}

	return s.defaultPersistentState.Close()
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
	var (
		defaultPersistentState *PersistentState
		err                    error
	)

	switch params.Storage.Default.Kind {
	case "boltdb":
		defaultPersistentState, err = newBoltPersistentState(params.Storage.Default.Boltdb.Path, nil, false, logger)
	case "etcd":
		defaultPersistentState, err = newEtcdPersistentState(ctx, params, logger)
	default:
		return nil, fmt.Errorf("unknown storage kind %q", params.Storage.Default.Kind)
	}

	if err != nil {
		return nil, err
	}

	virtualState := virtual.NewState(state.WrapCore(defaultPersistentState.State))

	secondaryPersistentState, err := newSQLitePersistentState(
		ctx, params.Storage.SQLite.Path, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite state for secondary storage: %w", err)
	}

	if params.Storage.Secondary.Path != "" { //nolint:staticcheck // migration only
		// perform a one-time migration from BoltDB to SQLite
		if err = migrateBoltDBToSQLite(
			ctx,
			logger,
			params.Storage.Secondary.Path, //nolint:staticcheck // migration only
			secondaryPersistentState.State,
		); err != nil {
			return nil, fmt.Errorf("failed to migrate secondary storage from BoltDB to SQLite: %w", err)
		}
	}

	namespacedState, err := newNamespacedState(
		defaultPersistentState.State,
		secondaryPersistentState.State,
		virtualState,
		logger,
	)
	if err != nil {
		return nil, err
	}

	measuredState := stateWithMetrics(namespacedState, metricsRegistry)
	defaultState := state.WrapCore(measuredState)

	if err = initResources(ctx, defaultState, logger); err != nil {
		return nil, err
	}

	auditWrap, err := NewAuditWrap(defaultState, config.Config, logger)
	if err != nil {
		return nil, err
	}

	defaultState = auditWrap.WrapState(defaultState)

	return &State{
		defaultState: defaultState,
		virtualState: virtualState,
		auditWrap:    auditWrap,

		defaultPersistentState:   defaultPersistentState,
		secondaryPersistentState: secondaryPersistentState,
	}, nil
}

func newNamespacedState(
	defaultCoreState state.CoreState,
	secondaryCoreState state.CoreState,
	virtualState *virtual.State,
	logger *zap.Logger,
) (*namespaced.State, error) {
	storeFactory, err := store.NewStoreFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd backup store: %w", err)
	}

	buildEphemeralState := inmem.NewStateWithOptions(inmem.WithHistoryGap(20), inmem.WithHistoryMaxCapacity(10000))
	ephemeralState := buildEphemeralState(resources.EphemeralNamespace)
	metaEphemeralState := buildEphemeralState(meta.NamespaceName)
	infraProviderState := infraprovider.NewState(defaultCoreState, ephemeralState, logger.With(logging.Component("infraprovider_state")))

	namespacedState := namespaced.NewState(func(ns resource.Namespace) state.CoreState {
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

	return namespacedState, nil
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

// NewTestState creates a new test state using the in-memory storage and no virtual one.
func NewTestState(logger *zap.Logger) *State {
	defaultPersistentState := &PersistentState{
		State: namespaced.NewState(inmem.Build),
	}

	return &State{
		defaultState:           state.WrapCore(defaultPersistentState.State),
		defaultPersistentState: defaultPersistentState,
		logger:                 logger,
	}
}
