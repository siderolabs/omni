// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlite_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func execSQL(t *testing.T, db *sqlitex.Pool, sql string) {
	t.Helper()

	conn, err := db.Take(t.Context())
	require.NoError(t, err)

	defer db.Put(conn)

	require.NoError(t, sqlitex.ExecScript(conn, sql))
}

type fakeSQLState struct {
	state.CoreState
	size int64
}

func (f *fakeSQLState) DBSize(context.Context) (int64, error) {
	return f.size, nil
}

func setupMetrics(t *testing.T, db *sqlitex.Pool, cosiState state.CoreState, opts ...sqlite.MetricsOption) *prometheus.Registry {
	t.Helper()

	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	registry.MustRegister(sqlite.NewMetrics(db, cosiState, logger, opts...))

	return registry
}

func TestMetricsCollect(t *testing.T) {
	t.Parallel()

	db, st := setupTestDB(t)

	execSQL(t, db, `
		CREATE TABLE machine_logs (id INTEGER PRIMARY KEY, machine_id TEXT, message TEXT);
		INSERT INTO machine_logs (machine_id, message) VALUES ('m1', 'log1');
		INSERT INTO machine_logs (machine_id, message) VALUES ('m1', 'log2');
		INSERT INTO machine_logs (machine_id, message) VALUES ('m2', 'log3');

		CREATE TABLE audit_logs (id INTEGER PRIMARY KEY, event TEXT);
		INSERT INTO audit_logs (event) VALUES ('e1');

		CREATE TABLE discovery_service_state (id TEXT PRIMARY KEY, data BLOB);
		INSERT INTO discovery_service_state (id, data) VALUES ('state', x'00');
	`)

	registry := setupMetrics(t, db, &fakeSQLState{CoreState: st, size: 8192})

	// Verify subsystem row counts (state has no row count since it uses sqlState).
	expected := `
		# HELP omni_sqlite_subsystem_row_count Total number of rows across a subsystem's tables.
		# TYPE omni_sqlite_subsystem_row_count gauge
		omni_sqlite_subsystem_row_count{subsystem="audit_logs"} 1
		omni_sqlite_subsystem_row_count{subsystem="discovery"} 1
		omni_sqlite_subsystem_row_count{subsystem="machine_logs"} 3
	`
	assert.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expected), "omni_sqlite_subsystem_row_count"))

	// Verify subsystem sizes: 3 Omni subsystems + 1 state (from sqlState).
	families, err := registry.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() == "omni_sqlite_subsystem_size_bytes" {
			assert.Len(t, f.GetMetric(), 4)

			for _, m := range f.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "subsystem" && lp.GetValue() == "state" {
						assert.Equal(t, float64(8192), m.GetGauge().GetValue())
					}
				}
			}
		}

		if f.GetName() == "omni_sqlite_db_size_bytes" {
			require.Len(t, f.GetMetric(), 1)
			assert.Greater(t, f.GetMetric()[0].GetGauge().GetValue(), float64(0))
		}
	}
}

func TestMetricsCaching(t *testing.T) {
	t.Parallel()

	db, st := setupTestDB(t)

	execSQL(t, db, `
		CREATE TABLE audit_logs (id INTEGER PRIMARY KEY, data TEXT);
		INSERT INTO audit_logs (data) VALUES ('row1');
	`)

	registry := setupMetrics(t, db, &fakeSQLState{CoreState: st})

	// First gather triggers refresh — audit_logs has 1 row.
	expected := `
		# HELP omni_sqlite_subsystem_row_count Total number of rows across a subsystem's tables.
		# TYPE omni_sqlite_subsystem_row_count gauge
		omni_sqlite_subsystem_row_count{subsystem="audit_logs"} 1
		omni_sqlite_subsystem_row_count{subsystem="discovery"} 0
		omni_sqlite_subsystem_row_count{subsystem="machine_logs"} 0
	`
	assert.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expected), "omni_sqlite_subsystem_row_count"))

	// Insert another row.
	execSQL(t, db, `INSERT INTO audit_logs (data) VALUES ('row2')`)

	// Second gather should return cached value (default 60s interval not elapsed).
	assert.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expected), "omni_sqlite_subsystem_row_count"))

	// A new metrics instance with zero interval should see the new row.
	registry2 := setupMetrics(t, db, &fakeSQLState{CoreState: st}, sqlite.WithRefreshInterval(0))

	expected2 := `
		# HELP omni_sqlite_subsystem_row_count Total number of rows across a subsystem's tables.
		# TYPE omni_sqlite_subsystem_row_count gauge
		omni_sqlite_subsystem_row_count{subsystem="audit_logs"} 2
		omni_sqlite_subsystem_row_count{subsystem="discovery"} 0
		omni_sqlite_subsystem_row_count{subsystem="machine_logs"} 0
	`
	assert.NoError(t, testutil.GatherAndCompare(registry2, strings.NewReader(expected2), "omni_sqlite_subsystem_row_count"))
}

func TestMetricsStateSubsystemUsesDBSizer(t *testing.T) {
	t.Parallel()

	db, st := setupTestDB(t)

	registry := setupMetrics(t, db, &fakeSQLState{CoreState: st, size: 42000})

	expected := `
		# HELP omni_sqlite_subsystem_size_bytes Size of a subsystem's tables in the SQLite database in bytes.
		# TYPE omni_sqlite_subsystem_size_bytes gauge
		omni_sqlite_subsystem_size_bytes{subsystem="audit_logs"} 0
		omni_sqlite_subsystem_size_bytes{subsystem="discovery"} 0
		omni_sqlite_subsystem_size_bytes{subsystem="machine_logs"} 0
		omni_sqlite_subsystem_size_bytes{subsystem="state"} 42000
	`
	assert.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expected), "omni_sqlite_subsystem_size_bytes"))
}

func TestMetricsEmptyDB(t *testing.T) {
	t.Parallel()

	db, st := setupTestDB(t)

	registry := setupMetrics(t, db, &fakeSQLState{CoreState: st})

	// All Omni subsystems should report 0 rows since no tables exist.
	expected := `
		# HELP omni_sqlite_subsystem_row_count Total number of rows across a subsystem's tables.
		# TYPE omni_sqlite_subsystem_row_count gauge
		omni_sqlite_subsystem_row_count{subsystem="audit_logs"} 0
		omni_sqlite_subsystem_row_count{subsystem="discovery"} 0
		omni_sqlite_subsystem_row_count{subsystem="machine_logs"} 0
	`
	assert.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expected), "omni_sqlite_subsystem_row_count"))
}

func TestCleanupCallback(t *testing.T) {
	t.Parallel()

	db, st := setupTestDB(t)
	logger := zaptest.NewLogger(t)

	m := sqlite.NewMetrics(db, &fakeSQLState{CoreState: st}, logger)

	registry := prometheus.NewRegistry()
	registry.MustRegister(m)

	cb := m.CleanupCallback(sqlite.SubsystemAuditLogs)

	// No calls yet — counter should not appear in output (no initialized series).
	families, err := registry.Gather()
	require.NoError(t, err)

	for _, f := range families {
		assert.NotEqual(t, "omni_sqlite_cleanup_rows_deleted_total", f.GetName(), "counter should not appear before any callback")
	}

	// Call with 0 — should not initialize the counter.
	cb(0)

	families, err = registry.Gather()
	require.NoError(t, err)

	for _, f := range families {
		assert.NotEqual(t, "omni_sqlite_cleanup_rows_deleted_total", f.GetName(), "counter should not appear after cb(0)")
	}

	// Call with positive value.
	cb(5)
	cb(3)

	expected := `
		# HELP omni_sqlite_cleanup_rows_deleted_total Total rows deleted by cleanup.
		# TYPE omni_sqlite_cleanup_rows_deleted_total counter
		omni_sqlite_cleanup_rows_deleted_total{subsystem="audit_logs"} 8
	`
	assert.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expected), "omni_sqlite_cleanup_rows_deleted_total"))

	// Multiple subsystems.
	mlCb := m.CleanupCallback(sqlite.SubsystemMachineLogs)
	mlCb(10)

	expected2 := `
		# HELP omni_sqlite_cleanup_rows_deleted_total Total rows deleted by cleanup.
		# TYPE omni_sqlite_cleanup_rows_deleted_total counter
		omni_sqlite_cleanup_rows_deleted_total{subsystem="audit_logs"} 8
		omni_sqlite_cleanup_rows_deleted_total{subsystem="machine_logs"} 10
	`
	assert.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expected2), "omni_sqlite_cleanup_rows_deleted_total"))
}

// setupTestDB helper handles the standard SQLite test setup.
func setupTestDB(t *testing.T) (*sqlitex.Pool, state.State) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	conf := config.Default().Storage.Sqlite
	conf.SetPath(path)

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	state := state.WrapCore(namespaced.NewState(inmem.Build))

	return db, state
}
