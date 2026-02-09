// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlitelog

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"iter"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"go.uber.org/zap"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

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
func NewStore(config config.LogsMachineStorage, db *sqlitex.Pool, id string, logger *zap.Logger) (*Store, error) {
	sqliteTimeout := config.GetSqliteTimeout()
	if sqliteTimeout <= 0 {
		sqliteTimeout = 30 * time.Second
	}

	s := &Store{
		id:            truncateMachineID(id),
		config:        config,
		db:            db,
		logger:        logger,
		sqliteTimeout: sqliteTimeout,
	}

	return s, nil
}

// Store implements the logstore.LogStore interface using SQLite as the backend.
type Store struct {
	config        config.LogsMachineStorage
	db            *sqlitex.Pool
	logger        *zap.Logger
	id            string
	subscribers   []chan struct{}
	mu            sync.Mutex
	closed        bool
	sqliteTimeout time.Duration
}

// WriteLine implements the logstore.LogStore interface.
func (s *Store) WriteLine(ctx context.Context, message []byte) error {
	message = truncateMessage(message)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, s.sqliteTimeout)
	defer cancel()

	err := func() error {
		conn, err := s.db.Take(ctx)
		if err != nil {
			return fmt.Errorf("failed to take connection from pool: %w", err)
		}

		defer s.db.Put(conn)

		query := fmt.Sprintf(`INSERT INTO %s (%s, %s, %s) VALUES ($machine_id, $message, $created_at)`, tableName, machineIDColumn, messageColumn, createdAtColumn)

		q, err := sqlitexx.NewQuery(conn, query)
		if err != nil {
			return fmt.Errorf("failed to prepare sqlite statement: %w", err)
		}

		err = q.
			BindString("$machine_id", s.id).
			BindBytes("$message", message).
			BindInt64("$created_at", time.Now().Unix()).
			Exec()
		if err != nil {
			return fmt.Errorf("failed to write log message: %w", err)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	if rand.Float64() < s.config.GetCleanupProbability() {
		if err := s.doCleanup(ctx); err != nil { // log the error but do not return an error, as the log message was written successfully
			s.logger.Warn("failed to cleanup old logs after writing log message", zap.Error(err))
		}
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

func (s *Store) doCleanup(ctx context.Context) error {
	conn, err := s.db.Take(ctx)
	if err != nil {
		return fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer s.db.Put(conn)

	// find the cutoff ID
	query := fmt.Sprintf("SELECT COALESCE(MAX(%s), 0) FROM (SELECT %s FROM %s WHERE %s = $machine_id ORDER BY %s DESC LIMIT 1 OFFSET $max_lines)",
		idColumn, idColumn, tableName, machineIDColumn, idColumn)

	var cutoffID int64

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.
		BindString("$machine_id", s.id).
		BindInt("$max_lines", s.config.GetMaxLinesPerMachine()).
		QueryRow(func(stmt *zombiesqlite.Stmt) error {
			cutoffID = stmt.ColumnInt64(0)

			return nil
		})
	if err != nil {
		return fmt.Errorf("failed to determine cutoff ID for cleanup: %w", err)
	}

	// delete logs older than cutoff ID
	delQuery := fmt.Sprintf("DELETE FROM %s WHERE %s = $machine_id AND %s <= $cutoff_id", tableName, machineIDColumn, idColumn)

	q, err = sqlitexx.NewQuery(conn, delQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.
		BindString("$machine_id", s.id).
		BindInt64("$cutoff_id", cutoffID).
		Exec()
	if err != nil {
		return fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	numRowsDeleted := conn.Changes()
	s.logger.Debug("deleted old logs", zap.Int("num_rows_deleted", numRowsDeleted))

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

	conn, next, stop, err := s.readerRows(ctx, nLines) //nolint:rowserrcheck // false positive, we do not iterate the logs here
	if err != nil {
		s.unsubscribe(followCh)
		close(closeCh)

		return nil, fmt.Errorf("failed to build reader rows: %w", err)
	}

	return &lineReader{
		store:     s,
		conn:      conn,
		next:      next,
		stop:      stop,
		followCh:  followCh,
		closeCh:   closeCh,
		lastLogID: lastLogID,
	}, nil
}

type lineReader struct {
	store     *Store
	conn      *zombiesqlite.Conn
	next      func() (*zombiesqlite.Stmt, error, bool)
	stop      func()
	followCh  chan struct{}
	closeCh   chan struct{}
	lastLogID int64
	closeOnce sync.Once
}

// ReadLine implements the logstore.LineReader interface.
func (r *lineReader) ReadLine(ctx context.Context) ([]byte, error) {
	for {
		// If there is a row available in the result set, return that
		result, err, ok := r.next()
		if err != nil {
			// this error is returned when the context is canceled (via pool.Take(ctx))
			if zombiesqlite.ErrCode(err) == zombiesqlite.ResultInterrupt {
				return nil, io.EOF
			}

			return nil, fmt.Errorf("failed to read next log message: %w", err)
		}

		if ok {
			r.lastLogID = result.GetInt64(idColumn)

			message := make([]byte, result.GetLen(messageColumn))
			result.GetBytes(messageColumn, message)

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
	r.stop()
	r.stop = nil

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

	var err error

	r.next, r.stop, err = r.store.readerRowsAfter(r.conn, r.lastLogID) //nolint:rowserrcheck // false positive, we do not iterate the logs here
	if err != nil {
		// If the context was canceled during the query, return EOF
		// to allow graceful shutdown of the reader loop.
		//
		// this error is returned when the context is canceled (via pool.Take(ctx))
		if zombiesqlite.ErrCode(err) == zombiesqlite.ResultInterrupt {
			return io.EOF
		}

		return fmt.Errorf("failed to query new logs: %w", err)
	}

	return nil
}

// Close implements the logstore.LineReader interface.
func (r *lineReader) Close() error {
	if r.stop != nil {
		r.stop()
	}

	r.closeOnce.Do(func() {
		r.store.db.Put(r.conn)
		r.store.unsubscribe(r.followCh)
		close(r.closeCh)
	})

	return nil
}

func (s *Store) readerRows(ctx context.Context, nLines int) (*zombiesqlite.Conn, func() (*zombiesqlite.Stmt, error, bool), func(), error) {
	conn, err := s.db.Take(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	if nLines == 0 { // No lines are requested, return an empty result set
		return conn,
			func() (*zombiesqlite.Stmt, error, bool) {
				return nil, nil, false
			},
			func() {},
			nil
	}

	startID, err := s.readerStartID(conn, nLines)
	if err != nil {
		s.db.Put(conn)

		return nil, nil, nil, fmt.Errorf("failed to determine start ID: %w", err)
	}

	query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s = $machine_id AND %s >= $start_id ORDER BY %s ASC", idColumn, messageColumn, tableName, machineIDColumn, idColumn, idColumn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		s.db.Put(conn)

		return nil, nil, nil, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	it := q.
		BindString("$machine_id", s.id).
		BindInt64("$start_id", startID).
		QueryIter()

	next, stop := iter.Pull2(it)

	return conn, next, stop, err
}

func (s *Store) readerRowsAfter(conn *zombiesqlite.Conn, lastLogID int64) (func() (*zombiesqlite.Stmt, error, bool), func(), error) {
	query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s = $machine_id AND %s > $last_log_id ORDER BY %s ASC",
		idColumn, messageColumn, tableName, machineIDColumn, idColumn, idColumn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	it := q.
		BindString("$machine_id", s.id).
		BindInt64("$last_log_id", lastLogID).
		QueryIter()

	next, stop := iter.Pull2(it)

	return next, stop, nil
}

func (s *Store) readerStartID(conn *zombiesqlite.Conn, nLines int) (int64, error) {
	if nLines < 0 {
		return 0, nil
	}

	// Do COALESCE to return 0 if there are no logs for the machine
	query := fmt.Sprintf("SELECT COALESCE(MIN(id), 0) FROM (SELECT %s AS id FROM %s WHERE %s = $machine_id ORDER BY %s DESC LIMIT $limit)",
		idColumn, tableName, machineIDColumn, idColumn)

	var startID int64

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.
		BindString("$machine_id", s.id).
		BindInt("$limit", nLines).
		QueryRow(func(stmt *zombiesqlite.Stmt) error {
			startID = stmt.ColumnInt64(0)

			return nil
		})
	if err != nil {
		return 0, fmt.Errorf("failed to query start log id: %w", err)
	}

	return startID, nil
}

func (s *Store) currentMaxID(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COALESCE(MAX(%s), 0) FROM %s WHERE %s = $machine_id", idColumn, tableName, machineIDColumn)

	ctx, cancel := context.WithTimeout(ctx, s.sqliteTimeout)
	defer cancel()

	conn, err := s.db.Take(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer s.db.Put(conn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	var id int64

	err = q.
		BindString("$machine_id", s.id).
		QueryRow(func(stmt *zombiesqlite.Stmt) error {
			id = stmt.ColumnInt64(0)

			return nil
		})
	if err != nil {
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
