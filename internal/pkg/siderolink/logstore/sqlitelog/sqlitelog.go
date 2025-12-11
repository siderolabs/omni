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

	"github.com/siderolabs/omni/client/pkg/panichandler"
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
func NewStore(timeout time.Duration, db *sql.DB, id string, logger *zap.Logger) (*Store, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	s := &Store{
		id:      truncateMachineID(id),
		timeout: timeout,
		db:      db,
		logger:  logger,
	}

	return s, nil
}

// Store implements the logstore.LogStore interface using SQLite as the backend.
type Store struct {
	db          *sql.DB
	logger      *zap.Logger
	id          string
	subscribers []chan struct{}
	mu          sync.Mutex
	closed      bool

	timeout time.Duration
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

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if _, err := s.db.ExecContext(ctx, query, s.id, message, time.Now().Unix()); err != nil {
		return fmt.Errorf("failed to write log message: %w", err)
	}

	if len(s.subscribers) == 0 {
		return nil
	}

	for _, ch := range s.subscribers {
		select {
		case ch <- struct{}{}:
		default:
			// channel is full, reader is already notified
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

func (s *Store) subscribe() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan struct{}, 1)

	s.subscribers = append(s.subscribers, ch)

	return ch
}

func (s *Store) unsubscribe(ch chan struct{}) {
	if ch == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribers = slices.DeleteFunc(s.subscribers, func(c chan struct{}) bool {
		return c == ch
	})
}

// Reader implements the logstore.LogStore interface.
func (s *Store) Reader(ctx context.Context, nLines int, follow bool) (logstore.LineReader, error) {
	var (
		closeCh  = make(chan struct{})
		followCh chan struct{}
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

	// If nLines is 0, we are not reading any history.
	// However, we need to know the current max ID to start following new logs correctly.
	// Otherwise, lastLogID defaults to 0, and the first "follow" signal will fetch the entire history.
	var lastLogID int64

	if nLines == 0 {
		var err error

		if lastLogID, err = s.currentMaxID(ctx); err != nil {
			s.unsubscribe(followCh)
			close(closeCh)

			return nil, fmt.Errorf("failed to get current max id: %w", err)
		}
	}

	rows, err := s.readerRows(ctx, nLines) //nolint:rowserrcheck // false positive, we do not iterate the logs here
	if err != nil {
		s.unsubscribe(followCh)
		close(closeCh)

		return nil, fmt.Errorf("failed to build reader rows: %w", err)
	}

	return &lineReader{
		store:     s,
		rows:      rows,
		followCh:  followCh,
		closeCh:   closeCh,
		lastLogID: lastLogID,
	}, nil
}

type lineReader struct {
	store     *Store
	rows      *sql.Rows
	followCh  chan struct{}
	closeCh   chan struct{}
	lastLogID int64
	closeOnce sync.Once
}

// ReadLine implements the logstore.LineReader interface.
func (r *lineReader) ReadLine(ctx context.Context) ([]byte, error) {
	for {
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

		// Current batch is exhausted; wait for the next batch of logs.
		if err := r.fetchNextBatch(ctx); err != nil {
			return nil, err
		}
	}
}

// fetchNextBatch handles the transition between the current exhausted result set
// and the next batch of logs. It closes the current rows, waits for a notification,
// and queries the database for new logs.
func (r *lineReader) fetchNextBatch(ctx context.Context) error {
	// Check for errors in the exhausted result set
	if err := r.rows.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return io.EOF
		}

		return fmt.Errorf("failed to read log message: %w", err)
	}

	if err := r.rows.Close(); err != nil {
		if errors.Is(err, context.Canceled) {
			return io.EOF
		}

		return fmt.Errorf("failed to close rows: %w", err)
	}

	if r.followCh == nil {
		return io.EOF
	}

	// Wait for notification
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return io.EOF
		}

		return ctx.Err()
	case _, ok := <-r.followCh:
		if !ok {
			return io.EOF
		}
	}

	newRows, err := r.store.readerRowsAfter(ctx, r.lastLogID) //nolint:rowserrcheck // false positive, we do not iterate the logs here
	if err != nil {
		// If the context was canceled during the query, return EOF
		// to allow graceful shutdown of the reader loop.
		if errors.Is(err, context.Canceled) {
			return io.EOF
		}

		return fmt.Errorf("failed to query new logs: %w", err)
	}

	r.rows = newRows

	return nil
}

// Close implements the logstore.LineReader interface.
func (r *lineReader) Close() error {
	var closeErr, rowsErr error

	if r.rows != nil {
		if err := r.rows.Close(); err != nil {
			closeErr = fmt.Errorf("failed to close rows: %w", err)
		}

		if err := r.rows.Err(); err != nil {
			rowsErr = fmt.Errorf("rows error: %w", err)
		}
	}

	r.closeOnce.Do(func() {
		r.store.unsubscribe(r.followCh)
		close(r.closeCh)
	})

	return errors.Join(closeErr, rowsErr)
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

func (s *Store) readerRowsAfter(ctx context.Context, lastLogID int64) (*sql.Rows, error) {
	query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s = ? AND %s > ? ORDER BY %s ASC",
		idColumn, messageColumn, tableName, machineIDColumn, idColumn, idColumn)

	rows, err := s.db.QueryContext(ctx, query, s.id, lastLogID)
	if err != nil {
		return nil, err
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

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var startID int64

	err := s.db.QueryRowContext(ctx, query, s.id, nLines).Scan(&startID)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to query start log id: %w", err)
	}

	return startID, nil
}

func (s *Store) currentMaxID(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COALESCE(MAX(%s), 0) FROM %s WHERE %s = ?", idColumn, tableName, machineIDColumn)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var id int64
	if err := s.db.QueryRowContext(ctx, query, s.id).Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
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
