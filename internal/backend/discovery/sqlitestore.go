// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package discovery

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	_ "modernc.org/sqlite"
)

const (
	tableName  = "discovery_service_state"
	idColumn   = "id"
	dataColumn = "data"

	id = "state"

	queryTimeout = 30 * time.Second
)

type SQLiteStore struct {
	db *sql.DB
}

func (s *SQLiteStore) Reader(ctx context.Context) (io.ReadCloser, error) {
	return &reader{
		db:  s.db,
		ctx: ctx,
	}, nil
}

func (s *SQLiteStore) Writer(ctx context.Context) (io.WriteCloser, error) {
	return &writer{
		db:  s.db,
		ctx: ctx,
	}, nil
}

func NewSQLiteStore(ctx context.Context, db *sql.DB) (*SQLiteStore, error) {
	schema := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
      %s TEXT PRIMARY KEY,
      %s BLOB
    ) STRICT;`, tableName, idColumn, dataColumn)

	if _, err := db.ExecContext(ctx, schema); err != nil {
		return nil, fmt.Errorf("failed to create discovery service state table: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

type writer struct {
	db *sql.DB

	ctx context.Context //nolint:containedctx

	buf bytes.Buffer

	closed bool
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

	query := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES (?, ?) ON CONFLICT(%s) DO UPDATE SET %s=excluded.%s", tableName, idColumn, dataColumn, idColumn, dataColumn, dataColumn)

	ctx, cancel := context.WithTimeout(w.ctx, queryTimeout)
	defer cancel()

	if _, err := w.db.ExecContext(ctx, query, id, w.buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write snapshot to sqlite: %w", err)
	}

	w.closed = true

	return nil
}

type reader struct {
	db  *sql.DB
	ctx context.Context //nolint:containedctx

	reader *bytes.Reader

	closed bool
}

func (r *reader) Read(p []byte) (n int, err error) {
	if r.closed {
		return 0, errors.New("read from closed reader")
	}

	if r.reader == nil {
		query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", dataColumn, tableName, idColumn)

		ctx, cancel := context.WithTimeout(r.ctx, queryTimeout)
		defer cancel()

		row := r.db.QueryRowContext(ctx, query, id)

		var data []byte

		if err = row.Scan(&data); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				r.reader = bytes.NewReader(nil)

				return 0, io.EOF
			}

			return 0, fmt.Errorf("failed to read snapshot from sqlite: %w", err)
		}

		r.reader = bytes.NewReader(data)
	}

	return r.reader.Read(p)
}

func (r *reader) Close() error {
	r.closed = true

	return nil
}
