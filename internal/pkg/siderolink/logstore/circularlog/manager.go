// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//nolint:staticcheck // circularlog is deprecated, but it is fine here
package circularlog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-circular/zstd"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
)

// StoreManager manages circular log stores for machines.
type StoreManager struct {
	config     *config.LogsMachine
	logger     *zap.Logger
	compressor *zstd.Compressor
}

// Run implements the LogStoreManager interface.
func (m *StoreManager) Run(ctx context.Context) error {
	// nothing to do, circular log cleanup is handled inside the library
	<-ctx.Done()

	return nil
}

// Exists implements the LogStoreManager interface.
func (m *StoreManager) Exists(_ context.Context, id string) (bool, error) {
	if !m.config.Storage.Enabled {
		return false, nil
	}

	matches, err := m.logFiles(id)
	if err != nil {
		return false, fmt.Errorf("failed to list log files for machine %q: %w", id, err)
	}

	return len(matches) > 0, nil
}

// Remove implements the LogStoreManager interface.
func (m *StoreManager) Remove(_ context.Context, id string) error {
	matches, err := m.logFiles(id)
	if err != nil {
		return fmt.Errorf("failed to list log files for machine %q: %w", id, err)
	}

	var errs error

	for _, match := range matches {
		if err = os.Remove(match); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

// MachineIDs returns the list of machine IDs that have persistent log stores.
func (m *StoreManager) MachineIDs() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(m.config.Storage.Path, "*.log*"))
	if err != nil {
		return nil, fmt.Errorf("failed to list log files: %w", err)
	}

	ids := make([]string, 0, len(matches))

	for _, match := range matches {
		base := filepath.Base(match)
		id := strings.SplitN(base, ".log", 2)[0]
		ids = append(ids, id)
	}

	slices.Sort(ids)

	return slices.Compact(ids), nil
}

// logFiles returns all log files for the given machine ID.
//
// It probes the file system to check if a log file exists for this machine.
// Checks both for the old (/path/machine-id.log) and the new (/path/machine-id.log.NUM) format.
func (m *StoreManager) logFiles(id string) ([]string, error) {
	return filepath.Glob(filepath.Join(m.config.Storage.Path, id+".log*"))
}

// Create implements the LogStoreManager interface.
func (m *StoreManager) Create(id string) (logstore.LogStore, error) {
	return NewStore(m.config, id, m.compressor, m.logger)
}

// NewStoreManager returns a new StoreManager.
func NewStoreManager(config *config.LogsMachine, compressor *zstd.Compressor, logger *zap.Logger) *StoreManager {
	return &StoreManager{
		config:     config,
		compressor: compressor,
		logger:     logger,
	}
}
