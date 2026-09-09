// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package sqlite provides helper functions for working with SQLite databases in the Omni runtime.
package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"github.com/siderolabs/gen/panicsafe"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// OpenDB opens a SQLite database with the given configuration.
func OpenDB(config config.SQLite) (*sqlitexx.Pool, error) {
	configPath := config.GetPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory for sqlite database %q: %w", configPath, err)
	}

	dsn := "file:" + configPath
	if extraParams := config.GetExtraParams(); extraParams != "" {
		dsn += "?" + extraParams
	}

	db, err := sqlitexx.NewPool(
		dsn,
		sqlitexx.PoolOptions{
			Flags:         zombiesqlite.OpenReadWrite | zombiesqlite.OpenCreate | zombiesqlite.OpenWAL | zombiesqlite.OpenURI,
			LowWatermark:  config.GetCachedPoolSize(),
			HighWatermark: config.GetPoolSize(),
			// the synchronous mode is per connection, NORMAL skips the fsync on every commit in WAL mode
			PrepareConn: func(conn *zombiesqlite.Conn) error {
				return sqlitex.ExecuteTransient(conn, "PRAGMA synchronous=NORMAL", nil)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %q: %w", dsn, err)
	}

	return db, nil
}

// CloseDB closes the given SQLite database pool.
//
// The upstream Close function might block until all connections are returned to the pool.
// Provide a timeout to avoid blocking indefinitely.
func CloseDB(db *sqlitexx.Pool, timeout time.Duration) error {
	errCh := make(chan error)

	go func() {
		errCh <- panicsafe.RunErr(db.Close)
	}()

	select {
	case err := <-errCh:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout of %s exceeded while closing sqlite database", timeout)
	}
}
