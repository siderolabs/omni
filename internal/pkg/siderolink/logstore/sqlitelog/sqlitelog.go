// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlitelog

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
)

const (
	schemaSQL = `
CREATE TABLE IF NOT EXISTS "%[1]s" (
  id         INTEGER NOT NULL PRIMARY KEY,
  created_at INTEGER NOT NULL,
  message    BLOB    NOT NULL
) STRICT;
`
)

func getTableName(id string) string {
	return "logs_" + id
}

// OpenDB opens a SQLite database at the given path with appropriate options for log storage.
func OpenDB(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_txlock=immediate&_pragma=busy_timeout(50000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"

	return sql.Open("sqlite", dsn)
}

func Exists(ctx context.Context, db *sql.DB, id string, timeout time.Duration) (bool, error) {
	tableName := getTableName(id)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var count int

	query := "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?"
	if err := db.QueryRowContext(ctx, query, tableName).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

func Remove(ctx context.Context, db *sql.DB, id string, timeout time.Duration) error {
	tableName := getTableName(id)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %q`, tableName))
	if err != nil {
		return fmt.Errorf("failed to drop sqlite log table %q: %w", tableName, err)
	}

	return nil
}

func NewStore(ctx context.Context, config config.LogsMachineSQLite, db *sql.DB, id string, logger *zap.Logger) (*Store, error) {
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	tableName := getTableName(id)

	schemaReplaced := fmt.Sprintf(schemaSQL, tableName)

	if _, err := db.ExecContext(ctx, schemaReplaced); err != nil {
		return nil, fmt.Errorf("applying schema migration: %w", err)
	}

	// Initialize the ID counter by finding the last ID in the database.
	// We need this because we are manually assigning IDs to support the Hybrid Reader.
	var maxID sql.NullInt64

	if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT MAX(id) FROM %q`, tableName)).Scan(&maxID); err != nil {
		return nil, fmt.Errorf("failed to get max id: %w", err)
	}

	s := &Store{
		config:              config,
		tableName:           tableName,
		db:                  db,
		logger:              logger,
		subscriptionManager: NewManager(),
		lastFlush:           time.Now(),
		buffer:              make([]bufferedLog, 0, 1024),
	}

	// Initialize nextID to (MaxID + 1). If MaxID is NULL (empty table), Int64 is 0, so nextID becomes 1.
	s.nextID.Store(maxID.Int64 + 1)

	return s, nil
}

type bufferedLog struct {
	message   []byte
	id        int64
	createdAt int64
}

type Store struct {
	lastFlush           time.Time
	db                  *sql.DB
	logger              *zap.Logger
	subscriptionManager *Manager
	tableName           string
	buffer              []bufferedLog
	config              config.LogsMachineSQLite
	bufferSizeBytes     int
	nextID              atomic.Int64
	mu                  sync.Mutex
	closed              atomic.Bool
}

func (c *Store) WriteLine(ctx context.Context, line []byte) error {
	if c.closed.Load() {
		return errors.New("store is closed")
	}

	// Generate the next ID.
	id := c.nextID.Add(1) - 1

	c.mu.Lock()
	defer c.mu.Unlock()

	c.buffer = append(c.buffer, bufferedLog{
		id:        id,
		createdAt: time.Now().Unix(),
		message:   line,
	})
	c.bufferSizeBytes += len(line)

	// Notify the subscribers that new data is available.
	c.subscriptionManager.Notify()

	// Flush to DB if needed.
	if c.bufferSizeBytes >= c.config.CacheSize || time.Since(c.lastFlush) > c.config.FlushInterval {
		return c.flush(ctx)
	}

	return nil
}

// flush writes all buffered lines to the database in a single transaction.
//
// It assumes c.mu is already held.
func (c *Store) flush(ctx context.Context) error {
	if len(c.buffer) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("error starting flush transaction: %w", err)
	}

	defer tx.Rollback() //nolint:errcheck

	query := fmt.Sprintf(`INSERT INTO "%s" (id, created_at, message) VALUES (?, ?, ?)`, c.tableName)

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error preparing flush statement: %w", err)
	}

	defer stmt.Close() //nolint:errcheck

	for _, entry := range c.buffer {
		if _, err = stmt.ExecContext(ctx, entry.id, entry.createdAt, entry.message); err != nil {
			return fmt.Errorf("failed to insert buffered log: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit flush transaction: %w", err)
	}

	c.buffer = c.buffer[:0]
	c.bufferSizeBytes = 0
	c.lastFlush = time.Now()

	// ensure that readers are notified about the flush
	c.subscriptionManager.Notify()

	return nil
}

func (c *Store) Close() error {
	if c.closed.Swap(true) {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Do the final flush on close.
	flushCtx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	if err := c.flush(flushCtx); err != nil {
		c.logger.Error("failed to flush buffer on close", zap.Error(err))
	}

	c.subscriptionManager.Notify()

	return nil
}

func (c *Store) Reader(nLines int, follow bool) (logstore.LineReader, error) {
	var lastID int64

	if nLines > 0 {
		currentMax := c.nextID.Load() - 1

		lastID = max(currentMax-int64(nLines), 0)
	}

	var sub Subscription

	if follow {
		sub = c.subscriptionManager.Subscribe()
	}

	return &sqliteLineReader{
		store:   c,
		lastID:  lastID,
		follow:  follow,
		sub:     sub,
		closeCh: make(chan struct{}),
		buffer:  make([][]byte, 0, c.config.ReadBatchSize),
	}, nil
}

type sqliteLineReader struct {
	sub     Subscription
	store   *Store
	closeCh chan struct{}
	buffer  [][]byte
	lastID  int64
	once    sync.Once
	follow  bool
}

func (r *sqliteLineReader) ReadLine(ctx context.Context) ([]byte, error) {
	for {
		// If there is a line in the buffer, read it and set buffer to the remaining lines.
		if len(r.buffer) > 0 {
			line := r.buffer[0]
			r.buffer = r.buffer[1:]

			return line, nil
		}

		// Buffer is empty, try to refill it.
		if err := r.refillBuffer(ctx); err != nil {
			r.Close() //nolint:errcheck

			if errors.Is(err, context.Canceled) {
				return nil, io.EOF
			}

			return nil, err
		}

		// If the buffer is filled, continue the loop to read from it.
		if len(r.buffer) > 0 {
			continue
		}

		// No lines available (neither in DB nor in Memory), we are at the end of the log.
		//
		// If the follow was not requested, or if the store is closed, return EOF to signal end of log.
		//
		// Otherwise, wait for a notification of new data.
		if !r.follow || r.store.closed.Load() {
			return nil, io.EOF
		}

		// Wait for notification or closure.
		select {
		case <-r.sub.NotifyCh():
			continue
		case <-r.closeCh:
			return nil, io.EOF
		}
	}
}

func (r *sqliteLineReader) refillBuffer(ctx context.Context) error {
	r.buffer = r.buffer[:0]

	ctx, cancel := context.WithTimeout(ctx, r.store.config.Timeout)
	defer cancel()

	query := fmt.Sprintf(`SELECT id, message FROM "%s" WHERE id > ? ORDER BY id LIMIT %d`, r.store.tableName, r.store.config.ReadBatchSize)

	rows, err := r.store.db.QueryContext(ctx, query, r.lastID)
	if err != nil {
		return err
	}

	defer rows.Close() //nolint:errcheck

	count := 0

	for rows.Next() {
		var (
			id  int64
			msg []byte
		)

		if err = rows.Scan(&id, &msg); err != nil {
			return err
		}

		r.lastID = id
		r.buffer = append(r.buffer, msg)
		count++
	}

	if err = rows.Err(); err != nil {
		return err
	}

	// If we haven't filled the batch, check the in-memory Buffer (uncommitted data)
	if count < r.store.config.ReadBatchSize {
		r.store.mu.Lock()
		defer r.store.mu.Unlock()

		for _, entry := range r.store.buffer {
			if entry.id <= r.lastID {
				continue // Already consumed this ID
			}

			// If the id is not the next expected one, we stop reading from memory,
			// we were too slow and missed some entries that are not yet flushed to the DB.
			if entry.id != r.lastID+1 {
				break
			}

			r.buffer = append(r.buffer, entry.message)
			r.lastID = entry.id

			count++

			if count >= r.store.config.ReadBatchSize {
				break
			}
		}
	}

	return nil
}

func (r *sqliteLineReader) Close() error {
	r.once.Do(func() {
		close(r.closeCh)

		if r.sub != nil {
			r.sub.Unsubscribe()
		}
	})

	return nil
}
