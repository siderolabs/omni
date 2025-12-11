// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package discovery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/siderolabs/discovery-service/pkg/storage"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// InitSQLiteSnapshotStore initializes a SQLite snapshot store if enabled in the config.
func InitSQLiteSnapshotStore(ctx context.Context, config *config.EmbeddedDiscoveryService, db *sql.DB, logger *zap.Logger) (storage.SnapshotStore, error) {
	store, err := NewSQLiteStore(ctx, db, config.SQLiteTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sqlite snapshot store: %w", err)
	}

	if config.SnapshotsPath != "" { //nolint:staticcheck
		if err = migrate(ctx, config.SnapshotsPath, store, logger); err != nil { //nolint:staticcheck
			logger.Error("failed to migrate discovery service state to sqlite store", zap.Error(err))
		}
	}

	return store, nil
}

func migrate(ctx context.Context, path string, sqliteStore *SQLiteStore, logger *zap.Logger) error {
	rdr, err := sqliteStore.Reader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get reader for sqlite store: %w", err)
	}

	defer rdr.Close() //nolint:errcheck

	checkBuf := make([]byte, 1)

	n, err := rdr.Read(checkBuf)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to read existing discovery service state from sqlite store: %w", err)
	}

	if n > 0 {
		logger.Info("skip discovery service state migration: data already exists in SQLite store")

		return nil
	}

	fileData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("skip discovery service state migration: no such file or directory")

			return nil
		}

		return fmt.Errorf("failed to read discovery service state from file: %w", err)
	}

	wd, err := sqliteStore.Writer(ctx)
	if err != nil {
		return fmt.Errorf("failed to get writer for sqlite store: %w", err)
	}

	defer wd.Close() //nolint:errcheck

	if _, err = wd.Write(fileData); err != nil {
		return fmt.Errorf("failed to write discovery service state to sqlite store: %w", err)
	}

	if err = wd.Close(); err != nil {
		return fmt.Errorf("failed to close (flush) sqlite store writer: %w", err)
	}

	logger.Info("discovery service state migrated to sqlite store successfully")

	if err = os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to remove old discovery service state file: %w", err)
	}

	logger.Info("old discovery service state file removed successfully")

	return nil
}
