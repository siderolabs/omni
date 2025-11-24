// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlitelog_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-circular/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/circularlog"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/sqlitelog"
)

func TestReadWrite(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	db := setupDB(t)

	logger := zaptest.NewLogger(t)

	sqliteStore, err := sqlitelog.NewStore(ctx, config.Default().Logs.Machine.SQLite, db, "test-1", logger)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, sqliteStore.Close())
	})

	defaultConfig := config.Default()

	compressor, err := zstd.NewCompressor()
	require.NoError(t, err)

	circularStore, err := circularlog.NewStore(&defaultConfig.Logs.Machine, "test-1", compressor, zaptest.NewLogger(t))
	require.NoError(t, err)

	numLines := 1000

	for i := range numLines {
		require.NoError(t, sqliteStore.WriteLine(ctx, fmt.Appendf(nil, "Hello, World %d!", i)))
		require.NoError(t, circularStore.WriteLine(ctx, fmt.Appendf(nil, "Hello, World %d!", i)))
	}

	t.Run("read all", func(t *testing.T) {
		t.Parallel()

		testRead(ctx, t, sqliteStore, circularStore, numLines, 0)
	})

	t.Run("tail", func(t *testing.T) {
		t.Parallel()

		testRead(ctx, t, sqliteStore, circularStore, 100, 100)
	})
}

func testRead(ctx context.Context, t *testing.T, sqliteStore, circularStore logstore.LogStore, expectedLines, tailLines int) {
	sqliteReader, err := sqliteStore.Reader(tailLines, false)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, sqliteReader.Close())
	})

	sqliteLines := readAllLines(ctx, t, sqliteReader)

	circularReader, err := circularStore.Reader(tailLines, false)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, circularReader.Close())
	})

	circularLines := readAllLines(ctx, t, circularReader)

	assert.Len(t, sqliteLines, expectedLines)
	assert.Equal(t, circularLines, sqliteLines)
}

func readAllLines(ctx context.Context, t *testing.T, rdr logstore.LineReader) []string {
	var lines []string

	for {
		line, err := rdr.ReadLine(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return lines
			}

			require.NoError(t, err)
		}

		lines = append(lines, string(line))
	}
}

func readLines(ctx context.Context, t *testing.T, rdr logstore.LineReader, ch chan<- string) {
	for {
		line, err := rdr.ReadLine(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			require.NoError(t, err)
		}

		select {
		case ch <- string(line):
		case <-ctx.Done():
			return
		}
	}
}

func TestFollow(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	db := setupDB(t)

	logger := zaptest.NewLogger(t)

	sqliteStore, err := sqlitelog.NewStore(ctx, config.Default().Logs.Machine.SQLite, db, "test-1", logger)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, sqliteStore.Close())
	})

	require.NoError(t, sqliteStore.WriteLine(ctx, []byte("Hello, World 1!")))
	require.NoError(t, sqliteStore.WriteLine(ctx, []byte("Hello, World 2!")))

	rdr, err := sqliteStore.Reader(0, true)
	require.NoError(t, err)

	var wg sync.WaitGroup

	lineCh := make(chan string)

	wg.Go(func() {
		readLines(ctx, t, rdr, lineCh)
	})

	assertLine(ctx, t, lineCh, "Hello, World 1!")
	assertLine(ctx, t, lineCh, "Hello, World 2!")

	require.NoError(t, sqliteStore.WriteLine(ctx, []byte("Hello, World 3!")))
	require.NoError(t, sqliteStore.WriteLine(ctx, []byte("Hello, World 4!")))

	assertLine(ctx, t, lineCh, "Hello, World 3!")
	assertLine(ctx, t, lineCh, "Hello, World 4!")

	cancel()

	wg.Wait()
}

func assertLine(ctx context.Context, t *testing.T, lineCh <-chan string, expected string) {
	select {
	case line := <-lineCh:
		assert.Equal(t, expected, line)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}
}

// TestWriteBatchSize verifies buffering behavior by observing side effects on the DB.
func TestWriteBatchSize(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	db := setupDB(t)
	logger := zaptest.NewLogger(t)

	id := "batch-test-bb"
	store, err := sqlitelog.NewStore(ctx, config.Default().Logs.Machine.SQLite, db, id, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	tableName := fmt.Sprintf("logs_%s", id)

	// Write a payload slightly smaller than the batch limit (60KB <= 64KB) - should stay in memory
	largePayload := strings.Repeat("a", 60*1024)
	require.NoError(t, store.WriteLine(ctx, []byte(largePayload)))

	// Check DB state - should be empty
	var count int
	require.NoError(t, db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM %q", tableName)).Scan(&count))
	assert.Equal(t, 0, count, "Database should be empty (0 rows) because 60KB < 64KB threshold")

	// 2. Write enough data to tip over the limit (60KB + 5KB > 64KB).
	smallPayload := strings.Repeat("b", 5*1024)
	require.NoError(t, store.WriteLine(ctx, []byte(smallPayload)))

	// Check DB state again
	require.NoError(t, db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM %q", tableName)).Scan(&count))
	assert.Equal(t, 2, count, "Database should contain 2 rows after exceeding 64KB threshold")
}

// TestHybridReader verifies that the Reader can fetch data that hasn't been flushed to DB yet.
func TestHybridReader(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	db := setupDB(t)
	logger := zaptest.NewLogger(t)

	id := "hybrid-test-bb"

	store, err := sqlitelog.NewStore(ctx, config.Default().Logs.Machine.SQLite, db, id, logger)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, store.Close()) })

	tableName := fmt.Sprintf("logs_%s", id)

	// 1. Write data that is small enough to stay in memory (Buffer).
	liveMsg := "sitting-in-buffer"

	require.NoError(t, store.WriteLine(ctx, []byte(liveMsg)))

	// Verify it is not in the DB.
	var count int
	require.NoError(t, db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM %q", tableName)).Scan(&count))
	require.Equal(t, 0, count, "Pre-condition: Data must not be in DB for this test to be valid")

	// 2. Create a Reader.
	// Since the DB is empty, if this returns data, it MUST be coming from the memory buffer.
	reader, err := store.Reader(0, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })

	// 3. Read and Verify.
	line, err := reader.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, liveMsg, string(line), "Reader should retrieve data from memory buffer")

	// Expect EOF
	_, err = reader.ReadLine(ctx)
	assert.ErrorIs(t, err, io.EOF)
}

// TestReaderBatching verifies that the reader fetches data lazily by closing the database connection halfway through.
func TestReaderBatching(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	db := setupDB(t)
	logger := zaptest.NewLogger(t)

	store, err := sqlitelog.NewStore(ctx, config.Default().Logs.Machine.SQLite, db, "lazy-load-test", logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// Write more lines than one batch (512)
	totalLines := 600
	for i := range totalLines {
		require.NoError(t, store.WriteLine(ctx, fmt.Appendf(nil, "line-%d", i)))
	}

	// Force flush to ensure data is in DB and not in the Store's memory buffer.
	require.NoError(t, store.Close())

	// Re-open the store.
	store, err = sqlitelog.NewStore(ctx, config.Default().Logs.Machine.SQLite, db, "lazy-load-test", logger)
	require.NoError(t, err)

	reader, err := store.Reader(0, false)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reader.Close())
	})

	// 2. Consume exactly one batch (512 lines).
	knownBatchSize := 512
	for i := range knownBatchSize {
		line, lineErr := reader.ReadLine(ctx)
		require.NoError(t, lineErr)
		require.Equal(t, fmt.Sprintf("line-%d", i), string(line))
	}

	// Close the underlying database connection.
	require.NoError(t, db.Close())

	// 4. Try to read line 513 - it should fail because reader will try to fetch the next batch from the closed DB.
	_, err = reader.ReadLine(ctx)

	assert.ErrorContains(t, err, "sql: database is closed")
}

// setupDB helper handles the standard SQLite test setup.
func setupDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	db, err := sqlitelog.OpenDB(path)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	return db
}
