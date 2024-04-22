// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-circular"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstorage"
)

// MachineCache stores a map of machines to their circular log buffers. It also allows to access the buffers
// using the machine IP.
type MachineCache struct {
	machineBuffers containers.LazyMap[MachineID, *circular.Buffer]
	logger         *zap.Logger
	Storage        optional.Optional[*logstorage.Storage]
	mx             sync.Mutex
	inited         bool
}

// NewMachineCache returns a new MachineCache.
func NewMachineCache(storage optional.Optional[*logstorage.Storage], logger *zap.Logger) *MachineCache {
	return &MachineCache{
		Storage: storage,
		logger:  logger,
	}
}

// WriteMessage writes the message surrounded with '\n' to the circular buffer for the given machine ID.
func (m *MachineCache) WriteMessage(id MachineID, rawData []byte) error {
	buffer, err := m.GetWriter(id)
	if err != nil {
		return err
	}

	_, err = buffer.Write([]byte("\n"))
	if err != nil {
		return err
	}

	// bytes were written, mark the buffer as dirty
	if storage, ok := m.Storage.Get(); ok {
		storage.MarkDirty(string(id))
	}

	_, err = buffer.Write(rawData)
	if err != nil {
		return err
	}

	_, err = buffer.Write([]byte("\n"))
	if err != nil {
		return err
	}

	return nil
}

// GetBuffer returns the circular buffer for the given machine ID.
func (m *MachineCache) GetBuffer(id MachineID) (*circular.Buffer, error) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.init()

	val, ok := m.machineBuffers.Get(id)
	if ok {
		return val, nil
	}

	storage, storageOk := m.Storage.Get()
	if !storageOk {
		return nil, &BufferNotFoundError{id: id}
	}

	// if storage is enabled and the log file exists, initialize the buffer from the file
	exists, err := storage.Exists(string(id))
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, &BufferNotFoundError{id: id}
	}

	return m.machineBuffers.GetOrCreate(id)
}

// GetWriter returns the circular buffer for the given machine ID. Creates buffer if it doesn't exist.
func (m *MachineCache) GetWriter(id MachineID) (io.Writer, error) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.init()

	val, err := m.machineBuffers.GetOrCreate(id)
	if err != nil {
		return nil, err
	}

	return val, nil
}

// Remove removes the circular buffer for the given machine ID.
// If storage is enabled, it also removes the logs from the storage i.e. the file system.
func (m *MachineCache) Remove(id MachineID) error {
	m.mx.Lock()
	defer m.mx.Unlock()

	idStr := string(id)

	m.logger.Info("remove logs for machine", zap.String("machine_id", idStr))

	if !m.inited {
		return nil
	}

	m.machineBuffers.Remove(id)

	if storage, ok := m.Storage.Get(); ok {
		return storage.Remove(idStr)
	}

	return nil
}

// SaveAll saves all the logs to the storage, i.e., the file system.
func (m *MachineCache) SaveAll() error {
	storage, storageEnabled := m.Storage.Get()

	if !storageEnabled {
		return errors.New("storage is not enabled")
	}

	var multiErr error

	for id, reader := range m.getMachineBufferReaders() {
		err := storage.Save(string(id), reader)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	return multiErr
}

// getMachineBufferReaders takes a snapshot (copy) of the machine ID to buffer map in a thread-safe manner.
func (m *MachineCache) getMachineBufferReaders() map[MachineID]io.Reader {
	m.mx.Lock()
	defer m.mx.Unlock()

	bufferReaderMap := map[MachineID]io.Reader{}

	m.machineBuffers.ForEach(func(id MachineID, buffer *circular.Buffer) {
		bufferReaderMap[id] = buffer.GetReader()
	})

	return bufferReaderMap
}

func (m *MachineCache) init() {
	if m.inited {
		return
	}

	m.inited = true
	m.machineBuffers = containers.LazyMap[MachineID, *circular.Buffer]{
		Creator: func(id MachineID) (*circular.Buffer, error) {
			buffer, err := circular.NewBuffer(
				circular.WithInitialCapacity(InitialCapacity),
				circular.WithMaxCapacity(MaxCapacity),
				circular.WithSafetyGap(SafetyGap))
			if err != nil {
				return nil, fmt.Errorf("failed to create circular buffer for machine '%s': %w", id, err)
			}

			storage, storageEnabled := m.Storage.Get()

			if storageEnabled {
				idStr := string(id)

				m.logger.Info("load logs for machine", zap.String("machine_id", idStr))

				if err = storage.Load(idStr, buffer); err != nil {
					m.logger.Error("failed to load logs for machine", zap.String("machine_id", idStr), zap.Error(err))
				}
			}

			return buffer, nil
		},
	}
}

// These constants should some day move to config.
const (
	// Some logs are tiny, no need to reserve too much memory.
	InitialCapacity = 16384
	// Cap each log at 1M.
	MaxCapacity = 1048576
	// Safety gap to avoid buffer overruns.
	SafetyGap = 2048
)

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

// BufferNotFoundError is returned when the machine buffer is not found.
type BufferNotFoundError struct {
	id MachineID
}

// Error returns the error message.
func (e *BufferNotFoundError) Error() string {
	return fmt.Sprintf("no buffer found for machine '%s'", e.id)
}

// IsBufferNotFoundError returns true if the error is of type BufferNotFoundError.
func IsBufferNotFoundError(err error) bool {
	var target *BufferNotFoundError

	return errors.As(err, &target)
}
