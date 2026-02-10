// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auditlogsqlite_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogsqlite"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestReadWrite(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	store, _ := setupStore(ctx, t, logger)

	// 1. Verify HasData is false initially
	hasData, err := store.HasData(ctx)
	require.NoError(t, err)
	assert.False(t, hasData)

	// 2. Write Events
	event1 := auditlog.MakeEvent("create", "test.resource", "test-id", &auditlog.Data{
		Session: auditlog.Session{UserID: "user-1"},
	})
	// ensure deterministic time for tests
	event1.TimeMillis = time.Now().Add(-1 * time.Hour).UnixMilli()

	event2 := auditlog.MakeEvent("update", "test.resource", "test-id", &auditlog.Data{
		Session: auditlog.Session{UserID: "user-2"},
	})
	event2.TimeMillis = time.Now().UnixMilli()

	require.NoError(t, store.Write(ctx, event1))
	require.NoError(t, store.Write(ctx, event2))

	// 3. Verify HasData is true
	hasData, err = store.HasData(ctx)
	require.NoError(t, err)
	assert.True(t, hasData)

	t.Run("read all", func(t *testing.T) {
		t.Parallel()

		rdr, err := store.Reader(ctx, time.Time{}, time.Now().Add(time.Hour))
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, rdr.Close())
		})

		events := readAllEvents(t, rdr)
		require.Len(t, events, 2)

		assert.Equal(t, event1.Type, events[0].Type)
		assert.Equal(t, event1.Data.Session.UserID, events[0].Data.Session.UserID)

		assert.Equal(t, event2.Type, events[1].Type)
		assert.Equal(t, event2.Data.Session.UserID, events[1].Data.Session.UserID)
	})
}

func TestRemove(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	store, _ := setupStore(ctx, t, logger)

	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	// 1. Write 3 events at different times
	times := []time.Time{
		baseTime.Add(-2 * time.Hour),
		baseTime.Add(-1 * time.Hour),
		baseTime,
	}

	for _, ts := range times {
		evt := auditlog.Event{
			Type:       "test",
			TimeMillis: ts.UnixMilli(),
			Data:       &auditlog.Data{Session: auditlog.Session{UserID: "remove-test"}},
		}
		require.NoError(t, store.Write(ctx, evt))
	}

	// 2. Remove the first two events
	start := baseTime.Add(-3 * time.Hour)
	end := baseTime.Add(-30 * time.Minute)

	err := store.Remove(ctx, start, end)
	require.NoError(t, err)

	// 3. Verify only the last event remains
	rdr, err := store.Reader(ctx, time.Time{}, time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)
	require.Len(t, events, 1)
	assert.Equal(t, times[2].UnixMilli(), events[0].TimeMillis)
}

func TestRemoveByRetentionPeriod(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	store, _ := setupStore(ctx, t, logger)

	now := time.Now()
	retentionPeriod := 7 * 24 * time.Hour // 7 days

	// Write events at various ages relative to now.
	eventTimes := []time.Time{
		now.Add(-30 * 24 * time.Hour),        // 30 days ago — should be removed
		now.Add(-14 * 24 * time.Hour),        // 14 days ago — should be removed
		now.Add(-7*24*time.Hour - time.Hour), // 7 days + 1 hour ago — should be removed
		now.Add(-6 * 24 * time.Hour),         // 6 days ago — should remain
		now.Add(-1 * 24 * time.Hour),         // 1 day ago — should remain
		now.Add(-1 * time.Hour),              // 1 hour ago — should remain
	}

	for i, ts := range eventTimes {
		evt := auditlog.Event{
			Type:       "retention-test",
			TimeMillis: ts.UnixMilli(),
			Data:       &auditlog.Data{Session: auditlog.Session{UserID: fmt.Sprintf("user-%d", i)}},
		}
		require.NoError(t, store.Write(ctx, evt))
	}

	// Simulate RunCleanup: remove from epoch to (now - retentionPeriod).
	err := store.Remove(ctx, time.Unix(0, 0), now.Add(-retentionPeriod))
	require.NoError(t, err)

	// Verify only the 3 recent events remain.
	rdr, err := store.Reader(ctx, time.Time{}, now.Add(time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)
	require.Len(t, events, 3)

	// The remaining events should be the 3 newest, ordered by time.
	assert.Equal(t, eventTimes[3].UnixMilli(), events[0].TimeMillis)
	assert.Equal(t, eventTimes[4].UnixMilli(), events[1].TimeMillis)
	assert.Equal(t, eventTimes[5].UnixMilli(), events[2].TimeMillis)
}

func TestRemoveByMaxSize(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	// First, create a store without size limit to populate data and measure table size.
	seedStore, db := setupStore(ctx, t, logger)

	fakeEventCount := 50
	fakeTimeMillis := func(i int) int64 { return int64(i) * 1000 }

	// Write 50 events with known data.
	for i := range fakeEventCount {
		evt := auditlog.Event{
			Type:       "size-test",
			TimeMillis: fakeTimeMillis(i),
			Data: &auditlog.Data{
				Session: auditlog.Session{
					UserID: fmt.Sprintf("user-%d", i),
					Email:  fmt.Sprintf("user-%d@example.com", i),
				},
			},
		}
		require.NoError(t, seedStore.Write(ctx, evt))
	}

	// Get the current table size from dbstat.
	conn, err := db.Take(ctx)
	require.NoError(t, err)

	var tableSize int64

	err = sqlitex.ExecuteTransient(conn, "SELECT SUM(pgsize) FROM dbstat WHERE name = 'audit_logs'", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *zombiesqlite.Stmt) error {
			tableSize = stmt.ColumnInt64(0)

			return nil
		},
	})
	require.NoError(t, err)

	db.Put(conn)

	require.Greater(t, tableSize, int64(0), "table size should be > 0 after writing events")

	// Create a new store on the same DB with maxSize=half, probability=1.0 to always trigger cleanup.
	maxSize := uint64(tableSize / 2)

	sizedStore, err := auditlogsqlite.NewStore(ctx, db, 5*time.Second, maxSize, 1.0, logger)
	require.NoError(t, err)

	// Write one more event to trigger cleanup.
	require.NoError(t, sizedStore.Write(ctx, auditlog.Event{
		Type:       "size-test-trigger",
		TimeMillis: fakeTimeMillis(fakeEventCount),
		Data:       &auditlog.Data{},
	}))

	// Verify some events were deleted.
	rdr, err := sizedStore.Reader(ctx, time.Time{}, time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)
	assert.Less(t, len(events), fakeEventCount, "some events should have been deleted")
	assert.Greater(t, len(events), 0, "not all events should be deleted")

	// Verify that the remaining events are the newest ones (oldest were deleted).
	for i, evt := range events {
		if i > 0 {
			assert.Greater(t, evt.TimeMillis, events[i-1].TimeMillis, "remaining events should be ordered by time")
		}
	}

	// The last event should be the newest one.
	assert.Equal(t, fakeTimeMillis(fakeEventCount), events[len(events)-1].TimeMillis)
}

func TestRemoveByMaxSizeUnlimited(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	// maxSize=0 means unlimited, probability=1.0 to ensure the check runs on every write.
	store, _ := setupStoreWithOpts(ctx, t, logger, 0, 1.0)

	// Write a few events (timestamps start at 1000ms to avoid matching time-based deletion range).
	for i := range 5 {
		evt := auditlog.Event{
			Type:       "unlimited-test",
			TimeMillis: int64(i+1) * 1000,
			Data:       &auditlog.Data{},
		}
		require.NoError(t, store.Write(ctx, evt))
	}

	rdr, err := store.Reader(ctx, time.Time{}, time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)
	assert.Len(t, events, 5, "all events should remain when maxSize is 0")
}

func TestRemoveByMaxSizeBatchCap(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	// Seed 1500 events into a store with no size limit.
	seedStore, db := setupStore(ctx, t, logger)

	const totalEvents = 1500

	for i := range totalEvents {
		evt := auditlog.Event{
			Type:       "batch-cap-test",
			TimeMillis: int64(i) * 1000,
			Data: &auditlog.Data{
				Session: auditlog.Session{
					UserID: fmt.Sprintf("user-%d", i),
					Email:  fmt.Sprintf("user-%d@example.com", i),
				},
			},
		}
		require.NoError(t, seedStore.Write(ctx, evt))
	}

	// Create a new store on the same DB with maxSize=1 (effectively 0) and probability=1.0.
	// This means all rows exceed the limit, but the batch cap (1000) should prevent
	// deleting them all in a single cleanup pass.
	sizedStore, err := auditlogsqlite.NewStore(ctx, db, 5*time.Second, 1, 1.0, logger)
	require.NoError(t, err)

	// Write one event to trigger a single cleanup pass.
	require.NoError(t, sizedStore.Write(ctx, auditlog.Event{
		Type:       "batch-cap-trigger",
		TimeMillis: int64(totalEvents) * 1000,
		Data:       &auditlog.Data{},
	}))

	// Count remaining rows. The batch cap is 1000, so roughly 1000 should have been
	// deleted, leaving ~501 rows (1500 - 1000 + 1 trigger event).
	rdr, err := sizedStore.Reader(ctx, time.Time{}, time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)

	// The cap prevents deleting everything at once: at least 400 rows must survive.
	assert.Greater(t, len(events), 400, "batch cap should prevent deleting all excess rows in one pass")
	// But some rows should have been deleted.
	assert.Less(t, len(events), totalEvents, "some events should have been deleted")

	// Verify remaining events are the newest (oldest were deleted first).
	for i := range len(events) - 1 {
		assert.Greater(t, events[i+1].TimeMillis, events[i].TimeMillis, "remaining events should be ordered by time")
	}
}

func TestReaderParameters(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	store, _ := setupStore(ctx, t, logger)

	base := time.Date(2023, 5, 20, 10, 0, 0, 0, time.UTC)

	for i := range 3 {
		evt := auditlog.Event{
			Type:       "range-test",
			TimeMillis: base.Add(time.Duration(i) * 10 * time.Minute).UnixMilli(),
			Data:       &auditlog.Data{},
		}
		require.NoError(t, store.Write(ctx, evt))
	}

	tests := []struct {
		start     time.Time
		end       time.Time
		name      string
		wantCount int
	}{
		{
			name:      "All Inclusive",
			start:     base.Add(-1 * time.Minute),
			end:       base.Add(1 * time.Hour),
			wantCount: 3,
		},
		{
			name:      "First Only",
			start:     base,
			end:       base.Add(5 * time.Minute),
			wantCount: 1,
		},
		{
			name:      "Last Only",
			start:     base.Add(15 * time.Minute),
			end:       base.Add(30 * time.Minute),
			wantCount: 1,
		},
		{
			name:      "Middle Only",
			start:     base.Add(5 * time.Minute),
			end:       base.Add(15 * time.Minute),
			wantCount: 1,
		},
		{
			name:      "None (Before)",
			start:     base.Add(-1 * time.Hour),
			end:       base.Add(-10 * time.Minute),
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rdr, err := store.Reader(ctx, tt.start, tt.end)
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, rdr.Close())
			})

			events := readAllEvents(t, rdr)
			assert.Len(t, events, tt.wantCount)
		})
	}
}

// TestExtractedColumns verifies that the denormalized/extracted columns (actor_email, cluster_id, etc.)
// are correctly populated in SQLite, enabling future indexing and direct queries.
func TestExtractedColumns(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	store, db := setupStore(ctx, t, logger)

	// 1. Event with Actor + Machine ID + Cluster Label
	evt1 := auditlog.MakeEvent("create", "omni.MachineSetNode", "machine-123", &auditlog.Data{
		Session: auditlog.Session{
			UserID: "user-abc",
			Email:  "user@example.com",
		},
		MachineSetNode: &auditlog.MachineSetNode{
			ID:        "machine-123",
			ClusterID: "cluster-id",
		},
	})
	evt1.TimeMillis = 1000
	require.NoError(t, store.Write(ctx, evt1))

	// 2. Event with Cluster ID (via Cluster struct)
	evt2 := auditlog.MakeEvent("update", "omni.Cluster", "cluster-xyz", &auditlog.Data{
		Cluster: &auditlog.Cluster{
			ID: "cluster-xyz",
		},
	})
	evt2.TimeMillis = 2000
	require.NoError(t, store.Write(ctx, evt2))

	// 3. Event with Cluster ID (via K8SAccess)
	evt3 := auditlog.MakeEvent("access", "k8s", "", &auditlog.Data{
		K8SAccess: &auditlog.K8SAccess{
			ClusterName: "cluster-k8s",
		},
	})
	evt3.TimeMillis = 3000
	require.NoError(t, store.Write(ctx, evt3))

	// 4. Event with nil Data (Nil Safety Check)
	evt4 := auditlog.Event{
		Type:       "nil_data_check",
		TimeMillis: 4000,
		Data:       nil,
	}
	require.NoError(t, store.Write(ctx, evt4))

	// 5. Verify Raw Columns in DB
	// We query the DB directly because these columns are not exposed via the Reader.
	conn, err := db.Take(ctx)
	require.NoError(t, err)

	t.Cleanup(func() { db.Put(conn) })

	q, err := sqlitexx.NewQuery(conn, "SELECT actor_email, resource_id, cluster_id FROM audit_logs ORDER BY event_ts_ms ASC")
	require.NoError(t, err)

	idx := 0

	for result, err := range q.QueryIter() {
		require.NoError(t, err)

		switch idx {
		case 0:
			// Row 1: Check Actor & Resource ID & Cluster ID from Labels
			assert.Equal(t, "user@example.com", result.GetText("actor_email"))
			assert.Equal(t, "machine-123", result.GetText("resource_id"))
			assert.Equal(t, "cluster-id", result.GetText("cluster_id"), "cluster_id should be extracted from machine labels")
		case 1:
			// Row 2: Check Cluster ID (from Cluster struct)
			assert.Equal(t, "cluster-xyz", result.GetText("resource_id"), "cluster.ID is considered the Resource ID")
			assert.Equal(t, "cluster-xyz", result.GetText("cluster_id"), "cluster.ID is also extracted as Cluster ID")
		case 2:
			// Row 3: Check Cluster ID (from K8SAccess)
			assert.True(t, result.IsNull("resource_id"))
			assert.Equal(t, "cluster-k8s", result.GetText("cluster_id"))
		case 3:
			// Row 4: Nil Data Check
			assert.True(t, result.IsNull("actor_email"))
			assert.True(t, result.IsNull("resource_id"))
			assert.True(t, result.IsNull("cluster_id"))
		default:
			require.Failf(t, "unexpected row", " index %d", idx)
		}

		idx++
	}
}

func setupStore(ctx context.Context, t *testing.T, logger *zap.Logger) (*auditlogsqlite.Store, *sqlitex.Pool) {
	return setupStoreWithOpts(ctx, t, logger, 0, 0)
}

func setupStoreWithOpts(ctx context.Context, t *testing.T, logger *zap.Logger, maxSize uint64, cleanupProbability float64) (*auditlogsqlite.Store, *sqlitex.Pool) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	conf := config.Default().Storage.Sqlite
	conf.SetPath(path)

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	store, err := auditlogsqlite.NewStore(ctx, db, 5*time.Second, maxSize, cleanupProbability, logger)
	require.NoError(t, err)

	return store, db
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
