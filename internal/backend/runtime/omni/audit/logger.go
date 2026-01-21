// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogfile"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogsqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func initLogger(ctx context.Context, config config.LogsAudit, db *sql.DB, logger *zap.Logger) (Logger, error) {
	if !pointer.SafeDeref(config.Enabled) {
		logger.Info("audit logging is disabled")

		return &nopLogger{}, nil
	}

	dbAuditLogger, err := auditlogsqlite.NewStore(ctx, db, config.SQLiteTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlite audit logger: %w", err)
	}

	path := config.Path //nolint:staticcheck

	if path == "" { // nothing to migrate, just use sqlite
		return dbAuditLogger, nil
	}

	if _, err = os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Error("failed to read audit log path, skip migration", zap.Error(err))
		}

		return dbAuditLogger, nil
	}

	// migrate from file to sqlite

	fileAuditLogger := auditlogfile.New(path)

	if err = migrateFromFileToSQLite(ctx, fileAuditLogger, dbAuditLogger, logger); err != nil {
		logger.Error("failed to migrate from audit log to sqlite", zap.Error(err))
	} else {
		logger.Info("audit logs migrated from file to sqlite, using sqlite logger going forward")
	}

	return dbAuditLogger, nil
}

func migrateFromFileToSQLite(ctx context.Context, fileAuditLogger Logger, dbAuditLogger *auditlogsqlite.Store, logger *zap.Logger) error {
	hasData, err := dbAuditLogger.HasData(ctx)
	if err != nil {
		return err
	}

	if hasData {
		logger.Info("sqlite audit log already contains data, skipping migration")

		return nil
	}

	reader, err := fileAuditLogger.Reader(ctx, time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	var (
		readFailed            bool
		migrated, writeFailed int
		lastTs                int64 // track the last valid timestamp to keep ordering for corrupt events
	)

	for {
		var eventData []byte

		if eventData, err = reader.Read(); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			logger.Warn("failed to read audit log while doing migration", zap.Error(err))

			readFailed = true

			break
		}

		var event auditlog.Event

		if err = json.Unmarshal(eventData, &event); err != nil {
			// unmarshal failed: The source file has a corrupt line - use a fallback to preserve the raw data
			event = auditlog.Event{
				Type: "migration_parse_error",
				// Use the last seen timestamp. This ensures the corrupt line appears
				// immediately after the previous valid line when sorted by (time, id).
				TimeMillis: lastTs,
				Data: &auditlog.Data{
					MigrationError: &auditlog.MigrationError{
						RawData: string(eventData), // save the raw bytes
						Error:   err.Error(),
					},
				},
			}
		} else {
			lastTs = event.TimeMillis
		}

		if err = dbAuditLogger.Write(ctx, event); err != nil {
			logger.Warn("failed to write audit log while doing migration", zap.Error(err))

			writeFailed++

			continue
		}

		migrated++
	}

	if migrated > 0 || readFailed || writeFailed > 0 {
		logger.Info("audit log migration summary",
			zap.Int("migrated", migrated),
			zap.Bool("read_failed", readFailed),
			zap.Int("write_failed", writeFailed),
		)
	}

	if readFailed || writeFailed > 0 {
		logger.Warn("skipping deletion of old audit log files due to migration errors - manual cleanup may be required")

		return nil
	}

	// remove the old logs
	if err = fileAuditLogger.Remove(ctx, time.Time{}, time.Now().Add(time.Hour)); err != nil {
		return err
	}

	logger.Info("old audit log files removed successfully")

	return nil
}

type nopLogger struct{}

func (n *nopLogger) Write(context.Context, auditlog.Event) error { return nil }

func (n *nopLogger) Remove(context.Context, time.Time, time.Time) error { return nil }

func (n *nopLogger) Reader(context.Context, time.Time, time.Time) (auditlog.Reader, error) {
	return &nopReader{}, nil
}

type nopReader struct{}

func (n *nopReader) Close() error { return nil }

func (n *nopReader) Read() ([]byte, error) { return nil, fmt.Errorf("audit logs are disabled") }
