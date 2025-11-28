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
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
)

const (
	// machineIDMaxLength is the maximum length allowed for the machine ID.
	//
	// Normally, a machine ID is a UUID, so 36 characters should be enough. However, to be safe, we allow up to 128 characters.
	//
	// If the machine ID exceeds this length, it will be truncated.
	machineIDMaxLength = 128

	// messageMaxLength is the maximum length allowed for a log message.
	//
	// If a log message exceeds this length, it will be truncated.
	messageMaxLength = 16 * 1024 // 16 KB
)

// NewStore creates a new Store.
func NewStore(config config.LogsMachineSQLite, db *sql.DB, id string, logger *zap.Logger) (*Store, error) {
	s := &Store{
		id:     truncateMachineID(id),
		config: config,
		db:     db,
		logger: logger,
	}

	return s, nil
}

type logEntry struct {
	message []byte
	id      int64
}

// Store implements the logstore.LogStore interface using SQLite as the backend.
type Store struct {
	db          *sql.DB
	logger      *zap.Logger
	id          string
	subscribers []chan logEntry
	config      config.LogsMachineSQLite
	mu          sync.Mutex
	closed      bool
}

// WriteLine implements the logstore.LogStore interface.
func (s *Store) WriteLine(ctx context.Context, message []byte) error {
	message = truncateMessage(message)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	query := fmt.Sprintf(`INSERT INTO %s (%s, %s, %s) VALUES (?, ?, ?)`, tableName, machineIDColumn, messageColumn, createdAtColumn)

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, s.id, message, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to write log message: %w", err)
	}

	if len(s.subscribers) == 0 {
		return nil
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	for _, ch := range s.subscribers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- logEntry{message: message, id: lastID}:
		default:
			s.logger.Warn("reader buffer is full, skipped log message delivery", zap.String("id", s.id))
		}
	}

	return nil
}

// Close implements the logstore.LogStore interface.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	for _, ch := range s.subscribers {
		close(ch)
	}

	s.subscribers = nil

	return nil
}

func (s *Store) subscribe() chan logEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan logEntry, 128)

	s.subscribers = append(s.subscribers, ch)

	return ch
}

func (s *Store) unsubscribe(ch chan logEntry) {
	if ch == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribers = slices.DeleteFunc(s.subscribers, func(c chan logEntry) bool {
		return c == ch
	})
}

// Reader implements the logstore.LogStore interface.
func (s *Store) Reader(ctx context.Context, nLines int, follow bool) (logstore.LineReader, error) {
	var (
		closeCh  = make(chan struct{})
		followCh chan logEntry
	)

	if follow {
		followCh = s.subscribe()

		// Make sure that we unsubscribe when the context is done
		panichandler.Go(func() {
			select {
			case <-closeCh: // normal close, reader is already unsubscribed
				return
			case <-ctx.Done():
				s.unsubscribe(followCh)
			}
		}, s.logger)
	}

	rows, err := s.readerRows(ctx, nLines) //nolint:rowserrcheck // false positive, we do not iterate the logs here
	if err != nil {
		s.unsubscribe(followCh)
		close(closeCh)

		return nil, fmt.Errorf("failed to build reader rows: %w", err)
	}

	return &lineReader{
		store:    s,
		rows:     rows,
		followCh: followCh,
		closeCh:  closeCh,
	}, nil
}

type lineReader struct {
	store     *Store
	rows      *sql.Rows
	followCh  chan logEntry
	closeCh   chan struct{}
	lastLogID int64
	closeOnce sync.Once
}

// ReadLine implements the logstore.LineReader interface.
func (r *lineReader) ReadLine(ctx context.Context) ([]byte, error) {
	// If there is a row available in the result set, return that
	if r.rows.Next() {
		var message []byte

		if err := r.rows.Scan(&r.lastLogID, &message); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, io.EOF
			}

			return nil, fmt.Errorf("failed to scan log message: %w", err)
		}

		return message, nil
	}

	// rows are exhausted, check for errors
	if err := r.rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log message: %w", err)
	}

	if r.followCh == nil {
		return nil, io.EOF
	}

	return r.waitForNextLine(ctx)
}

// waitForNextLine waits for the next log line from the follow channel or context cancellation.
func (r *lineReader) waitForNextLine(ctx context.Context) ([]byte, error) {
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, io.EOF
			}

			return nil, ctx.Err()
		case entry, ok := <-r.followCh:
			if !ok {
				return nil, io.EOF
			}

			// Skip entries that we've already seen
			if entry.id <= r.lastLogID {
				continue
			}

			// Update lastLogID to the current entry's ID
			r.lastLogID = entry.id

			return entry.message, nil
		}
	}
}

// Close implements the logstore.LineReader interface.
func (r *lineReader) Close() error {
	var closeErr error

	if r.rows != nil {
		if err := r.rows.Close(); err != nil {
			closeErr = fmt.Errorf("failed to close rows: %w", err)
		}
	}

	r.closeOnce.Do(func() {
		r.store.unsubscribe(r.followCh)
		close(r.closeCh)
	})

	return closeErr
}

func (s *Store) readerRows(ctx context.Context, nLines int) (*sql.Rows, error) {
	if nLines == 0 { // No lines are requested, return an empty result set
		query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE 1=0", idColumn, messageColumn, tableName)

		return s.db.QueryContext(ctx, query)
	}

	startID, err := s.readerStartID(ctx, nLines)
	if err != nil {
		return nil, fmt.Errorf("failed to determine start ID: %w", err)
	}

	query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s = ? AND %s >= ? ORDER BY %s ASC", idColumn, messageColumn, tableName, machineIDColumn, idColumn, idColumn)

	rows, err := s.db.QueryContext(ctx, query, s.id, startID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize rows: %w", err)
	}

	return rows, nil
}

func (s *Store) readerStartID(ctx context.Context, nLines int) (int64, error) {
	if nLines < 0 {
		return 0, nil
	}

	// Do COALESCE to return 0 if there are no logs for the machine
	query := fmt.Sprintf("SELECT COALESCE(MIN(id), 0) FROM (SELECT %s AS id FROM %s WHERE %s = ? ORDER BY %s DESC LIMIT ?)",
		idColumn, tableName, machineIDColumn, idColumn)

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var startID int64

	err := s.db.QueryRowContext(ctx, query, s.id, nLines).Scan(&startID)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to query start log id: %w", err)
	}

	return startID, nil
}

func truncateMessage(message []byte) []byte {
	if len(message) <= messageMaxLength {
		return message
	}

	return message[:messageMaxLength]
}

func truncateMachineID(id string) string {
	if len(id) <= machineIDMaxLength {
		return id
	}

	return id[:machineIDMaxLength]
}
