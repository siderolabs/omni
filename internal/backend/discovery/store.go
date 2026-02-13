// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package discovery

import (
	"context"
	"fmt"

	"github.com/siderolabs/discovery-service/pkg/storage"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// InitSQLiteSnapshotStore initializes a SQLite snapshot store if enabled in the config.
func InitSQLiteSnapshotStore(ctx context.Context, config config.EmbeddedDiscoveryService, db *sqlitex.Pool) (storage.SnapshotStore, error) {
	store, err := NewSQLiteStore(ctx, db, config.GetSqliteTimeout())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sqlite snapshot store: %w", err)
	}

	return store, nil
}
