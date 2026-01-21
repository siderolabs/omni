// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// OpenDB opens a SQLite database with the given configuration.
func OpenDB(config config.SQLite) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(config.Path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory for sqlite database %q: %w", config.Path, err)
	}

	allParams := config.ExperimentalBaseParams
	if config.ExtraParams != "" {
		allParams += "&" + config.ExtraParams
	}

	dsn := "file:" + config.Path
	if allParams != "" {
		dsn += "?" + allParams
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %q: %w", dsn, err)
	}

	return db, nil
}
