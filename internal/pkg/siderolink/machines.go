// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/go-circular"
	"github.com/siderolabs/go-circular/zstd"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// MachineCache stores a map of machines to their circular log buffers. It also allows to access the buffers
// using the machine IP.
type MachineCache struct {
	machineBuffers   containers.LazyMap[MachineID, *circular.Buffer]
	logger           *zap.Logger
	logStorageConfig *config.MachineLogConfigParams
	compressor       *zstd.Compressor
	mx               sync.Mutex
	inited           bool
}

// NewMachineCache returns a new MachineCache.
func NewMachineCache(logStorageConfig *config.MachineLogConfigParams, logger *zap.Logger) (*MachineCache, error) {
	compressor, err := zstd.NewCompressor()
	if err != nil {
		return nil, fmt.Errorf("failed to create log compressor: %w", err)
	}

	return &MachineCache{
		logStorageConfig: logStorageConfig,
		compressor:       compressor,
		logger:           logger,
	}, nil
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

	if !m.logStorageConfig.StorageEnabled {
		return nil, &BufferNotFoundError{id: id}
	}

	// Probe the file system to check if a log file exists for this machine.
	// Check both for the old (/path/machine-id.log) and the new (/path/machine-id.log.NUM) format.
	glob := filepath.Join(m.logStorageConfig.StoragePath, string(id)+".log*")

	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("failed to list log files for machine %q: %w", id, err)
	}

	if len(matches) == 0 {
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
// If storage is enabled, it also removes the logs from the storage i.e., the file system.
func (m *MachineCache) Remove(id MachineID) error {
	m.mx.Lock()
	defer m.mx.Unlock()

	m.logger.Info("remove logs for machine", zap.String("machine_id", string(id)))

	if !m.inited {
		return nil
	}

	buf, ok := m.machineBuffers.Get(id)
	if !ok {
		return nil
	}

	if err := buf.Close(); err != nil {
		m.logger.Error("failed to close buffer", zap.String("machine_id", string(id)), zap.Error(err))
	}

	m.machineBuffers.Remove(id)

	matches, err := filepath.Glob(filepath.Join(m.logStorageConfig.StoragePath, string(id)+".log*"))
	if err != nil {
		return fmt.Errorf("failed to list log files for machine %q: %w", id, err)
	}

	var errs error

	for _, match := range matches {
		if err = os.Remove(match); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = multierror.Append(errs, err)
		}
	}

	if errs != nil {
		return fmt.Errorf("failed to remove log files for machine %q: %w", id, errs)
	}

	return nil
}

// Close closes all the circular buffers, triggering a flush to the storage for each of them if they have persistence enabled.
func (m *MachineCache) Close() error {
	var errs error

	m.machineBuffers.ForEach(func(_ MachineID, buffer *circular.Buffer) {
		if err := buffer.Close(); err != nil {
			errs = multierror.Append(errs, err)
		}
	})

	return errs
}

func (m *MachineCache) init() {
	if m.inited {
		return
	}

	m.inited = true
	m.machineBuffers = containers.LazyMap[MachineID, *circular.Buffer]{
		Creator: func(id MachineID) (*circular.Buffer, error) {
			bufferOpts := []circular.OptionFunc{
				circular.WithInitialCapacity(m.logStorageConfig.BufferInitialCapacity),
				circular.WithMaxCapacity(m.logStorageConfig.BufferMaxCapacity),
				circular.WithSafetyGap(m.logStorageConfig.BufferSafetyGap),
				circular.WithNumCompressedChunks(m.logStorageConfig.NumCompressedChunks, m.compressor),
				circular.WithLogger(m.logger),
			}

			if m.logStorageConfig.StorageEnabled {
				bufferOpts = append(bufferOpts, circular.WithPersistence(circular.PersistenceOptions{
					ChunkPath:     filepath.Join(m.logStorageConfig.StoragePath, string(id)+".log"),
					FlushInterval: m.logStorageConfig.StorageFlushPeriod,
					FlushJitter:   m.logStorageConfig.StorageFlushJitter,
				}))
			}

			buffer, err := circular.NewBuffer(bufferOpts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create circular buffer for machine '%s': %w", id, err)
			}

			if m.logStorageConfig.StorageEnabled {
				loadLegacyLogs(m.logStorageConfig, id, buffer, m.logger)
			}

			return buffer, nil
		},
	}
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

// loadLegacyLogs loads logs stored of the machine with the given id in the old format, if exists, into the given writer.
// It is used to migrate logs from the old format to the new format.
// It removes the old log file and its hash file regardless of the result.
//
// It is a best-effort function and does not return any error.
func loadLegacyLogs(config *config.MachineLogConfigParams, id MachineID, writer io.Writer, logger *zap.Logger) {
	filePath := filepath.Join(config.StoragePath, fmt.Sprintf("%s.log", id))
	shaSumPath := filePath + ".sha256sum"

	defer func() {
		if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Error("failed to remove legacy log file", zap.String("path", filePath), zap.Error(err))
		}

		if err := os.Remove(shaSumPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Error("failed to remove legacy log hash file", zap.String("path", shaSumPath), zap.Error(err))
		}
	}()

	bufferData, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}

		logger.Error("failed to read legacy log buffer file", zap.String("path", filePath), zap.Error(err))

		return
	}

	hashHexExpectedBytes, err := os.ReadFile(shaSumPath)
	if err != nil {
		logger.Error("failed to read legacy log buffer hash file", zap.String("path", shaSumPath), zap.Error(err))

		return
	}

	hashHexExpected := string(hashHexExpectedBytes)

	// verify the hash
	hashActual := sha256.Sum256(bufferData)
	hashHexActual := hex.EncodeToString(hashActual[:])

	if hashHexExpected != hashHexActual {
		logger.Error("invalid legacy log buffer hash in file", zap.String("expected", hashHexExpected), zap.String("actual", hashHexActual))

		return
	}

	if _, err = io.Copy(writer, bytes.NewReader(bufferData)); err != nil {
		logger.Error("failed to write legacy log buffer to writer", zap.Error(err))
	}

	logger.Info("loaded legacy log buffer", zap.String("path", filePath))
}
