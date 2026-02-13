// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/state-sqlite/pkg/state/impl/sqlite"
	"go.uber.org/zap"
	"zombiezen.com/go/sqlite/sqlitex"
)

func newSQLitePersistentState(ctx context.Context, db *sqlitex.Pool, logger *zap.Logger) (*PersistentState, error) {
	st, err := sqlite.NewState(ctx, db, store.ProtobufMarshaler{},
		sqlite.WithLogger(logger),
		sqlite.WithTablePrefix("metrics_"),
		// run aggressive compaction, as we store frequently updated link counters here
		sqlite.WithCompactionInterval(5*time.Minute),
		sqlite.WithCompactMinAge(10*time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlite state: %w", err)
	}

	return &PersistentState{
		State: st,
		Close: func() error {
			st.Close()

			return nil
		},
	}, nil
}
