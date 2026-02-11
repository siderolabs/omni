// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auditlogsqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"math/rand/v2"
	"strings"
	"text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
)

const (
	// removeBySizeBatchCap is the maximum number of rows deleted per removeBySize invocation.
	// The probabilistic trigger ensures convergence over multiple writes.
	removeBySizeBatchCap = 1000

	// TableName is the SQLite table name used by the audit log store.
	TableName           = "audit_logs"
	idColumn            = "id"
	eventTypeColumn     = "event_type"
	resourceTypeColumn  = "resource_type"
	eventTSMillisColumn = "event_ts_ms"
	eventDataColumn     = "event_data"

	// Extracted columns for indexing/querying.
	actorEmailColumn = "actor_email"
	resourceIDColumn = "resource_id"
	clusterIDColumn  = "cluster_id"
)

// Schema includes the new nullable TEXT columns for specific lookups.
const schemaTmpl = `
    CREATE TABLE IF NOT EXISTS {{.TableName}} (
      {{.IDColumn}}            INTEGER PRIMARY KEY,
      {{.EventTypeColumn}}     TEXT,
      {{.ResourceTypeColumn}}  TEXT,
      {{.EventTSMillisColumn}} INTEGER NOT NULL,
      {{.EventDataColumn}}     BLOB,
      
      {{.ActorEmailColumn}}    TEXT,
      {{.ResourceIDColumn}}    TEXT,
      {{.ClusterIDColumn}}     TEXT
    ) STRICT;

    CREATE INDEX IF NOT EXISTS idx_{{.TableName}}_{{.EventTSMillisColumn}}
    ON {{.TableName}}({{.EventTSMillisColumn}});
`

type schemaParams struct {
	TableName           string
	IDColumn            string
	EventTypeColumn     string
	ResourceTypeColumn  string
	EventTSMillisColumn string
	EventDataColumn     string
	ActorEmailColumn    string
	ResourceIDColumn    string
	ClusterIDColumn     string
}

// Option configures optional Store behavior.
type Option func(*Store)

// WithCleanupCallback sets a callback that is called after cleanup with the number of deleted rows.
func WithCleanupCallback(cb func(int)) Option {
	return func(s *Store) {
		s.onCleanup = cb
	}
}

// Store is the SQLite-backed audit log store.
type Store struct {
	db                 *sqlitex.Pool
	logger             *zap.Logger
	onCleanup          func(int)
	timeout            time.Duration
	maxSize            uint64
	cleanupProbability float64
}

// NewStore creates a new audit log SQLite store.
func NewStore(ctx context.Context, db *sqlitex.Pool, timeout time.Duration, maxSize uint64, cleanupProbability float64, logger *zap.Logger, opts ...Option) (*Store, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	templateParams := schemaParams{
		TableName:           TableName,
		IDColumn:            idColumn,
		EventTypeColumn:     eventTypeColumn,
		ResourceTypeColumn:  resourceTypeColumn,
		EventTSMillisColumn: eventTSMillisColumn,
		EventDataColumn:     eventDataColumn,
		ActorEmailColumn:    actorEmailColumn,
		ResourceIDColumn:    resourceIDColumn,
		ClusterIDColumn:     clusterIDColumn,
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

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := db.Take(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer db.Put(conn)

	if err = sqlitex.ExecScript(conn, schemaSQL); err != nil {
		return nil, fmt.Errorf("failed to create sqlite log table schema: %w", err)
	}

	store := &Store{
		db:                 db,
		logger:             logger,
		timeout:            timeout,
		maxSize:            maxSize,
		cleanupProbability: cleanupProbability,
	}

	for _, opt := range opts {
		opt(store)
	}

	return store, nil
}

func (s *Store) Write(ctx context.Context, event auditlog.Event) error {
	query := fmt.Sprintf(`INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s) VALUES 
	($event_type, $resource_type, $event_ts_ms, $event_data, $actor_email, $resource_id, $cluster_id)`,
		TableName, eventTypeColumn, resourceTypeColumn, eventTSMillisColumn, eventDataColumn,
		actorEmailColumn, resourceIDColumn, clusterIDColumn)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		dataJSON              []byte
		actorEmail, clusterID string
	)

	if event.Data != nil {
		var err error

		if dataJSON, err = json.Marshal(event.Data); err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}

		if event.Data.Session.Email != "" {
			actorEmail = event.Data.Session.Email
		}

		clusterID = extractClusterID(event.Data)
	}

	conn, err := s.db.Take(ctx)
	if err != nil {
		return fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer s.db.Put(conn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.
		BindStringIfSet("$event_type", event.Type).
		BindStringIfSet("$resource_type", event.ResourceType).
		BindInt64("$event_ts_ms", event.TimeMillis).
		BindBytes("$event_data", dataJSON).
		BindStringIfSet("$actor_email", actorEmail).
		BindStringIfSet("$resource_id", event.ResourceID).
		BindStringIfSet("$cluster_id", clusterID).
		Exec()
	if err != nil {
		return fmt.Errorf("failed to write audit log event: %w", err)
	}

	if s.maxSize > 0 && rand.Float64() < s.cleanupProbability {
		if err := s.removeBySize(conn); err != nil {
			s.logger.Warn("failed to cleanup audit logs by size", zap.Error(err))
		}
	}

	return nil
}

func (s *Store) Remove(ctx context.Context, start, end time.Time) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE %s >= $start AND %s <= $end`, TableName, eventTSMillisColumn, eventTSMillisColumn)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	conn, err := s.db.Take(ctx)
	if err != nil {
		return fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer s.db.Put(conn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.
		BindInt64("$start", start.UnixMilli()).
		BindInt64("$end", end.UnixMilli()).
		Exec()
	if err != nil {
		return fmt.Errorf("failed to remove audit log events: %w", err)
	}

	if s.onCleanup != nil {
		s.onCleanup(conn.Changes())
	}

	return nil
}

func (s *Store) removeBySize(conn *zombiesqlite.Conn) error {
	sizeQuery := fmt.Sprintf(`SELECT SUM(pgsize) FROM dbstat WHERE name = '%s'`, TableName)

	q, err := sqlitexx.NewQuery(conn, sizeQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare dbstat query: %w", err)
	}

	var tableSize int64

	if err = q.QueryRow(func(stmt *zombiesqlite.Stmt) error {
		tableSize = stmt.ColumnInt64(0)

		return nil
	}); err != nil {
		return fmt.Errorf("failed to query table size: %w", err)
	}

	if tableSize <= int64(s.maxSize) {
		return nil
	}

	// Use min/max ID to estimate row count and compute the cutoff ID for deletion. This is much faster than COUNT(*) on large tables, and good enough for our probabilistic cleanup.
	rangeQuery := fmt.Sprintf(`SELECT COALESCE(MIN(%s), 0), COALESCE(MAX(%s), 0) FROM %s`,
		idColumn, idColumn, TableName)

	q, err = sqlitexx.NewQuery(conn, rangeQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare ID range query: %w", err)
	}

	var minID, maxID int64

	if err = q.QueryRow(func(stmt *zombiesqlite.Stmt) error {
		minID = stmt.ColumnInt64(0)
		maxID = stmt.ColumnInt64(1)

		return nil
	}); err != nil {
		return fmt.Errorf("failed to query ID range: %w", err)
	}

	rowCount := maxID - minID + 1
	if rowCount <= 0 {
		return nil
	}

	avgRowSize := tableSize / rowCount
	if avgRowSize <= 0 {
		avgRowSize = 1
	}

	// Ceiling division ensures at least 1 row is deleted when slightly over maxSize.
	excess := tableSize - int64(s.maxSize)
	rowsToDelete := (excess + avgRowSize - 1) / avgRowSize

	if rowsToDelete <= 0 {
		return nil
	}

	// Cap per invocation to keep each cleanup fast. The probabilistic trigger
	// on each Write ensures we converge to the target size over time.
	if rowsToDelete > removeBySizeBatchCap {
		rowsToDelete = removeBySizeBatchCap
	}

	// Compute cutoff ID from the min ID, then range-delete everything at or
	// below it. This lets SQLite use the primary key index directly.
	cutoffID := minID + rowsToDelete - 1

	deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE %s <= $cutoff_id`, TableName, idColumn)

	q, err = sqlitexx.NewQuery(conn, deleteQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare size-based delete query: %w", err)
	}

	if err = q.BindInt64("$cutoff_id", cutoffID).Exec(); err != nil {
		return fmt.Errorf("failed to delete oldest audit log events by size: %w", err)
	}

	if s.onCleanup != nil {
		s.onCleanup(conn.Changes())
	}

	return nil
}

func (s *Store) Reader(ctx context.Context, start, end time.Time) (auditlog.Reader, error) {
	// we take the connection here, but it will be released in logReader.Close()
	conn, err := s.db.Take(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s, %s, %s, %s, %s FROM %s WHERE %s >= $start AND %s <= $end ORDER BY %s ASC, %s ASC`, eventTypeColumn,
		resourceTypeColumn, resourceIDColumn, eventTSMillisColumn, eventDataColumn, TableName, eventTSMillisColumn, eventTSMillisColumn, eventTSMillisColumn, idColumn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		s.db.Put(conn)

		return nil, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	it := q.
		BindInt64("$start", start.UnixMilli()).
		BindInt64("$end", end.UnixMilli()).
		QueryIter()

	next, stop := iter.Pull2(it)

	return &logReader{
		conn: conn,
		db:   s.db,
		next: next,
		stop: stop,
	}, nil
}

func (s *Store) HasData(ctx context.Context) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", TableName)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	conn, err := s.db.Take(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer s.db.Put(conn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return false, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	err = q.QueryRow(func(*zombiesqlite.Stmt) error { return nil })
	if err != nil {
		if errors.Is(err, sqlitexx.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check for existing data: %w", err)
	}

	return true, nil
}

type logReader struct {
	conn *zombiesqlite.Conn
	db   *sqlitex.Pool
	next func() (*zombiesqlite.Stmt, error, bool)
	stop func()
}

func (l *logReader) Close() error {
	l.stop()

	l.db.Put(l.conn)

	return nil
}

// rawEvent is like auditlog.Event but with Data as json.RawMessage for efficiency, to avoid unnecessary unmarshal/marshal.
type rawEvent struct {
	Type         *string         `json:"event_type,omitempty"`
	ResourceType *resource.Type  `json:"resource_type,omitempty"`
	ResourceID   *resource.ID    `json:"resource_id,omitempty"`
	Data         json.RawMessage `json:"event_data,omitempty"`
	TimeMillis   int64           `json:"event_ts,omitempty"`
}

func (l *logReader) Read() ([]byte, error) {
	result, err, ok := l.next()
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log event: %w", err)
	}

	if !ok {
		return nil, io.EOF
	}

	var dataJSON []byte

	var event rawEvent

	if !result.IsNull(eventTypeColumn) {
		event.Type = pointer.To(result.GetText(eventTypeColumn))
	}

	if !result.IsNull(resourceTypeColumn) {
		event.ResourceType = pointer.To(result.GetText(resourceTypeColumn))
	}

	if !result.IsNull(resourceIDColumn) {
		event.ResourceID = pointer.To(result.GetText(resourceIDColumn))
	}

	if !result.IsNull(eventDataColumn) {
		dataJSON = make([]byte, result.GetLen(eventDataColumn))
		result.GetBytes(eventDataColumn, dataJSON)

		event.Data = dataJSON
	}

	event.TimeMillis = result.GetInt64(eventTSMillisColumn)

	marshaled, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal audit log event: %w", err)
	}

	return append(marshaled, '\n'), nil
}

func extractClusterID(d *auditlog.Data) string {
	switch {
	case d.Cluster != nil:
		return d.Cluster.ID
	case d.K8SAccess != nil:
		return d.K8SAccess.ClusterName
	case d.TalosAccess != nil:
		return d.TalosAccess.ClusterName
	case d.MachineSet != nil:
		return d.MachineSet.ClusterID
	case d.MachineSetNode != nil:
		return d.MachineSetNode.ClusterID
	case d.ConfigPatch != nil:
		return d.ConfigPatch.ClusterID
	case d.MachineConfigDiff != nil:
		return d.MachineConfigDiff.ClusterID
	default:
		return ""
	}
}
