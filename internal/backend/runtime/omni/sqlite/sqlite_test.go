// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlite_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// TestOpenDBSetsSynchronousNormal checks that every connection of the pool runs with the NORMAL synchronous mode.
func TestOpenDBSetsSynchronousNormal(t *testing.T) {
	t.Parallel()

	conf := config.Default().Storage.Sqlite
	conf.SetPath(filepath.Join(t.TempDir(), "test.db"))
	conf.SetCachedPoolSize(2)
	conf.SetPoolSize(2)

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, sqlite.CloseDB(db, 5*time.Second))
	})

	ctx := t.Context()

	// take both connections at once, so that both are freshly opened
	conn1, err := db.Take(ctx)
	require.NoError(t, err)

	conn2, err := db.Take(ctx)
	require.NoError(t, err)

	for _, conn := range []*zombiesqlite.Conn{conn1, conn2} {
		assert.Equal(t, "wal", readPragma(t, conn, "journal_mode"))
		assert.Equal(t, "1", readPragma(t, conn, "synchronous"), "synchronous should be NORMAL")

		db.Put(conn)
	}
}

func readPragma(t *testing.T, conn *zombiesqlite.Conn, name string) string {
	t.Helper()

	var value string

	require.NoError(t, sqlitex.ExecuteTransient(conn, "PRAGMA "+name, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *zombiesqlite.Stmt) error {
			value = stmt.ColumnText(0)

			return nil
		},
	}))

	return value
}
