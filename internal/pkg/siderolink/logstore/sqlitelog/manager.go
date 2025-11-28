// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlitelog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
)

const (
	tableName       = "machine_logs"
	idColumn        = "id"
	machineIDColumn = "machine_id"
	createdAtColumn = "created_at"
	messageColumn   = "message"
)

// StoreManager manages log stores for machines.
type StoreManager struct {
	db     *sql.DB
	logger *zap.Logger
	config config.LogsMachineSQLite
}

// Run implements the LogStoreManager interface.
func (m *StoreManager) Run(ctx context.Context) error {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	// Do the initial cleanup immediately
	if err := m.doCleanup(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			return ctx.Err()
		case <-ticker.C:
		}

		if err := m.doCleanup(ctx); err != nil {
			m.logger.Error("failed to cleanup old logs", zap.Error(err))
		}
	}
}

func (m *StoreManager) doCleanup(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	query := fmt.Sprintf("DELETE FROM %s WHERE %s < ?", tableName, createdAtColumn)
	cutoff := time.Now().Add(-m.config.CleanupOlderThan).Unix()

	result, err := m.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	numRowsDeleted, err := result.RowsAffected()
	if err != nil {
		m.logger.Debug("failed to get number of rows affected during logs cleanup", zap.Error(err))
	}

	logLevel := zap.DebugLevel
	if numRowsDeleted > 0 {
		logLevel = zap.InfoLevel
	}

	m.logger.Log(logLevel, "completed logs cleanup", zap.Int64("rows_affected", numRowsDeleted))

	return nil
}

// Exists implements the LogStoreManager interface.
func (m *StoreManager) Exists(ctx context.Context, id string) (bool, error) {
	id = truncateMachineID(id)

	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	var dummy int

	query := fmt.Sprintf("SELECT 1 FROM %s WHERE %s=? LIMIT 1", tableName, machineIDColumn)
	if err := m.db.QueryRowContext(ctx, query, id).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check existence of logs for machine %q: %w", id, err)
	}

	return true, nil
}

// Remove implements the LogStoreManager interface.
func (m *StoreManager) Remove(ctx context.Context, id string) error {
	id = truncateMachineID(id)

	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	query := fmt.Sprintf("DELETE FROM %s WHERE %s=?", tableName, machineIDColumn)

	result, err := m.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete logs for machine %q: %w", id, err)
	}

	numRowsDeleted, err := result.RowsAffected()
	if err != nil {
		m.logger.Debug("failed to get number of rows affected when removing logs for machine", zap.String("machine_id", id), zap.Error(err))
	} else {
		m.logger.Info("removed logs for machine", zap.String("machine_id", id), zap.Int64("rows_affected", numRowsDeleted))
	}

	return nil
}

const schemaTmpl = `
    CREATE TABLE IF NOT EXISTS {{.TableName}} (
      {{.IDColumn}}        INTEGER PRIMARY KEY,
      {{.MachineIDColumn}} TEXT    NOT NULL,
      {{.MessageColumn}}   BLOB    NOT NULL,
      {{.CreatedAtColumn}} INTEGER NOT NULL
    ) STRICT;

    CREATE INDEX IF NOT EXISTS idx_{{.TableName}}_{{.MachineIDColumn}} 
    ON {{.TableName}}({{.MachineIDColumn}}, {{.IDColumn}});
`

type schemaParams struct {
	TableName       string
	IDColumn        string
	MachineIDColumn string
	MessageColumn   string
	CreatedAtColumn string
}

// NewStoreManager creates a new StoreManager.
func NewStoreManager(ctx context.Context, db *sql.DB, config config.LogsMachineSQLite, logger *zap.Logger) (*StoreManager, error) {
	templateParams := schemaParams{
		TableName:       tableName,
		IDColumn:        idColumn,
		MachineIDColumn: machineIDColumn,
		MessageColumn:   messageColumn,
		CreatedAtColumn: createdAtColumn,
	}

	tmpl, err := template.New("schema").Parse(schemaTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sqlite log table schema template: %w", err)
	}

	var sb strings.Builder

	if err = tmpl.Execute(&sb, templateParams); err != nil {
		return nil, fmt.Errorf("failed to execute sqlite log table schema template: %w", err)
	}

	schemaSQL := sb.String()

	if _, err = db.ExecContext(ctx, schemaSQL); err != nil {
		return nil, fmt.Errorf("failed to create sqlite log table schema: %w", err)
	}

	return &StoreManager{
		config: config,
		db:     db,
		logger: logger,
	}, nil
}

// Create implements the LogStoreManager interface.
func (m *StoreManager) Create(id string) (logstore.LogStore, error) {
	return NewStore(m.config, m.db, id, m.logger)
}
