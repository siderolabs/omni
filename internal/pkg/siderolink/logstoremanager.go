// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-circular/zstd"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/circularlog"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/sqlitelog"
)

type LogStoreManager interface {
	io.Closer
	Exists(ctx context.Context, id MachineID) (bool, error)
	Create(ctx context.Context, id MachineID) (logstore.LogStore, error)
	Remove(ctx context.Context, id MachineID) error
}

type sqliteLogStoreManager struct {
	db     *sql.DB
	logger *zap.Logger
	config config.LogsMachineSQLite
}

func (f *sqliteLogStoreManager) Exists(ctx context.Context, id MachineID) (bool, error) {
	return sqlitelog.Exists(ctx, f.db, string(id), f.config.Timeout)
}

func (f *sqliteLogStoreManager) Remove(ctx context.Context, id MachineID) error {
	return sqlitelog.Remove(ctx, f.db, string(id), f.config.Timeout)
}

func newSQLiteStoreManager(config config.LogsMachineSQLite, logger *zap.Logger) (*sqliteLogStoreManager, error) {
	path := config.Path
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory for sqlite database %q: %w", dir, err)
	}

	db, err := sqlitelog.OpenDB(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %q: %w", path, err)
	}

	return &sqliteLogStoreManager{
		config: config,
		db:     db,
		logger: logger,
	}, nil
}

func (f *sqliteLogStoreManager) Close() error {
	return f.db.Close()
}

func (f *sqliteLogStoreManager) Create(ctx context.Context, id MachineID) (logstore.LogStore, error) {
	return sqlitelog.NewStore(ctx, f.config, f.db, string(id), f.logger)
}

type circularLogStoreManager struct {
	config     *config.LogsMachine
	logger     *zap.Logger
	compressor *zstd.Compressor
}

func (c *circularLogStoreManager) Exists(_ context.Context, id MachineID) (bool, error) {
	if !c.config.Storage.Enabled {
		return false, nil
	}

	matches, err := c.logFiles(id)
	if err != nil {
		return false, fmt.Errorf("failed to list log files for machine %q: %w", id, err)
	}

	return len(matches) > 0, nil
}

func (c *circularLogStoreManager) Remove(_ context.Context, id MachineID) error {
	matches, err := c.logFiles(id)
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

// logFiles returns all log files for the given machine ID.
//
// It probes the file system to check if a log file exists for this machine.
// Checks both for the old (/path/machine-id.log) and the new (/path/machine-id.log.NUM) format.
func (c *circularLogStoreManager) logFiles(id MachineID) ([]string, error) {
	return filepath.Glob(filepath.Join(c.config.Storage.Path, string(id)+".log*"))
}

func (c *circularLogStoreManager) Create(_ context.Context, id MachineID) (logstore.LogStore, error) {
	return circularlog.NewStore(c.config, string(id), c.compressor, c.logger)
}

func newCircularLogStoreManager(config *config.LogsMachine, compressor *zstd.Compressor, logger *zap.Logger) *circularLogStoreManager {
	return &circularLogStoreManager{
		config:     config,
		compressor: compressor,
		logger:     logger,
	}
}

func (c *circularLogStoreManager) Close() error {
	return nil
}
