// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogfile"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogsqlite"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestMigrateFromFileToSQLite(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	// 1. Setup File Logger state (Source)
	// We manually populate the file backend to simulate existing legacy logs.
	dir := t.TempDir()
	fileStore := auditlogfile.New(dir)

	event1 := auditlog.MakeEvent("create", "file.resource", &auditlog.Data{Session: auditlog.Session{UserID: "u1"}})
	// ensure time is in the past for determinism
	event1.TimeMillis = time.Now().Add(-2 * time.Hour).UnixMilli()
	require.NoError(t, fileStore.Write(ctx, event1))

	event2 := auditlog.MakeEvent("update", "file.resource", &auditlog.Data{Session: auditlog.Session{UserID: "u2"}})
	event2.TimeMillis = time.Now().Add(-1 * time.Hour).UnixMilli()
	require.NoError(t, fileStore.Write(ctx, event2))

	// Verify files exist
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "log files should exist before migration")

	// 2. Setup Database
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dbConf := config.Default().Storage.SQLite
	dbConf.Path = dbPath

	db, err := sqlite.OpenDB(dbConf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	// 3. Trigger Migration via NewLog
	// We provide both Path and SQLite enabled, which triggers initLogger -> migrateFromFileToSQLite
	logConf := config.LogsAudit{
		Path: dir,
		SQLite: config.LogsAuditSQLite{
			Enabled: true,
			Timeout: 5 * time.Second,
		},
	}

	auditLogger, err := audit.NewLog(ctx, logConf, db, logger)
	require.NoError(t, err)

	// 4. Verify Data via the returned Logger
	// The logger should now be backed by SQLite and contain the migrated data.
	rdr, err := auditLogger.Reader(ctx, time.Time{}, time.Now().Add(time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)
	require.Len(t, events, 2)
	assert.Equal(t, event1.Data.Session.UserID, events[0].Data.Session.UserID)
	assert.Equal(t, event2.Data.Session.UserID, events[1].Data.Session.UserID)

	// 5. Verify File logs are removed
	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "log files should be removed after successful migration")
}

func TestMigrateSkipIfHasData(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	// 1. Setup File Logger state (Source)
	dir := t.TempDir()
	fileStore := auditlogfile.New(dir)

	fileEvt := auditlog.MakeEvent("create", "file.resource", &auditlog.Data{Session: auditlog.Session{UserID: "file-user"}})
	require.NoError(t, fileStore.Write(ctx, fileEvt))

	// 2. Setup Database with EXISTING DATA
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dbConf := config.Default().Storage.SQLite
	dbConf.Path = dbPath

	db, err := sqlite.OpenDB(dbConf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	// Pre-populate DB using internal store to simulate existing state
	sqliteStore, err := auditlogsqlite.NewStore(ctx, db, 5*time.Second)
	require.NoError(t, err)

	dbEvt := auditlog.MakeEvent("create", "db.resource", &auditlog.Data{Session: auditlog.Session{UserID: "db-user"}})
	require.NoError(t, sqliteStore.Write(ctx, dbEvt))

	// 3. Trigger NewLog
	logConf := config.LogsAudit{
		Path: dir,
		SQLite: config.LogsAuditSQLite{
			Enabled: true,
			Timeout: 5 * time.Second,
		},
	}

	auditLogger, err := audit.NewLog(ctx, logConf, db, logger)
	require.NoError(t, err)

	// 4. Verify Migration SKIPPED
	// The reader should only return the DB event ("db-user"), IGNORING the file event ("file-user")
	rdr, err := auditLogger.Reader(ctx, time.Time{}, time.Now().Add(time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)
	require.Len(t, events, 1)
	assert.Equal(t, "db-user", events[0].Data.Session.UserID)

	// 5. Verify File logs are NOT removed
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "log files should NOT be removed if migration was skipped")
}

func TestMigrateWithCorruptData(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	// 1. Setup Source Directory
	dir := t.TempDir()

	// 2. Create a log file with mixed content (Valid, Corrupt, Valid)
	logContent := `{"event_type":"create","event_ts":100,"event_data":{"session":{"user_id":"u1"}}}
THIS_IS_BROKEN_JSON
{"event_type":"update","event_ts":200,"event_data":{"session":{"user_id":"u2"}}}`

	logPath := filepath.Join(dir, time.Now().UTC().Format("2006-01-02")+".jsonlog")
	require.NoError(t, os.WriteFile(logPath, []byte(logContent), 0o644))

	// 3. Setup Database
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dbConf := config.Default().Storage.SQLite
	dbConf.Path = dbPath

	db, err := sqlite.OpenDB(dbConf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	// 4. Trigger Migration
	logConf := config.LogsAudit{
		Path: dir,
		SQLite: config.LogsAuditSQLite{
			Enabled: true,
			Timeout: 5 * time.Second,
		},
	}

	auditLogger, err := audit.NewLog(ctx, logConf, db, logger)
	require.NoError(t, err)

	// 5. Verify Data in SQLite
	rdr, err := auditLogger.Reader(ctx, time.Time{}, time.Now().Add(time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)
	// We expect 3 events.
	require.Len(t, events, 3, "should preserve all lines, including corrupt ones")

	// 1. First event: u1 (TS=100)
	assert.Equal(t, "u1", events[0].Data.Session.UserID)
	assert.Equal(t, int64(100), events[0].TimeMillis)

	// 2. Second event: Corrupt
	// It should inherit the timestamp of the previous valid event (100)
	require.Equal(t, "migration_parse_error", events[1].Type)
	assert.Equal(t, int64(100), events[1].TimeMillis, "corrupt event should inherit last valid timestamp")
	// The file reader implementation (log_file.go) appends a newline to the data.
	assert.Equal(t, "THIS_IS_BROKEN_JSON\n", events[1].Data.MigrationError.RawData)

	// 3. Third event: u2 (TS=200)
	assert.Equal(t, "u2", events[2].Data.Session.UserID)
	assert.Equal(t, int64(200), events[2].TimeMillis)

	// 6. Verify Files REMOVED
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "log files should be removed as corrupt lines were safely archived in DB")
}

func readAllEvents(t *testing.T, rdr auditlog.Reader) []auditlog.Event {
	t.Helper()

	var events []auditlog.Event

	for {
		data, err := rdr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return events
			}

			require.NoError(t, err)
		}

		var evt auditlog.Event

		err = json.Unmarshal(data, &evt)
		require.NoError(t, err)

		events = append(events, evt)
	}
}
