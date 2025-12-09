// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package discovery_test

import (
	"context"
	"database/sql"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/siderolabs/omni/internal/backend/discovery"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestReadWrite(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t)

	expectedData := []byte(`SOME_DATA`)

	wd, err := store.Writer(ctx)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, wd.Close()) })

	n, err := wd.Write(expectedData)
	require.NoError(t, err)
	assert.Equal(t, len(expectedData), n)

	// Explicit close is required to flush the buffer to the DB
	require.NoError(t, wd.Close())

	rdr, err := store.Reader(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	data, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Equal(t, expectedData, data)
}

func TestOverwrite(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t)

	data1 := []byte("v1-data")
	data2 := []byte("v2-data-updated")

	// 1. Write Initial Data
	wd1, err := store.Writer(ctx)
	require.NoError(t, err)
	_, err = wd1.Write(data1)
	require.NoError(t, err)
	require.NoError(t, wd1.Close())

	// 2. Overwrite Data (UPSERT logic)
	wd2, err := store.Writer(ctx)
	require.NoError(t, err)
	_, err = wd2.Write(data2)
	require.NoError(t, err)
	require.NoError(t, wd2.Close())

	// 3. Verify Last Write Wins
	rdr, err := store.Reader(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rdr.Close()) })

	readData, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Equal(t, data2, readData)
}

func TestChunkedWrites(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, db := setupStore(ctx, t)

	part1 := []byte("Hello, ")
	part2 := []byte("World!")

	wd, err := store.Writer(ctx)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, wd.Close()) })

	// 1. Write Part 1
	_, err = wd.Write(part1)
	require.NoError(t, err)

	// 2. Verify nothing is in DB yet (Writer buffers until Close)
	// We query directly to verify the buffering behavior
	var count int

	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM discovery_service_state").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Database should be empty before Writer.Close() is called")

	// 3. Write Part 2
	_, err = wd.Write(part2)
	require.NoError(t, err)

	// 4. Close/Flush
	require.NoError(t, wd.Close())

	// 5. Verify Full Data
	rdr, err := store.Reader(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rdr.Close()) })

	fullData, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", string(fullData))
}

func TestEmptyReader(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, _ := setupStore(ctx, t)

	// 1. Create Reader without writing anything
	rdr, err := store.Reader(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rdr.Close()) })

	// 2. Should return EOF immediately (simulating empty file)
	// The implementation handles sql.ErrNoRows by returning EOF
	data, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestReaderCaching(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	store, db := setupStore(ctx, t)
	payload := []byte("cached-data-payload")

	// 1. Write Data
	wd, err := store.Writer(ctx)
	require.NoError(t, err)
	_, err = wd.Write(payload)
	require.NoError(t, err)
	require.NoError(t, wd.Close())

	// 2. Open Reader
	rdr, err := store.Reader(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rdr.Close()) })

	// 3. Partial Read (triggers DB query and loads full blob into memory)
	// We read just the first 6 bytes ("cached")
	buf := make([]byte, 6)
	n, err := rdr.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, "cached", string(buf))

	// 4. Manually corrupt DB
	_, err = db.ExecContext(ctx, "DELETE FROM discovery_service_state")
	require.NoError(t, err)

	// 5. Finish Reading
	remaining, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Equal(t, "-data-payload", string(remaining))
}

func TestWriteAfterClose(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store, _ := setupStore(ctx, t)

	wd, err := store.Writer(ctx)
	require.NoError(t, err)

	require.NoError(t, wd.Close())

	_, err = wd.Write([]byte("oops"))
	assert.Error(t, err, "writing to a closed writer should error")
}

func setupStore(ctx context.Context, t *testing.T) (*discovery.SQLiteStore, *sql.DB) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	store, err := discovery.NewSQLiteStore(ctx, db)
	require.NoError(t, err)

	return store, db
}
