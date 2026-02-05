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
	"strings"
	"text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"github.com/siderolabs/go-pointer"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
)

const (
	tableName           = "audit_logs"
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

type Store struct {
	db      *sqlitex.Pool
	timeout time.Duration
}

func NewStore(ctx context.Context, db *sqlitex.Pool, timeout time.Duration) (*Store, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	templateParams := schemaParams{
		TableName:           tableName,
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

	return &Store{db: db, timeout: timeout}, nil
}

func (s *Store) Write(ctx context.Context, event auditlog.Event) error {
	query := fmt.Sprintf(`INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s) VALUES 
	($event_type, $resource_type, $event_ts_ms, $event_data, $actor_email, $resource_id, $cluster_id)`,
		tableName, eventTypeColumn, resourceTypeColumn, eventTSMillisColumn, eventDataColumn,
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

	return nil
}

func (s *Store) Remove(ctx context.Context, start, end time.Time) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE %s >= $start AND %s <= $end`, tableName, eventTSMillisColumn, eventTSMillisColumn)

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

	return nil
}

func (s *Store) Reader(ctx context.Context, start, end time.Time) (auditlog.Reader, error) {
	// we take the connection here, but it will be released in logReader.Close()
	conn, err := s.db.Take(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take connection from pool: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s, %s, %s, %s, %s FROM %s WHERE %s >= $start AND %s <= $end ORDER BY %s ASC, %s ASC`, eventTypeColumn,
		resourceTypeColumn, resourceIDColumn, eventTSMillisColumn, eventDataColumn, tableName, eventTSMillisColumn, eventTSMillisColumn, eventTSMillisColumn, idColumn)

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
	query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", tableName)

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
