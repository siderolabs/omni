// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
package discovery_test

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	// Ensure SQLite driver is loaded.
	_ "modernc.org/sqlite"

	"github.com/siderolabs/omni/internal/backend/discovery"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestInitSQLiteSnapshotStore_MigrateSuccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	// 1. Setup Legacy File State
	// Create a temporary directory for the snapshot file
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshot.bin")
	expectedData := []byte("legacy-snapshot-data")

	require.NoError(t, os.WriteFile(snapshotPath, expectedData, 0o644))

	// 2. Trigger Init with Migration Enabled
	conf := &config.EmbeddedDiscoveryService{
		SQLiteSnapshotsEnabled: true,
		SnapshotsPath:          snapshotPath, //nolint:staticcheck
	}

	store, err := discovery.InitSQLiteSnapshotStore(ctx, conf, db, logger)
	require.NoError(t, err)

	// 3. Verify Data moved to SQLite
	rdr, err := store.Reader(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	data, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Equal(t, expectedData, data, "sqlite store should contain data from legacy file")

	// 4. Verify File is Removed
	_, err = os.Stat(snapshotPath)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err), "legacy file should be removed after successful migration")
}

func TestInitSQLiteSnapshotStore_SkipIfDataExists(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	// 1. Setup Pre-existing SQLite Data
	// We manually initialize a store to write data before testing Init
	preStore, err := discovery.NewSQLiteStore(ctx, db)
	require.NoError(t, err)

	dbData := []byte("existing-db-data")
	wd, err := preStore.Writer(ctx)
	require.NoError(t, err)
	_, err = wd.Write(dbData)
	require.NoError(t, err)
	require.NoError(t, wd.Close())

	// 2. Setup Legacy File (Simulate a stale file)
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshot.bin")
	fileData := []byte("stale-file-data")

	require.NoError(t, os.WriteFile(snapshotPath, fileData, 0o644))

	// 3. Trigger Init
	conf := &config.EmbeddedDiscoveryService{
		SQLiteSnapshotsEnabled: true,
		SnapshotsPath:          snapshotPath, //nolint:staticcheck
	}

	store, err := discovery.InitSQLiteSnapshotStore(ctx, conf, db, logger)
	require.NoError(t, err)

	// 4. Verify SQLite Data Preserved (Migration Skipped)
	rdr, err := store.Reader(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rdr.Close()) })

	actualData, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Equal(t, dbData, actualData, "should preserve existing sqlite data")
	assert.NotEqual(t, fileData, actualData, "should not overwrite with file data")

	// 5. Verify File Still Exists
	_, err = os.Stat(snapshotPath)
	require.NoError(t, err, "legacy file should NOT be removed if migration was skipped")
}

func TestInitSQLiteSnapshotStore_NoFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	// 1. Setup Path to non-existent file
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "does-not-exist.bin")

	// 2. Trigger Init
	conf := &config.EmbeddedDiscoveryService{
		SQLiteSnapshotsEnabled: true,
		SnapshotsPath:          snapshotPath,
	}

	store, err := discovery.InitSQLiteSnapshotStore(ctx, conf, db, logger)
	require.NoError(t, err)

	// 3. Verify SQLite is empty (no error)
	rdr, err := store.Reader(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rdr.Close()) })

	data, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestInitSQLiteSnapshotStore_Disabled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	tests := []struct {
		conf *config.EmbeddedDiscoveryService
		name string
	}{
		{
			name: "Nil Config",
			conf: nil,
		},
		{
			name: "Disabled Flag",
			conf: &config.EmbeddedDiscoveryService{
				SQLiteSnapshotsEnabled: false,
				SnapshotsPath:          "/some/path", //nolint:staticcheck
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store, err := discovery.InitSQLiteSnapshotStore(ctx, tt.conf, db, logger)
			require.NoError(t, err)
			assert.Nil(t, store, "store should be nil when disabled or config is nil")
		})
	}
}
