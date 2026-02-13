// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/containers"
	"go.uber.org/zap"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/sqlitelog"
)

// ErrLogStoreNotFound is returned when the log store for a machine is not found.
var ErrLogStoreNotFound = errors.New("log store not found")

// MachineCacheOption configures optional MachineCache behavior.
type MachineCacheOption func(*MachineCache)

// WithMachineCacheCleanupCallback sets a callback that is called after cleanup with the number of deleted rows.
func WithMachineCacheCleanupCallback(cb func(int)) MachineCacheOption {
	return func(m *MachineCache) {
		m.onCleanup = cb
	}
}

// MachineCache stores a map of machines to their circular log stores. It also allows to access the log stores using the machine IP.
type MachineCache struct {
	machineLogStoreManager LogStoreManager
	machineLogStores       map[MachineID]logstore.LogStore
	logger                 *zap.Logger
	onCleanup              func(int)
	logsConfig             *config.LogsMachine
	secondaryStorageDB     *sqlitex.Pool
	state                  state.State
	mx                     sync.Mutex
	inited                 bool
}

// NewMachineCache returns a new MachineCache.
func NewMachineCache(secondaryStorageDB *sqlitex.Pool, logStorageConfig *config.LogsMachine, omniState state.State, logger *zap.Logger, opts ...MachineCacheOption) (*MachineCache, error) {
	cache := &MachineCache{
		logsConfig:         logStorageConfig,
		secondaryStorageDB: secondaryStorageDB,
		state:              omniState,
		logger:             logger,
	}

	for _, opt := range opts {
		opt(cache)
	}

	return cache, nil
}

// Run starts the side tasks required by the MachineCache.
//
// Currently, it is only the periodic cleanup of old logs in the SQLite log store.
func (m *MachineCache) Run(ctx context.Context) error {
	if err := m.initLocked(ctx); err != nil {
		return fmt.Errorf("failed to initialize machine cache: %w", err)
	}

	if err := m.machineLogStoreManager.Run(ctx); err != nil {
		return fmt.Errorf("failed to run machine log store manager: %w", err)
	}

	return nil
}

// initLocked initializes the cache safely.
func (m *MachineCache) initLocked(ctx context.Context) error {
	m.mx.Lock()
	defer m.mx.Unlock()

	return m.init(ctx)
}

// WriteMessage writes the message surrounded with '\n' to the log store for the given machine ID.
func (m *MachineCache) WriteMessage(ctx context.Context, id MachineID, rawData []byte) error {
	logWriter, err := m.initAndGetLogStore(ctx, id)
	if err != nil {
		return err
	}

	return logWriter.WriteLine(ctx, rawData)
}

// getLogStore returns the log backend for the given machine ID.
func (m *MachineCache) getLogStore(ctx context.Context, id MachineID) (logstore.LogStore, error) {
	m.mx.Lock()
	defer m.mx.Unlock()

	if err := m.init(ctx); err != nil {
		return nil, err
	}

	val, ok := m.machineLogStores[id]
	if ok {
		return val, nil
	}

	exists, err := m.machineLogStoreManager.Exists(ctx, string(id))
	if err != nil {
		return nil, fmt.Errorf("failed to check if log store exists for machine %q: %w", id, err)
	}

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrLogStoreNotFound, id)
	}

	return m.getOrCreateStore(id)
}

// initAndGetLogStore returns the log store for the given machine ID, creating it if it does not exist.
func (m *MachineCache) initAndGetLogStore(ctx context.Context, id MachineID) (logstore.LogStore, error) {
	m.mx.Lock()
	defer m.mx.Unlock()

	if err := m.init(ctx); err != nil {
		return nil, err
	}

	val, err := m.getOrCreateStore(id)
	if err != nil {
		return nil, err
	}

	return val, nil
}

// Remove removes the log store for the given machine ID.
//
// If storage is enabled, it also removes the logs from the storage.
func (m *MachineCache) remove(ctx context.Context, id MachineID) error {
	m.mx.Lock()
	defer m.mx.Unlock()

	m.logger.Info("remove logs for machine", zap.String("machine_id", string(id)))

	if !m.inited {
		return nil
	}

	store, ok := m.machineLogStores[id]
	if !ok {
		return nil
	}

	if err := store.Close(); err != nil {
		m.logger.Error("failed to close store", zap.String("machine_id", string(id)), zap.Error(err))
	}

	delete(m.machineLogStores, id)

	if err := m.machineLogStoreManager.Remove(ctx, string(id)); err != nil {
		return fmt.Errorf("failed to remove logs from storage for machine %q: %w", id, err)
	}

	return nil
}

// Close closes all the log stores, triggering a flush to the storage for each of them if they have persistence enabled.
func (m *MachineCache) Close() error {
	m.mx.Lock()
	defer m.mx.Unlock()

	var errs error

	for _, store := range m.machineLogStores {
		if err := store.Close(); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

// init initializes the MachineCache.
func (m *MachineCache) init(ctx context.Context) error {
	if m.inited {
		return nil
	}

	var storeManagerOpts []sqlitelog.StoreManagerOption
	if m.onCleanup != nil {
		storeManagerOpts = append(storeManagerOpts, sqlitelog.WithCleanupCallback(m.onCleanup))
	}

	sqliteLogStoreManager, err := sqlitelog.NewStoreManager(ctx, m.secondaryStorageDB, m.logsConfig.Storage, m.state, m.logger, storeManagerOpts...)
	if err != nil {
		return fmt.Errorf("failed to create sqlite log store manager: %w", err)
	}

	m.machineLogStoreManager = sqliteLogStoreManager

	m.inited = true
	m.machineLogStores = map[MachineID]logstore.LogStore{}

	return nil
}

func (m *MachineCache) getOrCreateStore(id MachineID) (logstore.LogStore, error) {
	store, ok := m.machineLogStores[id]
	if ok {
		return store, nil
	}

	created, err := m.machineLogStoreManager.Create(string(id))
	if err != nil {
		return nil, fmt.Errorf("failed to create log store for machine %q: %w", id, err)
	}

	m.machineLogStores[id] = created

	return created, err
}

// NewMachineMap returns a new MachineMap.
func NewMachineMap(storage MachineStorage) *MachineMap {
	return &MachineMap{
		lazyMap: containers.LazyBiMap[string, MachineID]{
			Creator: func(ip string) (MachineID, error) {
				return storage.GetMachine(ip)
			},
		},
	}
}

// MachineMap is a map of IP to machineID.
type MachineMap struct {
	lazyMap containers.LazyBiMap[string, MachineID]
	mx      sync.Mutex
}

// MachineID is a machine ID type.
type MachineID string

// GetMachineID returns the machine ID for the given IP.
func (m *MachineMap) GetMachineID(ip string) (MachineID, error) {
	m.mx.Lock()
	defer m.mx.Unlock()

	return m.lazyMap.GetOrCreate(ip)
}

// RemoveByMachineID removes the given machine from the map.
func (m *MachineMap) RemoveByMachineID(id MachineID) {
	m.mx.Lock()
	defer m.mx.Unlock()

	m.lazyMap.RemoveInverse(id)
}

// MachineStorage returns machineID for the given IP.
type MachineStorage interface {
	GetMachine(machineIP string) (MachineID, error)
}

// MapStorage is a concrete implementation of MachineStorage that uses the map.
type MapStorage struct {
	IPToMachine map[string]MachineID
}

// GetMachine returns the machine with the given IP.
func (s *MapStorage) GetMachine(ip string) (MachineID, error) {
	if machine, ok := s.IPToMachine[ip]; ok {
		return machine, nil
	}

	panic("no machine found")
}

// NewStateStorage returns a new StateStorage.
func NewStateStorage(state state.State) *StateStorage {
	return &StateStorage{state: state}
}

// StateStorage is a concrete implementation of MachineStorage that uses the state.
type StateStorage struct {
	state state.State
}

// GetMachine returns the machineID from state using the IP address.
func (s *StateStorage) GetMachine(machineIP string) (MachineID, error) {
	ctx := actor.MarkContextAsInternalActor(context.Background())

	machines, err := safe.StateListAll[*omni.Machine](
		ctx,
		s.state,
		state.WithLabelQuery(resource.LabelEqual(omni.MachineAddressLabel, machineIP)),
	)
	if err != nil {
		return "", err
	}

	if machines.Len() == 0 {
		return "", fmt.Errorf("no machine found for address %s", machineIP)
	}

	return MachineID(machines.Get(0).Metadata().ID()), nil
}
