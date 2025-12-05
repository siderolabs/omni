// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auditlogsqlite_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

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
	store := setupStore(ctx, t, logger)

	// 1. Verify HasData is false initially
	hasData, err := store.HasData(ctx)
	require.NoError(t, err)
	assert.False(t, hasData)

	// 2. Write Events
	event1 := auditlog.MakeEvent("create", "test.resource", &auditlog.Data{
		Session: auditlog.Session{UserID: "user-1"},
	})
	// ensure deterministic time for tests
	event1.TimeMillis = time.Now().Add(-1 * time.Hour).UnixMilli()

	event2 := auditlog.MakeEvent("update", "test.resource", &auditlog.Data{
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
	store := setupStore(ctx, t, logger)

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

func TestReaderParameters(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	store := setupStore(ctx, t, logger)

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

func setupStore(ctx context.Context, t *testing.T, _ *zap.Logger) *auditlogsqlite.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	conf := config.Default().Storage.SQLite
	conf.Path = path

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	store, err := auditlogsqlite.NewStore(ctx, db, 5*time.Second)
	require.NoError(t, err)

	return store
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
