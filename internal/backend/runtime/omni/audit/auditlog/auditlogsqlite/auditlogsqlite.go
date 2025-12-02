// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auditlogsqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	_ "modernc.org/sqlite"

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
	db      *sql.DB
	timeout time.Duration
}

func NewStore(ctx context.Context, db *sql.DB, timeout time.Duration) (*Store, error) {
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

	if _, err = db.ExecContext(ctx, schemaSQL); err != nil {
		return nil, fmt.Errorf("failed to create sqlite log table schema: %w", err)
	}

	return &Store{db: db, timeout: timeout}, nil
}

func (s *Store) Write(ctx context.Context, event auditlog.Event) error {
	query := fmt.Sprintf(`INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tableName, eventTypeColumn, resourceTypeColumn, eventTSMillisColumn, eventDataColumn,
		actorEmailColumn, resourceIDColumn, clusterIDColumn)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		dataJSON                     []byte
		actorEmail, resID, clusterID string
	)

	if event.Data != nil {
		var err error

		if dataJSON, err = json.Marshal(event.Data); err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}

		if event.Data.Session.Email != "" {
			actorEmail = event.Data.Session.Email
		}

		resID = extractResourceID(event.Data)
		clusterID = extractClusterID(event.Data)
	}

	if _, err := s.db.ExecContext(ctx, query, strPtr(event.Type), strPtr(event.ResourceType), event.TimeMillis, dataJSON,
		strPtr(actorEmail), strPtr(resID), strPtr(clusterID)); err != nil {
		return fmt.Errorf("failed to write audit log event: %w", err)
	}

	return nil
}

func (s *Store) Remove(ctx context.Context, start, end time.Time) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE %s >= ? AND %s <= ?`, tableName, eventTSMillisColumn, eventTSMillisColumn)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if _, err := s.db.ExecContext(ctx, query, start.UnixMilli(), end.UnixMilli()); err != nil {
		return fmt.Errorf("failed to remove audit log events: %w", err)
	}

	return nil
}

func (s *Store) Reader(ctx context.Context, start, end time.Time) (auditlog.Reader, error) {
	query := fmt.Sprintf(`SELECT %s, %s, %s, %s FROM %s WHERE %s >= ? AND %s <= ? ORDER BY %s ASC, %s ASC`, eventTypeColumn,
		resourceTypeColumn, eventTSMillisColumn, eventDataColumn, tableName, eventTSMillisColumn, eventTSMillisColumn, eventTSMillisColumn, idColumn)

	rows, err := s.db.QueryContext(ctx, query, start.UnixMilli(), end.UnixMilli()) //nolint:rowserrcheck // false positive, we check for .Err() in Read() and Close().
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log events: %w", err)
	}

	return &logReader{rows: rows}, nil
}

func (s *Store) HasData(ctx context.Context) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", tableName)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var n int
	if err := s.db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check for existing data: %w", err)
	}

	return true, nil
}

type logReader struct {
	rows *sql.Rows
}

func (l *logReader) Close() error {
	var closeErr, rowsErr error

	if err := l.rows.Close(); err != nil {
		closeErr = fmt.Errorf("failed to close rows: %w", err)
	}

	if err := l.rows.Err(); err != nil {
		rowsErr = fmt.Errorf("rows error: %w", err)
	}

	return errors.Join(closeErr, rowsErr)
}

// rawEvent is like auditlog.Event but with Data as json.RawMessage for efficiency, to avoid unnecessary unmarshal/marshal.
type rawEvent struct {
	Type         *string         `json:"event_type,omitempty"`
	ResourceType *resource.Type  `json:"resource_type,omitempty"`
	Data         json.RawMessage `json:"event_data,omitempty"`
	TimeMillis   int64           `json:"event_ts,omitempty"`
}

func (l *logReader) Read() ([]byte, error) {
	if !l.rows.Next() {
		if err := l.rows.Err(); err != nil {
			return nil, fmt.Errorf("failed to read audit log event: %w", err)
		}

		return nil, io.EOF
	}

	var dataJSON []byte

	var event rawEvent

	if err := l.rows.Scan(&event.Type, &event.ResourceType, &event.TimeMillis, &dataJSON); err != nil {
		return nil, fmt.Errorf("failed to scan audit log event: %w", err)
	}

	event.Data = dataJSON

	marshaled, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal audit log event: %w", err)
	}

	return append(marshaled, '\n'), nil
}

func extractResourceID(d *auditlog.Data) string {
	if d == nil {
		return ""
	}

	switch {
	case d.NewUser != nil:
		return d.NewUser.UserID
	case d.Machine != nil:
		return d.Machine.ID
	case d.MachineLabels != nil:
		return d.MachineLabels.ID
	case d.AccessPolicy != nil:
		return d.AccessPolicy.ID
	case d.Cluster != nil:
		return d.Cluster.ID
	case d.MachineSet != nil:
		return d.MachineSet.ID
	case d.MachineSetNode != nil:
		return d.MachineSetNode.ID
	case d.ConfigPatch != nil:
		return d.ConfigPatch.ID
	case d.MachineConfigDiff != nil:
		return d.MachineConfigDiff.ID
	default:
		return ""
	}
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

func strPtr(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
