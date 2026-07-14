// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package auditlogsqlite implements an SQLite-backed audit log store.
package auditlogsqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"go.uber.org/zap"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
)

const (
	// removeBatchSize is the maximum number of rows deleted per batch in Remove and removeBySize.
	removeBatchSize = 1000

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
	db        *sqlitexx.Pool
	logger    *zap.Logger
	onCleanup func(int)

	// subscribers holds the follower wakeup channels, guarded by subscribersMu.
	subscribers   []chan struct{}
	subscribersMu sync.Mutex

	timeout            time.Duration
	maxSize            uint64
	cleanupProbability float64
}

// NewStore creates a new audit log SQLite store.
func NewStore(ctx context.Context, db *sqlitexx.Pool, timeout time.Duration, maxSize uint64, cleanupProbability float64, logger *zap.Logger, opts ...Option) (*Store, error) {
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

	// waking the followers before the opportunistic cleanup is safe: cleanup spares the
	// newest event, so it can never remove the one just announced
	s.notifySubscribers()

	if s.maxSize > 0 && rand.Float64() < s.cleanupProbability {
		if err := s.removeBySize(conn); err != nil {
			s.logger.Warn("failed to cleanup audit logs by size", zap.Error(err))
		}
	}

	return nil
}

// Remove deletes audit log events in the given time range in batches of removeBatchSize.
// Batching keeps each autocommit DELETE small, releasing the SQLite write lock between
// statements so other writers sharing the same database are not blocked for long.
//
// The event with the highest id is never deleted, no matter the range: without it, SQLite
// would reuse its id for the next event, and the ids serve as the positions of the follow
// streams, which reused ids would silently corrupt.
func (s *Store) Remove(ctx context.Context, start, end time.Time) error {
	// DELETE ... LIMIT is not supported (requires SQLITE_ENABLE_UPDATE_DELETE_LIMIT), so we use a subquery to select the IDs to delete.
	query := fmt.Sprintf(
		`DELETE FROM %s WHERE %s IN (SELECT %s FROM %s WHERE %s >= $start AND %s <= $end AND %s < (SELECT MAX(%s) FROM %s) LIMIT $limit)`,
		TableName, idColumn, idColumn, TableName, eventTSMillisColumn, eventTSMillisColumn, idColumn, idColumn, TableName,
	)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	conn, err := s.db.Take(ctx)
	if err != nil {
		return fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer s.db.Put(conn)

	var totalDeleted int

	for {
		q, qErr := sqlitexx.NewQuery(conn, query)
		if qErr != nil {
			return fmt.Errorf("failed to prepare sqlite statement: %w", qErr)
		}

		qErr = q.
			BindInt64("$start", start.UnixMilli()).
			BindInt64("$end", end.UnixMilli()).
			BindInt64("$limit", removeBatchSize).
			Exec()
		if qErr != nil {
			return fmt.Errorf("failed to remove audit log events: %w", qErr)
		}

		deleted := conn.Changes()
		totalDeleted += deleted

		if deleted == 0 || ctx.Err() != nil {
			break
		}
	}

	if s.onCleanup != nil && totalDeleted > 0 {
		s.onCleanup(totalDeleted)
	}

	return nil
}

func (s *Store) removeBySize(conn *zombiesqlite.Conn) error {
	sizeQuery := fmt.Sprintf(`SELECT COALESCE(SUM(d.pgsize), 0) FROM dbstat d JOIN sqlite_master m ON d.name = m.name WHERE m.tbl_name = '%s'`, TableName)

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
	if rowsToDelete > removeBatchSize {
		rowsToDelete = removeBatchSize
	}

	// Compute cutoff ID from the min ID, then range-delete everything at or
	// below it. This lets SQLite use the primary key index directly.
	cutoffID := minID + rowsToDelete - 1

	// The event with the highest id is never deleted: without it, SQLite would reuse its id
	// for the next event, corrupting the follow stream positions the ids serve as.
	if cutoffID >= maxID {
		cutoffID = maxID - 1
	}

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

func (s *Store) Reader(ctx context.Context, filters auditlog.ReadFilters) (auditlog.Reader, error) {
	// we take the connection here, but it will be released in logReader.Close()
	conn, err := s.db.Take(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	conditions := []string{
		fmt.Sprintf("%s >= $start", eventTSMillisColumn),
		fmt.Sprintf("%s <= $end", eventTSMillisColumn),
	}

	if filters.Search != "" {
		searchCols := []string{
			eventTypeColumn,
			resourceTypeColumn,
			actorEmailColumn,
			resourceIDColumn,
			clusterIDColumn,
			fmt.Sprintf("CAST(%s AS TEXT)", eventDataColumn),
		}

		orParts := make([]string, 0, len(searchCols))

		for _, col := range searchCols {
			orParts = append(orParts, fmt.Sprintf("%s LIKE '%%' || $search || '%%'", col))
		}

		conditions = append(conditions, "("+strings.Join(orParts, " OR ")+")")
	}

	eventTypeSQLStr := filters.EventType.SQLString()
	if eventTypeSQLStr != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $event_type", eventTypeColumn))
	}

	if filters.ResourceType != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $resource_type", resourceTypeColumn))
	}

	if filters.ResourceID != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $resource_id", resourceIDColumn))
	}

	if filters.ClusterID != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $cluster_id", clusterIDColumn))
	}

	if filters.Actor != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $actor", actorEmailColumn))
	}

	orderByCol := orderByFieldColumn(filters.OrderByField)
	orderByDir := orderByDirSQL(filters.OrderByDir)

	query := fmt.Sprintf(
		`SELECT %s, %s, %s, %s, %s FROM %s WHERE %s ORDER BY %s %s, %s ASC`,
		eventTypeColumn, resourceTypeColumn, resourceIDColumn, eventTSMillisColumn, eventDataColumn,
		TableName,
		strings.Join(conditions, " AND "),
		orderByCol, orderByDir,
		idColumn,
	)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		s.db.Put(conn)

		return nil, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	it := q.
		BindInt64("$start", filters.Start.UnixMilli()).
		BindInt64("$end", filters.End.UnixMilli()).
		BindStringIfSet("$search", filters.Search).
		BindStringIfSet("$event_type", eventTypeSQLStr).
		BindStringIfSet("$resource_type", filters.ResourceType).
		BindStringIfSet("$resource_id", filters.ResourceID).
		BindStringIfSet("$cluster_id", filters.ClusterID).
		BindStringIfSet("$actor", filters.Actor).
		QueryIter()

	next, stop := iter.Pull2(it)

	return &logReader{
		conn: conn,
		db:   s.db,
		next: next,
		stop: stop,
	}, nil
}

// FollowStart resolves the initial follow position for the given inclusive start timestamp:
// the position just below the first event at or after it, so that following from the returned
// position delivers that event first. A zero start, or a start beyond every stored event,
// resolves to the current tail exactly, delivering only events written afterwards.
//
// The timestamp resolution is best-effort under non-monotonic clocks: the ids follow the
// insertion order, and an event written under a stepped-back clock can carry a timestamp
// below its predecessors. Exactness is what the id positions are for.
func (s *Store) FollowStart(ctx context.Context, startTsMs int64) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	conn, err := s.db.Take(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer s.db.Put(conn)

	// A single statement resolves each case in one consistent snapshot: events committed
	// afterwards have higher ids than anything it observed, so the following FollowBatch
	// calls pick them up and nothing can fall between the lookup and the returned position.
	query := fmt.Sprintf("SELECT COALESCE(MAX(%s), 0) AS pos FROM %s", idColumn, TableName)
	if startTsMs > 0 {
		query = fmt.Sprintf(`SELECT COALESCE(
			(SELECT MIN(%s) - 1 FROM %s WHERE %s >= $start),
			(SELECT COALESCE(MAX(%s), 0) FROM %s)
		) AS pos`,
			idColumn, TableName, eventTSMillisColumn, idColumn, TableName)
	}

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	if startTsMs > 0 {
		q = q.BindInt64("$start", startTsMs)
	}

	var pos int64

	if err = q.QueryRow(func(stmt *zombiesqlite.Stmt) error {
		pos = stmt.GetInt64("pos")

		return nil
	}); err != nil {
		return 0, fmt.Errorf("failed to resolve the follow start position: %w", err)
	}

	return pos, nil
}

// FollowSubscribe registers a wakeup channel signaled after every write, for a follower to
// wait on between [Store.FollowBatch] calls instead of polling. The channel is buffered to
// one, so a wakeup arriving while the follower is busy is kept, never lost: subscribing
// before a scan therefore guarantees that no event written after that scan goes unnoticed.
// The returned function unsubscribes and must be called when the follower is done.
func (s *Store) FollowSubscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	s.subscribersMu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.subscribersMu.Unlock()

	return ch, func() {
		s.subscribersMu.Lock()
		defer s.subscribersMu.Unlock()

		s.subscribers = slices.DeleteFunc(s.subscribers, func(c chan struct{}) bool { return c == ch })
	}
}

// notifySubscribers wakes every follower after a write, without blocking on the ones that
// are already woken.
func (s *Store) notifySubscribers() {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- struct{}{}:
		default: // the channel already holds a wakeup
		}
	}
}

// FollowBatch reads up to limit events with ids above afterID, oldest first. The events are
// fully materialized, holding a database connection only for the read itself. Receiving
// fewer events than the limit means the log is exhausted for now: the caller waits before
// the next call, keeping its position at the last event it received.
//
// When the position no longer exists because every event at or above it was cleaned up and
// the storage ids restarted below it, FollowBatch returns [auditlog.ErrFollowPositionLost]:
// without this, a follower would wait blindly for the restarted ids to catch up.
func (s *Store) FollowBatch(ctx context.Context, afterID int64, limit int64) ([]auditlog.Entry, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	conn, err := s.db.Take(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	defer s.db.Put(conn)

	query := fmt.Sprintf(`SELECT %s, %s, %s, %s, %s, %s FROM %s WHERE %s > $after_id ORDER BY %s ASC LIMIT $limit`,
		idColumn, eventTypeColumn, resourceTypeColumn, resourceIDColumn, eventTSMillisColumn, eventDataColumn,
		TableName, idColumn, idColumn)

	q, err := sqlitexx.NewQuery(conn, query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	var entries []auditlog.Entry

	for stmt, iterErr := range q.
		BindInt64("$after_id", afterID).
		BindInt64("$limit", limit).
		QueryIter() {
		if iterErr != nil {
			return nil, fmt.Errorf("failed to read audit log event: %w", iterErr)
		}

		payload, marshalErr := marshalEventRow(stmt)
		if marshalErr != nil {
			return nil, marshalErr
		}

		entries = append(entries, auditlog.Entry{
			ID:      stmt.GetInt64(idColumn),
			Payload: payload,
		})
	}

	if len(entries) == 0 && afterID > 0 {
		maxID, maxErr := readMaxID(conn)
		if maxErr != nil {
			return nil, maxErr
		}

		if maxID < afterID {
			return nil, auditlog.ErrFollowPositionLost
		}
	}

	return entries, nil
}

// readMaxID returns the highest id in the audit log table, zero when the table is empty.
func readMaxID(conn *zombiesqlite.Conn) (int64, error) {
	q, err := sqlitexx.NewQuery(conn, fmt.Sprintf("SELECT COALESCE(MAX(%s), 0) AS max_id FROM %s", idColumn, TableName))
	if err != nil {
		return 0, fmt.Errorf("failed to prepare sqlite statement: %w", err)
	}

	var maxID int64

	if err = q.QueryRow(func(stmt *zombiesqlite.Stmt) error {
		maxID = stmt.GetInt64("max_id")

		return nil
	}); err != nil {
		return 0, fmt.Errorf("failed to read the max audit log event id: %w", err)
	}

	return maxID, nil
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
	db   *sqlitexx.Pool
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

	return marshalEventRow(result)
}

// marshalEventRow marshals the audit log event in the current statement row into its
// newline-terminated JSON payload.
func marshalEventRow(result *zombiesqlite.Stmt) ([]byte, error) {
	var dataJSON []byte

	var event rawEvent

	if !result.IsNull(eventTypeColumn) {
		event.Type = new(result.GetText(eventTypeColumn))
	}

	if !result.IsNull(resourceTypeColumn) {
		event.ResourceType = new(result.GetText(resourceTypeColumn))
	}

	if !result.IsNull(resourceIDColumn) {
		event.ResourceID = new(result.GetText(resourceIDColumn))
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

func orderByFieldColumn(f auditlog.OrderByField) string {
	switch f {
	case auditlog.OrderByFieldUnspecified:
		return eventTSMillisColumn
	case auditlog.OrderByFieldDate:
		return eventTSMillisColumn
	case auditlog.OrderByFieldEventType:
		return eventTypeColumn
	case auditlog.OrderByFieldResourceType:
		return resourceTypeColumn
	case auditlog.OrderByFieldResourceID:
		return resourceIDColumn
	case auditlog.OrderByFieldClusterID:
		return clusterIDColumn
	case auditlog.OrderByFieldActor:
		return actorEmailColumn
	}

	return eventTSMillisColumn
}

func orderByDirSQL(d auditlog.OrderByDir) string {
	if d == auditlog.OrderByDirDESC {
		return "DESC"
	}

	return "ASC"
}

func extractClusterID(d *auditlog.Data) string {
	switch {
	case d.Cluster != nil:
		return d.Cluster.ID
	case d.K8SAccess != nil:
		return d.K8SAccess.ClusterName
	case d.TalosAccess != nil:
		return d.TalosAccess.ClusterName
	case d.AuditLogAccess != nil:
		return d.AuditLogAccess.ClusterID
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
