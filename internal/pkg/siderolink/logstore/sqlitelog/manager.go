// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlitelog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"go.uber.org/zap"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
)

const (
	// TableName is the SQLite table name used by the machine log store.
	TableName       = "machine_logs"
	idColumn        = "id"
	machineIDColumn = "machine_id"
	createdAtColumn = "created_at"
	messageColumn   = "message"

	// cleanupBatchSize is the maximum number of rows deleted per batch in cleanup operations.
	cleanupBatchSize = 1000
)

// StoreManagerOption configures optional StoreManager behavior.
type StoreManagerOption func(*StoreManager)

// WithCleanupCallback sets a callback that is called after cleanup with the number of deleted rows.
func WithCleanupCallback(cb func(int)) StoreManagerOption {
	return func(m *StoreManager) {
		m.onCleanup = cb
	}
}

// StoreManager manages log stores for machines.
type StoreManager struct {
	state     state.State
	db        *sqlitexx.Pool
	logger    *zap.Logger
	onCleanup func(int)
	config    config.LogsMachineStorage
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

		// Run the first cleanup immediately but non-fatally.
		// This avoids blocking startup if another writer holds the SQLite lock.
		if err := m.DoCleanup(ctx); err != nil {
			m.logger.Warn("initial log cleanup failed, will retry on next tick", zap.Error(err))
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
//
// Each batch deletes up to cleanupBatchSize rows in its own implicit autocommit statement,
// releasing the SQLite write lock between statements so other writers are not blocked for long.
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

	cutoff := time.Now().Add(-m.config.GetCleanupOlderThan())

	// Phase 1: Time-based cleanup in batches
	timeDeleted, err := m.cleanupByTime(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("time-based cleanup: %w", err)
	}

	// Phase 2: Orphan cleanup in batches
	orphanDeleted, err := m.cleanupOrphans(ctx, machineIDs)
	if err != nil {
		return fmt.Errorf("orphan cleanup: %w", err)
	}

	rowsDeleted := timeDeleted + orphanDeleted

	if m.onCleanup != nil && rowsDeleted > 0 {
		m.onCleanup(rowsDeleted)
	}

	if rowsDeleted > 0 {
		m.logger.Info("completed logs cleanup",
			zap.Int("rows_deleted", rowsDeleted),
			zap.Int("active_machines", len(machineIDs)),
			zap.Time("cutoff_time", cutoff),
		)
	} else {
		m.logger.Debug("completed logs cleanup", zap.Int64("rows_deleted", 0))
	}

	// Phase 3: Size-based cleanup (already batched)
	if m.config.GetMaxSize() > 0 {
		sizeRowsDeleted, sizeErr := m.removeBySize(ctx)
		if sizeErr != nil {
			m.logger.Warn("failed to cleanup machine logs by size",
				zap.Int("rows_deleted_before_error", sizeRowsDeleted), zap.Error(sizeErr))
		} else if sizeRowsDeleted > 0 {
			m.logger.Info("completed size-based logs cleanup", zap.Int("rows_deleted", sizeRowsDeleted))
		}
	}

	return nil
}

// cleanupByTime deletes log rows older than cutoff in batches of cleanupBatchSize.
func (m *StoreManager) cleanupByTime(ctx context.Context, cutoff time.Time) (int, error) {
	// DELETE ... LIMIT is not supported (requires SQLITE_ENABLE_UPDATE_DELETE_LIMIT), so we use a subquery to select the IDs to delete.
	deleteSQL := fmt.Sprintf(
		`DELETE FROM %s WHERE %s IN (SELECT %s FROM %s WHERE %s < $cutoff LIMIT $limit)`,
		TableName, idColumn, idColumn, TableName, createdAtColumn,
	)

	conn, err := m.db.Take(ctx)
	if err != nil {
		return 0, fmt.Errorf("error taking connection: %w", err)
	}

	defer m.db.Put(conn)

	var totalDeleted int

	for {
		q, qErr := sqlitexx.NewQuery(conn, deleteSQL)
		if qErr != nil {
			return totalDeleted, fmt.Errorf("failed to prepare statement: %w", qErr)
		}

		qErr = q.
			BindInt64("$cutoff", cutoff.Unix()).
			BindInt64("$limit", cleanupBatchSize).
			Exec()
		if qErr != nil {
			return totalDeleted, fmt.Errorf("failed to execute delete: %w", qErr)
		}

		deleted := conn.Changes()
		totalDeleted += deleted

		if deleted == 0 || ctx.Err() != nil {
			break
		}
	}

	return totalDeleted, nil
}

// cleanupOrphans deletes log rows for machines that no longer exist, in batches of cleanupBatchSize.
// It creates a temp table with active machine IDs for efficient indexed lookups.
func (m *StoreManager) cleanupOrphans(ctx context.Context, activeMachineIDs []string) (int, error) {
	conn, err := m.db.Take(ctx)
	if err != nil {
		return 0, fmt.Errorf("error taking connection: %w", err)
	}

	defer m.db.Put(conn)

	// Temp table lives on the connection (not tied to a transaction) and doesn't lock the main DB.
	if err = sqlitex.ExecScript(conn, `CREATE TEMPORARY TABLE IF NOT EXISTS cleanup_machine_ids (machine_id TEXT PRIMARY KEY) STRICT`); err != nil {
		return 0, fmt.Errorf("failed to create temporary table: %w", err)
	}

	// Clear any stale data from a previous run if the table already existed on this pooled connection.
	if err = sqlitex.ExecScript(conn, `DELETE FROM cleanup_machine_ids`); err != nil {
		return 0, fmt.Errorf("failed to clear temporary table: %w", err)
	}

	defer func() {
		if dropErr := sqlitex.ExecScript(conn, `DROP TABLE IF EXISTS cleanup_machine_ids`); dropErr != nil {
			m.logger.Warn("failed to drop temporary table", zap.Error(dropErr))
		}
	}()

	for _, id := range activeMachineIDs {
		q, qErr := sqlitexx.NewQuery(conn, "INSERT OR IGNORE INTO cleanup_machine_ids (machine_id) VALUES ($machine_id)")
		if qErr != nil {
			return 0, fmt.Errorf("failed to prepare insert statement: %w", qErr)
		}

		if qErr = q.BindString("$machine_id", id).Exec(); qErr != nil {
			return 0, fmt.Errorf("failed to insert machine ID %q: %w", id, qErr)
		}
	}

	// DELETE ... LIMIT is not supported (requires SQLITE_ENABLE_UPDATE_DELETE_LIMIT), so we use a subquery to select the IDs to delete.
	deleteSQL := fmt.Sprintf(
		`DELETE FROM %s WHERE %s IN (SELECT %s FROM %s WHERE %s NOT IN (SELECT machine_id FROM cleanup_machine_ids) LIMIT $limit)`,
		TableName, idColumn, idColumn, TableName, machineIDColumn,
	)

	var totalDeleted int

	for {
		q, qErr := sqlitexx.NewQuery(conn, deleteSQL)
		if qErr != nil {
			return totalDeleted, fmt.Errorf("failed to prepare statement: %w", qErr)
		}

		if qErr = q.BindInt64("$limit", cleanupBatchSize).Exec(); qErr != nil {
			return totalDeleted, fmt.Errorf("failed to execute delete: %w", qErr)
		}

		deleted := conn.Changes()
		totalDeleted += deleted

		if deleted == 0 || ctx.Err() != nil {
			break
		}
	}

	return totalDeleted, nil
}

// removeBySize deletes the oldest log rows globally across all machines to bring the table under maxSize bytes.
// It estimates the number of rows to delete based on average row size, then deletes in batches of
// cleanupBatchSize. The estimate may slightly overshoot; any remaining excess is handled on the next cycle.
func (m *StoreManager) removeBySize(ctx context.Context) (int, error) {
	conn, err := m.db.Take(ctx)
	if err != nil {
		return 0, fmt.Errorf("error taking connection for size cleanup: %w", err)
	}

	defer m.db.Put(conn)

	rowsToDelete, err := m.computeSizeExcess(conn)
	if err != nil {
		return 0, err
	}

	if rowsToDelete <= 0 {
		return 0, nil
	}

	var totalDeleted int

	for rowsToDelete > 0 {
		batchSize := min(rowsToDelete, cleanupBatchSize)

		deleted, batchErr := m.deleteOldestBatch(conn, batchSize)
		if batchErr != nil {
			return totalDeleted, batchErr
		}

		if deleted == 0 {
			break
		}

		totalDeleted += deleted
		rowsToDelete -= int64(deleted)

		if ctx.Err() != nil {
			break
		}
	}

	if m.onCleanup != nil && totalDeleted > 0 {
		m.onCleanup(totalDeleted)
	}

	return totalDeleted, nil
}

// computeSizeExcess estimates how many rows need to be deleted based on the table size and average row size.
// Returns 0 when the table is within limits or empty.
func (m *StoreManager) computeSizeExcess(conn *sqlite.Conn) (int64, error) {
	sizeQuery := fmt.Sprintf(`SELECT COALESCE(SUM(d.pgsize), 0) FROM dbstat d JOIN sqlite_master m ON d.name = m.name WHERE m.tbl_name = '%s'`, TableName)

	q, err := sqlitexx.NewQuery(conn, sizeQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare dbstat query: %w", err)
	}

	var tableSize int64

	if err = q.QueryRow(func(stmt *sqlite.Stmt) error {
		tableSize = stmt.ColumnInt64(0)

		return nil
	}); err != nil {
		return 0, fmt.Errorf("failed to query table size: %w", err)
	}

	if tableSize <= int64(m.config.GetMaxSize()) {
		return 0, nil
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, TableName)

	q, err = sqlitexx.NewQuery(conn, countQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare row count query: %w", err)
	}

	var rowCount int64

	if err = q.QueryRow(func(stmt *sqlite.Stmt) error {
		rowCount = stmt.ColumnInt64(0)

		return nil
	}); err != nil {
		return 0, fmt.Errorf("failed to query row count: %w", err)
	}

	if rowCount <= 0 {
		return 0, nil
	}

	avgRowSize := tableSize / rowCount
	if avgRowSize <= 0 {
		avgRowSize = 1
	}

	// Ceiling division ensures at least 1 row is deleted when slightly over maxSize.
	excess := tableSize - int64(m.config.GetMaxSize())
	rowsToDelete := (excess + avgRowSize - 1) / avgRowSize

	if rowsToDelete <= 0 {
		return 0, nil
	}

	return rowsToDelete, nil
}

// deleteOldestBatch deletes up to limit oldest rows by selecting them via a subquery.
// This correctly handles non-contiguous IDs (gaps from per-machine deletions).
func (m *StoreManager) deleteOldestBatch(conn *sqlite.Conn, limit int64) (int, error) {
	deleteQuery := fmt.Sprintf(
		`DELETE FROM %s WHERE %s IN (SELECT %s FROM %s ORDER BY %s ASC LIMIT $limit)`,
		TableName, idColumn, idColumn, TableName, idColumn,
	)

	q, err := sqlitexx.NewQuery(conn, deleteQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare size-based delete query: %w", err)
	}

	if err = q.BindInt64("$limit", limit).Exec(); err != nil {
		return 0, fmt.Errorf("failed to delete oldest machine log rows by size: %w", err)
	}

	return conn.Changes(), nil
}

// Exists implements the LogStoreManager interface.
func (m *StoreManager) Exists(ctx context.Context, id string) (bool, error) {
	id = truncateMachineID(id)

	ctx, cancel := context.WithTimeout(ctx, m.config.GetSqliteTimeout())
	defer cancel()

	conn, err := m.db.Take(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer m.db.Put(conn)

	query := fmt.Sprintf("SELECT 1 FROM %s WHERE %s=$machine_id LIMIT 1", TableName, machineIDColumn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return false, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.
		BindString("$machine_id", id).
		QueryRow(func(stmt *sqlite.Stmt) error {
			return nil
		})
	if err != nil {
		if errors.Is(err, sqlitexx.ErrNoRows) {
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

	query := fmt.Sprintf("DELETE FROM %s WHERE %s=$machine_id", TableName, machineIDColumn)

	conn, err := m.db.Take(ctx)
	if err != nil {
		return fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer m.db.Put(conn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.
		BindString("$machine_id", id).
		Exec()
	if err != nil {
		return fmt.Errorf("failed to delete logs for machine %q: %w", id, err)
	}

	numRowsDeleted := conn.Changes()

	if m.onCleanup != nil {
		m.onCleanup(numRowsDeleted)
	}

	m.logger.Info("removed logs for machine", zap.String("machine_id", id), zap.Int("rows_affected", numRowsDeleted))

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
func NewStoreManager(ctx context.Context, db *sqlitexx.Pool, config config.LogsMachineStorage, omniState state.State, logger *zap.Logger, opts ...StoreManagerOption) (*StoreManager, error) {
	templateParams := schemaParams{
		TableName:       TableName,
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

	conn, err := db.Take(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer db.Put(conn)

	if err = sqlitex.ExecScript(conn, schemaSQL); err != nil {
		return nil, fmt.Errorf("failed to create sqlite log table schema: %w", err)
	}

	mgr := &StoreManager{
		config: config,
		db:     db,
		state:  omniState,
		logger: logger,
	}

	for _, opt := range opts {
		opt(mgr)
	}

	return mgr, nil
}

// Create implements the LogStoreManager interface.
func (m *StoreManager) Create(id string) (logstore.LogStore, error) {
	var opts []StoreOption
	if m.onCleanup != nil {
		opts = append(opts, WithStoreCleanupCallback(m.onCleanup))
	}

	return NewStore(m.config, m.db, id, m.logger, opts...)
}
