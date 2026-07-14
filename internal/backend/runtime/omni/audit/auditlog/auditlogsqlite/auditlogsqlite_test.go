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
	"golang.org/x/sync/errgroup"
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

		rdr, err := store.Reader(ctx, auditlog.ReadFilters{End: time.Now().Add(time.Hour)})
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
	rdr, err := store.Reader(ctx, auditlog.ReadFilters{End: time.Now().Add(24 * time.Hour)})
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
	rdr, err := store.Reader(ctx, auditlog.ReadFilters{End: now.Add(time.Hour)})
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

	err = sqlitex.ExecuteTransient(conn, "SELECT COALESCE(SUM(d.pgsize), 0) FROM dbstat d JOIN sqlite_master m ON d.name = m.name WHERE m.tbl_name = 'audit_logs'", &sqlitex.ExecOptions{
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

	sizedStore, err := auditlogsqlite.NewStore(ctx, db, 30*time.Second, maxSize, 1.0, logger)
	require.NoError(t, err)

	// Write one more event to trigger cleanup.
	require.NoError(t, sizedStore.Write(ctx, auditlog.Event{
		Type:       "size-test-trigger",
		TimeMillis: fakeTimeMillis(fakeEventCount),
		Data:       &auditlog.Data{},
	}))

	// Verify some events were deleted.
	rdr, err := sizedStore.Reader(ctx, auditlog.ReadFilters{End: time.Now().Add(24 * time.Hour)})
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

	rdr, err := store.Reader(ctx, auditlog.ReadFilters{End: time.Now().Add(24 * time.Hour)})
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
	_, db := setupStore(ctx, t, logger)

	const totalEvents = 1500

	require.NoError(t, func() (err error) {
		var conn *zombiesqlite.Conn

		if conn, err = db.Take(ctx); err != nil {
			return err
		}

		defer db.Put(conn)

		endTx, err := sqlitex.ImmediateTransaction(conn)
		if err != nil {
			return err
		}

		defer endTx(&err)

		insertQuery := fmt.Sprintf("INSERT INTO %s (%s, %s, %s) VALUES ($type, $ts, $data)",
			auditlogsqlite.TableName, "event_type", "event_ts_ms", "event_data")

		for i := range totalEvents {
			data, mErr := json.Marshal(&auditlog.Data{
				Session: auditlog.Session{
					UserID: fmt.Sprintf("user-%d", i),
					Email:  fmt.Sprintf("user-%d@example.com", i),
				},
			})
			if mErr != nil {
				return mErr
			}

			q, qErr := sqlitexx.NewQuery(conn, insertQuery)
			if qErr != nil {
				return qErr
			}

			if err = q.
				BindString("$type", "batch-cap-test").
				BindInt64("$ts", int64(i)*1000).
				BindBytes("$data", data).
				Exec(); err != nil {
				return err
			}
		}

		return nil
	}())

	// Create a new store on the same DB with maxSize=1 (effectively 0) and probability=1.0.
	// This means all rows exceed the limit, but the batch cap (1000) should prevent
	// deleting them all in a single cleanup pass.
	sizedStore, err := auditlogsqlite.NewStore(ctx, db, 30*time.Second, 1, 1.0, logger)
	require.NoError(t, err)

	// Write one event to trigger a single cleanup pass.
	require.NoError(t, sizedStore.Write(ctx, auditlog.Event{
		Type:       "batch-cap-trigger",
		TimeMillis: int64(totalEvents) * 1000,
		Data:       &auditlog.Data{},
	}))

	// Count remaining rows. The batch cap is 1000, so roughly 1000 should have been
	// deleted, leaving ~501 rows (1500 - 1000 + 1 trigger event).
	rdr, err := sizedStore.Reader(ctx, auditlog.ReadFilters{End: time.Now().Add(24 * time.Hour)})
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

func TestRemoveBatching(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	var totalCleaned int

	store, db := setupStoreWithOpts(ctx, t, logger, 0, 0, auditlogsqlite.WithCleanupCallback(func(n int) { totalCleaned += n }))

	const (
		inRangeCount  = 1500
		outRangeCount = 10
	)

	rangeStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	require.NoError(t, func() (err error) {
		var conn *zombiesqlite.Conn

		if conn, err = db.Take(ctx); err != nil {
			return err
		}

		defer db.Put(conn)

		endTx, err := sqlitex.ImmediateTransaction(conn)
		if err != nil {
			return err
		}

		defer endTx(&err)

		insertQuery := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES ($ts, $data)",
			auditlogsqlite.TableName, "event_ts_ms", "event_data")

		insert := func(ts int64) error {
			q, qErr := sqlitexx.NewQuery(conn, insertQuery)
			if qErr != nil {
				return qErr
			}

			return q.BindInt64("$ts", ts).BindBytes("$data", []byte(`{}`)).Exec()
		}

		for i := range inRangeCount {
			if err = insert(rangeStart.Add(time.Duration(i) * time.Second).UnixMilli()); err != nil {
				return err
			}
		}

		for i := range outRangeCount {
			if err = insert(rangeEnd.Add(time.Duration(i+1) * time.Hour).UnixMilli()); err != nil {
				return err
			}
		}

		return nil
	}())

	// Remove all events in range — requires multiple batches (1000 + 500).
	require.NoError(t, store.Remove(ctx, rangeStart, rangeEnd))

	// Verify only out-of-range events remain.
	rdr, err := store.Reader(ctx, auditlog.ReadFilters{End: time.Now().Add(24 * time.Hour)})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	events := readAllEvents(t, rdr)
	assert.Len(t, events, outRangeCount, "only out-of-range events should remain")
	assert.Equal(t, inRangeCount, totalCleaned, "cleanup callback should report all in-range rows deleted")

	// Verify remaining events are all after the range.
	for _, evt := range events {
		assert.Greater(t, evt.TimeMillis, rangeEnd.UnixMilli(), "remaining events should be after the removal range")
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

			rdr, err := store.Reader(ctx, auditlog.ReadFilters{Start: tt.start, End: tt.end})
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

func TestReaderFilters(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	store, _ := setupStore(ctx, t, logger)

	base := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	events := []auditlog.Event{
		{
			Type:         "create",
			ResourceType: "omni.Cluster",
			ResourceID:   "cluster-a",
			TimeMillis:   base.Add(1 * time.Second).UnixMilli(),
			Data: &auditlog.Data{
				Session: auditlog.Session{Email: "alice@example.com"},
				Cluster: &auditlog.Cluster{ID: "cluster-a"},
			},
		},
		{
			Type:         "update",
			ResourceType: "omni.Cluster",
			ResourceID:   "cluster-b",
			TimeMillis:   base.Add(2 * time.Second).UnixMilli(),
			Data: &auditlog.Data{
				Session: auditlog.Session{Email: "bob@example.com"},
				Cluster: &auditlog.Cluster{ID: "cluster-b"},
			},
		},
		{
			Type:         "create",
			ResourceType: "omni.MachineSetNode",
			ResourceID:   "node-1",
			TimeMillis:   base.Add(3 * time.Second).UnixMilli(),
			Data: &auditlog.Data{
				Session:        auditlog.Session{Email: "alice@example.com"},
				MachineSetNode: &auditlog.MachineSetNode{ID: "node-1", ClusterID: "cluster-a"},
			},
		},
	}

	for _, evt := range events {
		require.NoError(t, store.Write(ctx, evt))
	}

	timeFilters := auditlog.ReadFilters{
		Start: base,
		End:   base.Add(1 * time.Minute),
	}

	tests := []struct {
		name      string
		filters   auditlog.ReadFilters
		wantCount int
	}{
		{
			name:      "EventType=create",
			filters:   auditlog.ReadFilters{EventType: auditlog.EventTypeCreate},
			wantCount: 2,
		},
		{
			name:      "EventType=update",
			filters:   auditlog.ReadFilters{EventType: auditlog.EventTypeUpdate},
			wantCount: 1,
		},
		{
			name:      "ResourceType=omni.Cluster",
			filters:   auditlog.ReadFilters{ResourceType: "omni.Cluster"},
			wantCount: 2,
		},
		{
			name:      "ResourceType=omni.MachineSetNode",
			filters:   auditlog.ReadFilters{ResourceType: "omni.MachineSetNode"},
			wantCount: 1,
		},
		{
			name:      "ResourceID=cluster-a",
			filters:   auditlog.ReadFilters{ResourceID: "cluster-a"},
			wantCount: 1,
		},
		{
			name:      "ClusterID=cluster-a",
			filters:   auditlog.ReadFilters{ClusterID: "cluster-a"},
			wantCount: 2,
		},
		{
			name:      "Actor=alice@example.com",
			filters:   auditlog.ReadFilters{Actor: "alice@example.com"},
			wantCount: 2,
		},
		{
			name:      "Actor=bob@example.com",
			filters:   auditlog.ReadFilters{Actor: "bob@example.com"},
			wantCount: 1,
		},
		{
			name:      "Actor=alice + EventType=update (no match)",
			filters:   auditlog.ReadFilters{Actor: "alice@example.com", EventType: auditlog.EventTypeUpdate},
			wantCount: 0,
		},
		{
			name:      "Actor=alice + EventType=create",
			filters:   auditlog.ReadFilters{Actor: "alice@example.com", EventType: auditlog.EventTypeCreate},
			wantCount: 2,
		},
		{
			name:      "Search=alice (matches actor_email)",
			filters:   auditlog.ReadFilters{Search: "alice"},
			wantCount: 2,
		},
		{
			name:      "Search=cluster-b (matches resource_id and cluster_id)",
			filters:   auditlog.ReadFilters{Search: "cluster-b"},
			wantCount: 1,
		},
		{
			name:      "Search=omni.Cluster (matches resource_type)",
			filters:   auditlog.ReadFilters{Search: "omni.Cluster"},
			wantCount: 2,
		},
		{
			name:      "Search=update (matches event_type)",
			filters:   auditlog.ReadFilters{Search: "update"},
			wantCount: 1,
		},
		{
			name:      "Search=node-1 (matches resource_id in event_data JSON)",
			filters:   auditlog.ReadFilters{Search: "node-1"},
			wantCount: 1,
		},
		{
			name:      "Search=no-match",
			filters:   auditlog.ReadFilters{Search: "zzz-no-match-zzz"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := tt.filters
			f.Start = timeFilters.Start
			f.End = timeFilters.End

			rdr, err := store.Reader(ctx, f)
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, rdr.Close())
			})

			got := readAllEvents(t, rdr)
			assert.Len(t, got, tt.wantCount)
		})
	}
}

func setupStore(ctx context.Context, t *testing.T, logger *zap.Logger) (*auditlogsqlite.Store, *sqlitexx.Pool) {
	return setupStoreWithOpts(ctx, t, logger, 0, 0)
}

func setupStoreWithOpts(ctx context.Context, t *testing.T, logger *zap.Logger, maxSize uint64, cleanupProbability float64, opts ...auditlogsqlite.Option) (*auditlogsqlite.Store, *sqlitexx.Pool) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	conf := config.Default().Storage.Sqlite
	conf.SetPath(path)

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	store, err := auditlogsqlite.NewStore(ctx, db, 30*time.Second, maxSize, cleanupProbability, logger, opts...)
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

func TestFollowStartPositions(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t, zaptest.NewLogger(t))

	// empty table: every start resolves to the zero tail
	pos, err := store.FollowStart(ctx, 123)
	require.NoError(t, err)
	assert.Zero(t, pos)

	writeEventAt(ctx, t, store, 100)
	writeEventAt(ctx, t, store, 200)
	writeEventAt(ctx, t, store, 300)

	for _, testCase := range []struct {
		name        string
		startTsMs   int64
		expectedPos int64
	}{
		{name: "zero starts at the tail exactly", startTsMs: 0, expectedPos: 3},
		{name: "before all events starts at the beginning", startTsMs: 1, expectedPos: 0},
		{name: "exact match is inclusive", startTsMs: 200, expectedPos: 1},
		{name: "between events starts at the next one", startTsMs: 250, expectedPos: 2},
		{name: "after all events starts at the tail", startTsMs: 400, expectedPos: 3},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			startPos, startErr := store.FollowStart(ctx, testCase.startTsMs)
			require.NoError(t, startErr)
			assert.Equal(t, testCase.expectedPos, startPos)
		})
	}
}

func TestFollowBatchPagination(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t, zaptest.NewLogger(t))

	for i := range 5 {
		writeEventAt(ctx, t, store, int64(100*(i+1)))
	}

	entries, err := store.FollowBatch(ctx, 0, 2)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, int64(1), entries[0].ID)
	assert.Equal(t, int64(2), entries[1].ID)

	// payloads are newline-terminated JSON events
	var event auditlog.Event

	require.NoError(t, json.Unmarshal(entries[0].Payload, &event))
	assert.Equal(t, int64(100), event.TimeMillis)
	assert.Equal(t, byte('\n'), entries[0].Payload[len(entries[0].Payload)-1])

	entries, err = store.FollowBatch(ctx, 2, 2)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, int64(3), entries[0].ID)
	assert.Equal(t, int64(4), entries[1].ID)

	// a short batch means the log is exhausted for now
	entries, err = store.FollowBatch(ctx, 4, 2)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, int64(5), entries[0].ID)

	entries, err = store.FollowBatch(ctx, 5, 2)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCleanupSparesNewestEvent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t, zaptest.NewLogger(t))

	writeEventAt(ctx, t, store, 100) // id 1
	writeEventAt(ctx, t, store, 200) // id 2

	// a removal spanning every event spares the one with the highest id: SQLite would
	// otherwise hand out its id again, corrupting the follow positions
	require.NoError(t, store.Remove(ctx, time.UnixMilli(0), time.UnixMilli(250)))

	entries, err := store.FollowBatch(ctx, 0, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, int64(2), entries[0].ID, "the newest event survives a full-range removal")

	writeEventAt(ctx, t, store, 300)

	// ids keep increasing across the cleanup, so a follower position stays exact
	entries, err = store.FollowBatch(ctx, 2, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, int64(3), entries[0].ID)

	// an exhausted but existing position is not lost, it just has nothing new
	entries, err = store.FollowBatch(ctx, 3, 10)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestRemoveBySizeSparesNewestEvent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	// a tiny size budget with certain cleanup: every write triggers a size cleanup that
	// wants to delete far more rows than exist
	store, _ := setupStoreWithOpts(ctx, t, zaptest.NewLogger(t), 1, 1.0)

	writeEventAt(ctx, t, store, 100) // id 1
	writeEventAt(ctx, t, store, 200) // id 2, its write deletes everything the clamp allows

	entries, err := store.FollowBatch(ctx, 0, 10)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "the newest event must survive the size cleanup")
	assert.Equal(t, int64(2), entries[len(entries)-1].ID)

	// ids keep increasing across the cleanup, so a follower position stays exact
	writeEventAt(ctx, t, store, 300)

	entries, err = store.FollowBatch(ctx, 2, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, int64(3), entries[0].ID)
}

func TestFollowPositionLost(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t, zaptest.NewLogger(t))

	writeEventAt(ctx, t, store, 100) // id 1

	// a position beyond every stored event cannot happen through cleanup (the newest event
	// survives it), only through the database being replaced, e.g. restored from a backup:
	// reading from it reports the position as lost instead of waiting blindly
	_, err := store.FollowBatch(ctx, 5, 10)
	require.ErrorIs(t, err, auditlog.ErrFollowPositionLost)
}

func TestFollowSinglePoolConnection(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	path := filepath.Join(t.TempDir(), "test.db")

	conf := config.Default().Storage.Sqlite
	conf.SetPath(path)
	conf.SetPoolSize(1)

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	store, err := auditlogsqlite.NewStore(ctx, db, 30*time.Second, 0, 0, zaptest.NewLogger(t))
	require.NoError(t, err)

	// every operation must release the single connection, otherwise the next call deadlocks
	writeEventAt(ctx, t, store, 100)

	pos, err := store.FollowStart(ctx, 1)
	require.NoError(t, err)
	assert.Zero(t, pos)

	entries, err := store.FollowBatch(ctx, 0, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	writeEventAt(ctx, t, store, 200)

	entries, err = store.FollowBatch(ctx, 1, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestFollowCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t, zaptest.NewLogger(t))

	writeEventAt(ctx, t, store, 100)

	canceledCtx, cancelNow := context.WithCancel(ctx)
	cancelNow()

	_, err := store.FollowStart(canceledCtx, 1)
	require.Error(t, err)

	_, err = store.FollowBatch(canceledCtx, 0, 10)
	require.Error(t, err)
}

func TestFollowConcurrentWrites(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t, zaptest.NewLogger(t))

	const numEvents = 200

	var eg errgroup.Group

	eg.Go(func() error {
		for i := range numEvents {
			if err := store.Write(ctx, auditlog.Event{Type: "create", TimeMillis: int64(i + 1)}); err != nil {
				return err
			}
		}

		return nil
	})

	// follow the way the server does: read batches, advance to the watermark on a short batch,
	// and verify that no id is ever skipped
	eg.Go(func() error {
		var (
			pos      int64
			received []int64
		)

		for len(received) < numEvents {
			if ctx.Err() != nil {
				return fmt.Errorf("timed out after receiving %d events", len(received))
			}

			entries, err := store.FollowBatch(ctx, pos, 16)
			if err != nil {
				return err
			}

			for _, entry := range entries {
				received = append(received, entry.ID)
			}

			if len(entries) > 0 {
				pos = entries[len(entries)-1].ID
			}
		}

		for i, id := range received {
			if id != int64(i+1) {
				return fmt.Errorf("received id %d at position %d", id, i)
			}
		}

		return nil
	})

	require.NoError(t, eg.Wait())
}

func TestFollowSubscribeWakesOnWrite(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t, zaptest.NewLogger(t))

	wakeCh, unsubscribe := store.FollowSubscribe()
	defer unsubscribe()

	otherWakeCh, otherUnsubscribe := store.FollowSubscribe()
	defer otherUnsubscribe()

	select {
	case <-wakeCh:
		t.Fatal("no wakeup must arrive before a write")
	default:
	}

	writeEventAt(ctx, t, store, 100)

	select {
	case <-wakeCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the write wakeup")
	}

	// every subscriber gets its own wakeup
	select {
	case <-otherWakeCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the write wakeup of the second subscriber")
	}

	entries, err := store.FollowBatch(ctx, 0, 16)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// writes arriving while the follower is busy collapse into one pending wakeup
	writeEventAt(ctx, t, store, 200)
	writeEventAt(ctx, t, store, 300)

	select {
	case <-wakeCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the write wakeup")
	}

	entries, err = store.FollowBatch(ctx, entries[len(entries)-1].ID, 16)
	require.NoError(t, err)
	require.Len(t, entries, 2, "one wakeup is enough, the scan picks up everything written")

	unsubscribe()

	writeEventAt(ctx, t, store, 400)

	select {
	case <-wakeCh:
		t.Fatal("no wakeup must arrive after unsubscribing")
	default:
	}
}

func writeEventAt(ctx context.Context, t *testing.T, store *auditlogsqlite.Store, tsMillis int64) {
	t.Helper()

	event := auditlog.MakeEvent("create", "test.resource", "test-id", &auditlog.Data{
		Session: auditlog.Session{UserID: "user-1"},
	})
	event.TimeMillis = tsMillis

	require.NoError(t, store.Write(ctx, event))
}
