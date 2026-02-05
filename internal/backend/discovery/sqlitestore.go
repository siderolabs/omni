// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package discovery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

const (
	tableName  = "discovery_service_state"
	idColumn   = "id"
	dataColumn = "data"

	id = "state"
)

type SQLiteStore struct {
	db      *sqlitex.Pool
	timeout time.Duration
}

func (s *SQLiteStore) Reader(ctx context.Context) (io.ReadCloser, error) {
	return &reader{
		db:      s.db,
		ctx:     ctx,
		timeout: s.timeout,
	}, nil
}

func (s *SQLiteStore) Writer(ctx context.Context) (io.WriteCloser, error) {
	return &writer{
		db:      s.db,
		ctx:     ctx,
		timeout: s.timeout,
	}, nil
}

func NewSQLiteStore(ctx context.Context, db *sqlitex.Pool, timeout time.Duration) (*SQLiteStore, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	schema := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
      %s TEXT PRIMARY KEY,
      %s BLOB
    ) STRICT;`, tableName, idColumn, dataColumn)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := db.Take(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer db.Put(conn)

	if err = sqlitex.ExecScript(conn, schema); err != nil {
		return nil, fmt.Errorf("failed to create discovery service state table: %w", err)
	}

	return &SQLiteStore{db: db, timeout: timeout}, nil
}

type writer struct {
	db *sqlitex.Pool

	ctx context.Context //nolint:containedctx

	buf bytes.Buffer

	closed  bool
	timeout time.Duration
}

func (w *writer) Write(p []byte) (n int, err error) {
	if w.closed {
		return 0, errors.New("write to closed writer")
	}

	return w.buf.Write(p)
}

func (w *writer) Close() error {
	if w.closed {
		return nil
	}

	query := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES ($id, $data) ON CONFLICT(%s) DO UPDATE SET %s=excluded.%s", tableName, idColumn, dataColumn, idColumn, dataColumn, dataColumn)

	ctx, cancel := context.WithTimeout(w.ctx, w.timeout)
	defer cancel()

	conn, err := w.db.Take(ctx)
	if err != nil {
		return fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer w.db.Put(conn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.
		BindString("$id", id).
		BindBytes("$data", w.buf.Bytes()).
		Exec()
	if err != nil {
		return fmt.Errorf("failed to write snapshot to sqlite: %w", err)
	}

	w.closed = true

	return nil
}

type reader struct {
	db  *sqlitex.Pool
	ctx context.Context //nolint:containedctx

	reader *bytes.Reader

	closed  bool
	timeout time.Duration
}

func (r *reader) Read(p []byte) (n int, err error) {
	if r.closed {
		return 0, errors.New("read from closed reader")
	}

	if r.reader == nil {
		query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = $id", dataColumn, tableName, idColumn)

		ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()

		conn, err := r.db.Take(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to take connection from pool: %w", err)
		}

		defer r.db.Put(conn)

		q, err := sqlitexx.NewQuery(conn, query)
		if err != nil {
			return 0, fmt.Errorf("failed to prepare sqlite statement: %w", err)
		}

		var data []byte

		err = q.
			BindString("$id", id).
			QueryRow(func(stmt *sqlite.Stmt) error {
				data = make([]byte, stmt.GetLen(dataColumn))
				stmt.GetBytes(dataColumn, data)

				return nil
			})
		if err != nil {
			if errors.Is(err, sqlitexx.ErrNoRows) {
				r.reader = bytes.NewReader(nil)

				return 0, io.EOF
			}

			return 0, fmt.Errorf("failed to execute query: %w", err)
		}

		r.reader = bytes.NewReader(data)
	}

	return r.reader.Read(p)
}

func (r *reader) Close() error {
	r.closed = true

	return nil
}
