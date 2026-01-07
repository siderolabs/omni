// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlitelog_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-circular/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/circularlog"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/sqlitelog"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestReadWrite(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, _ := setupDB(ctx, t, logger)

	sqliteStore1, err := storeManager.Create("test-1")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, sqliteStore1.Close())
	})

	sqliteStore2, err := storeManager.Create("test-2")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, sqliteStore2.Close())
	})

	t.Cleanup(func() {
		require.NoError(t, sqliteStore1.Close())
	})

	defaultConfig := config.Default()

	compressor, err := zstd.NewCompressor()
	require.NoError(t, err)

	circularStore, err := circularlog.NewStore(&defaultConfig.Logs.Machine, "test-1", compressor, zaptest.NewLogger(t))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, circularStore.Close())
	})

	numLines := 1000

	for i := range numLines {
		require.NoError(t, sqliteStore1.WriteLine(ctx, fmt.Appendf(nil, "Hello, World %d!", i)))
		require.NoError(t, sqliteStore2.WriteLine(ctx, fmt.Appendf(nil, "Hello, World %d!", i)))
		require.NoError(t, circularStore.WriteLine(ctx, fmt.Appendf(nil, "Hello, World %d!", i)))
	}

	t.Run("read all", func(t *testing.T) {
		t.Parallel()

		testRead(ctx, t, sqliteStore1, circularStore, numLines, -1)
	})

	t.Run("tail", func(t *testing.T) {
		t.Parallel()

		testRead(ctx, t, sqliteStore1, circularStore, 100, 100)
	})
}

func TestFollow(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, _ := setupDB(ctx, t, logger)

	store, err := storeManager.Create("test-1")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	require.NoError(t, store.WriteLine(ctx, []byte("Hello, World 1!")))
	require.NoError(t, store.WriteLine(ctx, []byte("Hello, World 2!")))

	rdr, err := store.Reader(ctx, -1, true)
	require.NoError(t, err)

	var wg sync.WaitGroup

	lineCh := make(chan string)

	wg.Go(func() {
		readLines(ctx, t, rdr, lineCh)
	})

	assertLine(ctx, t, lineCh, "Hello, World 1!")
	assertLine(ctx, t, lineCh, "Hello, World 2!")

	require.NoError(t, store.WriteLine(ctx, []byte("Hello, World 3!")))
	require.NoError(t, store.WriteLine(ctx, []byte("Hello, World 4!")))

	assertLine(ctx, t, lineCh, "Hello, World 3!")
	assertLine(ctx, t, lineCh, "Hello, World 4!")

	cancel()

	wg.Wait()
}

func TestTruncation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, _ := setupDB(ctx, t, logger)

	// 1. Setup Long ID and Huge Message
	longID := "machine-" + string(make([]byte, 200))

	hugeMessage := make([]byte, 20*1024)
	for i := range hugeMessage {
		hugeMessage[i] = 'a'
	}

	store, err := storeManager.Create(longID)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// 2. Write the huge message immediately.
	// This populates the DB so Exists() can return true,
	// and allows us to test message truncation simultaneously.
	err = store.WriteLine(ctx, hugeMessage)
	require.NoError(t, err)

	// 3. Test Machine ID Truncation (in Manager)
	// The manager should find the store even if we pass the original long ID.
	exists, err := storeManager.Exists(ctx, longID)
	require.NoError(t, err)
	assert.True(t, exists, "Manager should find store with truncated ID (requires at least one log line written)")

	// 4. Test Message Truncation (in Store)
	// Read it back.
	rdr, err := store.Reader(ctx, -1, false)
	require.NoError(t, err)

	lines := readAllLines(ctx, t, rdr)
	require.Len(t, lines, 1)

	// It should be exactly 16KB (16 * 1024).
	assert.Len(t, lines[0], 16*1024, "Message should be truncated to 16KB")
}

func TestManagerLifecycle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, _ := setupDB(ctx, t, logger)
	id := "lifecycle-test"

	// 1. Check Exists before creation.
	exists, err := storeManager.Exists(ctx, id)
	require.NoError(t, err)
	assert.False(t, exists)

	// 2. Create and write.
	store, err := storeManager.Create(id)
	require.NoError(t, err)
	require.NoError(t, store.WriteLine(ctx, []byte("log data")))

	require.NoError(t, store.Close())

	// 3. Check Exists after creation and close.
	exists, err = storeManager.Exists(ctx, id)
	require.NoError(t, err)
	assert.True(t, exists)

	// 4. Remove.
	err = storeManager.Remove(ctx, id)
	require.NoError(t, err)

	// 5. Check Exists after removal.
	exists, err = storeManager.Exists(ctx, id)
	require.NoError(t, err)
	assert.False(t, exists)

	// 6. Verify data is actually gone.
	storeRecreated, err := storeManager.Create(id)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, storeRecreated.Close())
	})

	rdr, err := storeRecreated.Reader(ctx, -1, false)
	require.NoError(t, err)
	lines := readAllLines(ctx, t, rdr)
	assert.Empty(t, lines)
}

func TestReaderParameters(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, _ := setupDB(ctx, t, logger)
	store, err := storeManager.Create("reader-params")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// Write 10 lines.
	for i := range 10 {
		require.NoError(t, store.WriteLine(ctx, fmt.Appendf(nil, "line %d", i)))
	}

	tests := []struct {
		name      string
		firstLine string
		nLines    int
		wantCount int
	}{
		{
			name:      "Read All (Negative)",
			nLines:    -1,
			wantCount: 10,
			firstLine: "line 0",
		},
		{
			name:      "Read None (Zero)",
			nLines:    0,
			wantCount: 0,
		},
		{
			name:      "Read Tail (Small)",
			nLines:    3,
			wantCount: 3,
			firstLine: "line 7",
		},
		{
			name:      "Read Tail (Large)",
			nLines:    100,
			wantCount: 10,
			firstLine: "line 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rdr, err := store.Reader(ctx, tt.nLines, false)
			require.NoError(t, err)

			lines := readAllLines(ctx, t, rdr)
			assert.Len(t, lines, tt.wantCount)

			if tt.wantCount > 0 && tt.firstLine != "" {
				assert.Equal(t, tt.firstLine, lines[0])
			}
		})
	}
}

func TestFollowNoHistory(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, _ := setupDB(ctx, t, logger)
	store, err := storeManager.Create("follow-no-history")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// 1. Write historical data.
	require.NoError(t, store.WriteLine(ctx, []byte("history 1")))
	require.NoError(t, store.WriteLine(ctx, []byte("history 2")))

	// 2. Request 0 lines, but follow=true.
	rdr, err := store.Reader(ctx, 0, true)
	require.NoError(t, err)

	// 3. Write new data.
	expected := "new 1"
	require.NoError(t, store.WriteLine(ctx, []byte(expected)))

	// 4. Read the first line.
	// Logic check: nLines=0 should skip "history 1/2" and stream "new 1" immediately.
	line, err := rdr.ReadLine(ctx)
	require.NoError(t, err)

	assert.Equal(t, expected, string(line), "Reader(0, true) should ignore history and only return new logs")
}

func TestFollowTail(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, _ := setupDB(ctx, t, logger)
	store, err := storeManager.Create("follow-tail")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// 1. Write 20 lines of history
	for i := range 20 {
		require.NoError(t, store.WriteLine(ctx, fmt.Appendf(nil, "history %d", i)))
	}

	// 2. Request last 5 lines and follow
	rdr, err := store.Reader(ctx, 5, true)
	require.NoError(t, err)

	lineCh := make(chan string)

	var wg sync.WaitGroup

	wg.Go(func() {
		readLines(ctx, t, rdr, lineCh)
	})

	// 3. Verify we get the last 5 lines of history (15-19)
	for i := 15; i < 20; i++ {
		assertLine(ctx, t, lineCh, fmt.Sprintf("history %d", i))
	}

	// 4. Write new data
	require.NoError(t, store.WriteLine(ctx, []byte("new 1")))

	// 5. Verify we get the new line
	assertLine(ctx, t, lineCh, "new 1")

	cancel()
	wg.Wait()
}

// TestFollowRapidWrites tests that a reader following rapid writes
// receives all lines without gaps or duplication.
func TestFollowRapidWrites(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, _ := setupDB(ctx, t, logger)
	store, err := storeManager.Create("rapid-writes")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// 1. Start reader (following)
	rdr, err := store.Reader(ctx, 0, true)
	require.NoError(t, err)

	count := 5000

	var eg errgroup.Group

	eg.Go(func() error {
		for i := range count {
			if writeErr := store.WriteLine(ctx, fmt.Appendf(nil, "msg %d", i)); writeErr != nil {
				return writeErr
			}
		}

		return nil
	})

	// 3. Read and verify no gaps
	for i := range count {
		line, readErr := rdr.ReadLine(ctx)
		require.NoError(t, readErr)

		require.Equal(t, fmt.Sprintf("msg %d", i), string(line))
	}

	require.NoError(t, eg.Wait())
}

func TestCleanupProbabilities(t *testing.T) {
	t.Parallel()

	// Helper to create a store with specific config
	setupStore := func(t *testing.T, prob float64, limit int) (*sqlitelog.StoreManager, string) {
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), fmt.Sprintf("prob-test-%f.db", prob))

		dbConf := config.Default().Storage.SQLite
		dbConf.Path = path

		db, err := sqlite.OpenDB(dbConf)
		require.NoError(t, err)

		t.Cleanup(func() { require.NoError(t, db.Close()) })

		logConf := config.Default().Logs.Machine.Storage
		logConf.MaxLinesPerMachine = limit
		logConf.CleanupProbability = prob

		state := state.WrapCore(namespaced.NewState(inmem.Build))

		logger := zaptest.NewLogger(t)
		mgr, err := sqlitelog.NewStoreManager(ctx, db, logConf, state, logger)
		require.NoError(t, err)

		return mgr, fmt.Sprintf("machine-prob-%f", prob)
	}

	tests := []struct {
		name        string
		prob        float64
		limit       int
		writes      int
		expectExact int  // -1 if exact match not expected
		expectLess  bool // if true, expect actual < writes
	}{
		{
			name:        "Probability 0.0 (Never Clean)",
			prob:        0.0,
			limit:       10,
			writes:      50,
			expectExact: 50,
		},
		{
			name:        "Probability 1.0 (Always Clean)",
			prob:        1.0,
			limit:       10,
			writes:      50,
			expectExact: 10,
		},
		{
			name:        "Probability 0.5 (Sometimes Clean)",
			prob:        0.5,
			limit:       10,
			writes:      100,
			expectExact: -1,
			expectLess:  true, // Expect cleanup to have triggered at least once
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr, id := setupStore(t, tt.prob, tt.limit)
			store, err := mgr.Create(id)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })

			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			t.Cleanup(cancel)

			// Perform Writes
			for i := range tt.writes {
				require.NoError(t, store.WriteLine(ctx, fmt.Appendf(nil, "msg-%d", i)))
			}

			// Check Result
			rdr, err := store.Reader(ctx, -1, false)
			require.NoError(t, err)
			lines := readAllLines(ctx, t, rdr)
			count := len(lines)

			if tt.expectExact != -1 {
				assert.Equal(t, tt.expectExact, count, "Exact count mismatch for prob %f", tt.prob)
			}

			if tt.expectLess {
				assert.Less(t, count, tt.writes, "Cleanup should have triggered at least once")
				assert.GreaterOrEqual(t, count, tt.limit, "Count cannot be less than limit")
			}
		})
	}
}

func TestOrphanLogsCleanup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)
	storeManager, state := setupDB(ctx, t, logger)

	const (
		numLive        = 1000
		numNonExistent = 1500
	)

	seedLogs := func(prefix string, count int) {
		for i := range count {
			id := fmt.Sprintf("%s-%d", prefix, i)
			store, storeErr := storeManager.Create(id)
			require.NoError(t, storeErr)

			require.NoError(t, store.WriteLine(ctx, fmt.Appendf(nil, "log line A for %s", id)))
			require.NoError(t, store.WriteLine(ctx, fmt.Appendf(nil, "log line B for %s", id)))
			require.NoError(t, store.Close())
		}
	}

	logger.Info("Seed logs...", zap.Int("live", numLive), zap.Int("nonExistent", numNonExistent))

	seedLogs("live", numLive)
	seedLogs("gone", numNonExistent)

	logger.Info("Populate state with machines...")

	for i := range numLive {
		m := omni.NewMachine(resources.DefaultNamespace, fmt.Sprintf("live-%d", i))
		require.NoError(t, state.Create(ctx, m))

		// mark some of the machines as tearing down - their logs should still be preserved
		if i%10 == 0 {
			_, err := state.Teardown(ctx, m.Metadata())
			require.NoError(t, err)
		}
	}

	logger.Info("Trigger cleanup...")

	require.NoError(t, storeManager.DoCleanup(ctx))

	logger.Info("Verify results...")

	checkExistence := func(prefix string, count int, shouldExist bool) {
		for i := range count {
			id := fmt.Sprintf("%s-%d", prefix, i)
			exists, existsErr := storeManager.Exists(ctx, id)
			require.NoError(t, existsErr)

			if shouldExist {
				assert.Truef(t, exists, "Machine %q should have logs", id)
			} else {
				assert.Falsef(t, exists, "Machine %q logs should be deleted", id)
			}
		}
	}

	checkExistence("live", numLive, true)
	checkExistence("gone", numNonExistent, false)
}

func testRead(ctx context.Context, t *testing.T, sqliteStore, circularStore logstore.LogStore, expectedLines, tailLines int) {
	t.Helper()

	sqliteReader, err := sqliteStore.Reader(ctx, tailLines, false)
	require.NoError(t, err)

	sqliteLines := readAllLines(ctx, t, sqliteReader)

	circularReader, err := circularStore.Reader(ctx, tailLines, false)
	require.NoError(t, err)

	circularLines := readAllLines(ctx, t, circularReader)

	assert.Len(t, sqliteLines, expectedLines)
	assert.Equal(t, circularLines, sqliteLines)
}

func readAllLines(ctx context.Context, t *testing.T, rdr logstore.LineReader) []string {
	t.Helper()

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
	t.Helper()

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

func assertLine(ctx context.Context, t *testing.T, lineCh <-chan string, expected string) {
	t.Helper()

	select {
	case line := <-lineCh:
		assert.Equal(t, expected, line)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}
}

// setupDB helper handles the standard SQLite test setup.
func setupDB(ctx context.Context, t *testing.T, logger *zap.Logger) (*sqlitelog.StoreManager, state.State) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	conf := config.Default().Storage.SQLite
	conf.Path = path

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	state := state.WrapCore(namespaced.NewState(inmem.Build))

	storeManager, err := sqlitelog.NewStoreManager(ctx, db, config.Default().Logs.Machine.Storage, state, logger)
	require.NoError(t, err)

	return storeManager, state
}
