// Copyright (c) 2025 Sidero Labs, Inc.
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

	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogfile"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogsqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func initLogger(ctx context.Context, config config.LogsAudit, db *sql.DB, logger *zap.Logger) (Logger, error) {
	var (
		fileAuditLogger *auditlogfile.LogFile
		dbAuditLogger   *auditlogsqlite.Store
	)

	if config.Path != "" {
		if err := os.MkdirAll(config.Path, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create audit logger: %w", err)
		}

		fileAuditLogger = auditlogfile.New(config.Path)
	}

	if config.SQLite.Enabled {
		if db == nil {
			return nil, fmt.Errorf("database is required for sqlite audit logger")
		}

		if config.SQLite.Timeout <= 0 {
			return nil, fmt.Errorf("sqlite audit logger timeout must be a positive number")
		}

		var err error

		if dbAuditLogger, err = auditlogsqlite.NewStore(ctx, db, config.SQLite.Timeout); err != nil {
			return nil, err
		}
	}

	switch {
	case config.Path != "" && config.SQLite.Enabled:
		if err := migrateFromFileToSQLite(ctx, fileAuditLogger, dbAuditLogger, logger); err != nil {
			return nil, fmt.Errorf("failed to migrate audit logs from file to sqlite: %w", err)
		}

		logger.Info("audit logs migrated from file to sqlite, using sqlite logger going forward")

		return dbAuditLogger, nil
	case config.SQLite.Enabled:
		logger.Info("using sqlite audit logger")

		return dbAuditLogger, nil
	default:
		logger.Info("using file audit logger")

		return fileAuditLogger, nil
	}
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
		migrated, readFailed, writeFailed int
		lastTs                            int64 // track the last valid timestamp to keep ordering for corrupt events
	)

	for {
		var eventData []byte

		if eventData, err = reader.Read(); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			logger.Warn("failed to read audit log while doing migration", zap.Error(err))

			readFailed++

			continue
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

	if migrated > 0 || readFailed > 0 || writeFailed > 0 {
		logger.Info("audit log migration summary",
			zap.Int("migrated", migrated),
			zap.Int("read_failed", readFailed),
			zap.Int("write_failed", writeFailed),
		)
	}

	if readFailed > 0 || writeFailed > 0 {
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
