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

	"github.com/siderolabs/gen/panicsafe"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// OpenDB opens a SQLite database with the given configuration.
func OpenDB(config config.SQLite) (*sqlitex.Pool, error) {
	configPath := config.GetPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory for sqlite database %q: %w", configPath, err)
	}

	allParams := config.GetExperimentalBaseParams()

	extraParams := config.GetExtraParams()
	if extraParams != "" {
		allParams += "&" + extraParams
	}

	dsn := "file:" + configPath
	if allParams != "" {
		dsn += "?" + allParams
	}

	db, err := sqlitex.NewPool(dsn,
		sqlitex.PoolOptions{
			Flags:    zombiesqlite.OpenReadWrite | zombiesqlite.OpenCreate | zombiesqlite.OpenWAL | zombiesqlite.OpenURI,
			PoolSize: config.GetPoolSize(),
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
func CloseDB(db *sqlitex.Pool, timeout time.Duration) error {
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
