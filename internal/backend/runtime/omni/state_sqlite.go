// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/state-sqlite/pkg/state/impl/sqlite"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

func newSQLitePersistentState(ctx context.Context, path string, logger *zap.Logger) (*PersistentState, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_txlock=immediate&_pragma=busy_timeout(50000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	st, err := sqlite.NewState(ctx, db, store.ProtobufMarshaler{}, sqlite.WithLogger(logger))
	if err != nil {
		db.Close() //nolint:errcheck

		return nil, fmt.Errorf("failed to create sqlite state: %w", err)
	}

	return &PersistentState{
		State: st,
		Close: func() error {
			st.Close()

			return db.Close()
		},
	}, nil
}
