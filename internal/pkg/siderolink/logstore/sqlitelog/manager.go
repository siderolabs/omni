// Copyright (c) 2026 Sidero Labs, Inc.
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

	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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
	state  state.State
	db     *sql.DB
	logger *zap.Logger
	config config.LogsMachineStorage
}

// Run implements the LogStoreManager interface.
func (m *StoreManager) Run(ctx context.Context) error {
	tickerCh := make(<-chan time.Time)

	cleanupInterval := m.config.GetCleanupInterval()
	if cleanupInterval <= 0 {
		m.logger.Info("log cleanup is disabled")
	} else {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		tickerCh = ticker.C
		// Do the initial cleanup immediately
		if err := m.DoCleanup(ctx); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			return ctx.Err()
		case <-tickerCh:
		}

		if err := m.DoCleanup(ctx); err != nil {
			m.logger.Error("failed to cleanup old logs", zap.Error(err))
		}
	}
}

// DoCleanup performs the actual cleanup of old and orphaned logs.
func (m *StoreManager) DoCleanup(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, m.config.GetSqliteTimeout())
	defer cancel()

	// Fetch all live machines - we will keep logs for these machines only
	machineList, err := m.state.List(ctx, omni.NewMachine("").Metadata())
	if err != nil {
		return fmt.Errorf("failed to list machines from state: %w", err)
	}

	machineIDs := make([]string, 0, len(machineList.Items))
	for _, machine := range machineList.Items {
		machineIDs = append(machineIDs, truncateMachineID(machine.Metadata().ID()))
	}

	// Temporary table lives in a single transaction, so we need to do everything in one transaction
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback() //nolint:errcheck

	// Create a temporary table to store active machine IDs - it is used in the join query to delete orphaned logs
	if _, err = tx.ExecContext(ctx, `CREATE TEMPORARY TABLE machine_ids (machine_id TEXT PRIMARY KEY) STRICT`); err != nil {
		return fmt.Errorf("failed to create temporary table: %w", err)
	}

	// Populate the temporary table with active machine IDs
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO machine_ids (machine_id) VALUES (?)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}

	defer stmt.Close() //nolint:errcheck

	for _, id := range machineIDs {
		if _, err = stmt.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("failed to insert machine ID %q: %w", id, err)
		}
	}

	if err = stmt.Close(); err != nil {
		return fmt.Errorf("failed to close statement: %w", err)
	}

	// Delete if:
	//   (A) Log is older than cutoff (Time-based cleanup)
	//   OR
	//   (B) Machine ID is NOT in the active list (Orphan cleanup)
	deleteSQL := fmt.Sprintf(`DELETE FROM %s WHERE %s < ? OR %s NOT IN (SELECT machine_id FROM machine_ids)`, tableName, createdAtColumn, machineIDColumn)

	cutoff := time.Now().Add(-m.config.GetCleanupOlderThan())

	result, err := tx.ExecContext(ctx, deleteSQL, cutoff.Unix())
	if err != nil {
		return fmt.Errorf("failed to execute unified cleanup: %w", err)
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		m.logger.Debug("failed to get rows affected", zap.Error(err))
	}

	// Drop the temp table
	if _, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS machine_ids`); err != nil {
		return fmt.Errorf("failed to drop temporary table: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if rowsDeleted > 0 {
		m.logger.Info("completed logs cleanup",
			zap.Int64("rows_deleted", rowsDeleted),
			zap.Int("active_machines", len(machineIDs)),
			zap.Time("cutoff_time", cutoff),
		)
	} else {
		m.logger.Debug("completed logs cleanup", zap.Int64("rows_deleted", 0))
	}

	return nil
}

// Exists implements the LogStoreManager interface.
func (m *StoreManager) Exists(ctx context.Context, id string) (bool, error) {
	id = truncateMachineID(id)

	ctx, cancel := context.WithTimeout(ctx, m.config.GetSqliteTimeout())
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

	ctx, cancel := context.WithTimeout(ctx, m.config.GetSqliteTimeout())
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
func NewStoreManager(ctx context.Context, db *sql.DB, config config.LogsMachineStorage, omniState state.State, logger *zap.Logger) (*StoreManager, error) {
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
		state:  omniState,
		logger: logger,
	}, nil
}

// Create implements the LogStoreManager interface.
func (m *StoreManager) Create(id string) (logstore.LogStore, error) {
	return NewStore(m.config, m.db, id, m.logger)
}
