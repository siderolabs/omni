// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogsqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func initLogger(ctx context.Context, config config.LogsAudit, db *sqlitexx.Pool, logger *zap.Logger, onCleanup func(int)) (Logger, error) {
	if !config.GetEnabled() {
		logger.Info("audit logging is disabled")

		return &nopLogger{}, nil
	}

	var storeOpts []auditlogsqlite.Option
	if onCleanup != nil {
		storeOpts = append(storeOpts, auditlogsqlite.WithCleanupCallback(onCleanup))
	}

	dbAuditLogger, err := auditlogsqlite.NewStore(ctx, db, config.GetSqliteTimeout(), config.GetMaxSize(), config.GetCleanupProbability(), logger, storeOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlite audit logger: %w", err)
	}

	return dbAuditLogger, nil
}

type nopLogger struct{}

func (n *nopLogger) Write(context.Context, auditlog.Event) error { return nil }

func (n *nopLogger) Remove(context.Context, time.Time, time.Time) error { return nil }

func (n *nopLogger) Reader(context.Context, auditlog.ReadFilters) (auditlog.Reader, error) {
	return &nopReader{}, nil
}

func (n *nopLogger) FollowStart(context.Context, int64) (int64, error) {
	return 0, fmt.Errorf("audit logs are disabled")
}

func (n *nopLogger) FollowBatch(context.Context, int64, int64) ([]auditlog.Entry, error) {
	return nil, fmt.Errorf("audit logs are disabled")
}

// FollowSubscribe returns a nil channel: nothing is ever written, so there is nothing to
// wake up for, and following fails before it ever waits, at the start position resolution
// or at the first batch read of an id resume.
func (n *nopLogger) FollowSubscribe() (<-chan struct{}, func()) {
	return nil, func() {}
}

type nopReader struct{}

func (n *nopReader) Close() error { return nil }

func (n *nopReader) Read() ([]byte, error) {
	return nil, fmt.Errorf("audit logs are disabled")
}
