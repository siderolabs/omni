// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sqlite

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/internal/backend/discovery"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogsqlite"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/sqlitelog"
)

const (
	defaultRefreshInterval = 60 * time.Second
	queryTimeout           = 10 * time.Second
)

// Subsystem names used as label values.
const (
	SubsystemAuditLogs   = "audit_logs"
	SubsystemMachineLogs = "machine_logs"
	SubsystemDiscovery   = "discovery"
	SubsystemState       = "state"
)

// subsystemTables maps each Omni subsystem to the SQLite tables it owns.
// The state subsystem is handled separately via sqlState, which implements DBSize to satisfy sqlite.State.
var subsystemTables = map[string][]string{
	SubsystemAuditLogs:   {auditlogsqlite.TableName},
	SubsystemMachineLogs: {sqlitelog.TableName},
	SubsystemDiscovery:   {discovery.TableName},
}

type sqlState interface {
	DBSize(ctx context.Context) (int64, error)
}

// Metrics implements prometheus.Collector to expose SQLite database metrics.
type Metrics struct {
	lastRefresh              time.Time
	sqlState                 sqlState
	subsystemRowCountDesc    *prometheus.Desc
	dbSizeDesc               *prometheus.Desc
	subsystemSizeDesc        *prometheus.Desc
	cleanupRowsDeleted       *prometheus.CounterVec
	db                       *sqlitex.Pool
	logger                   *zap.Logger
	cachedSubsystemSizes     map[string]float64
	cachedSubsystemRowCounts map[string]float64
	refreshInterval          time.Duration
	cachedDBSize             float64
	mu                       sync.Mutex
}

var _ prometheus.Collector = &Metrics{}

// MetricsOption configures optional metrics behavior.
type MetricsOption func(*Metrics)

// WithRefreshInterval sets the cache refresh interval. Default is 60s.
func WithRefreshInterval(d time.Duration) MetricsOption {
	return func(m *Metrics) {
		m.refreshInterval = d
	}
}

// NewMetrics creates a *Metrics that exposes SQLite database metrics.
// If cosiState implements sqlState, the state subsystem size is reported via its DBSize method.
func NewMetrics(db *sqlitex.Pool, cosiState state.CoreState, logger *zap.Logger, opts ...MetricsOption) *Metrics {
	dbSizer, ok := cosiState.(sqlState)
	if !ok {
		logger.Warn("COSI state does not implement sqlState, state subsystem size will not be reported")
	}

	m := &Metrics{
		db:              db,
		sqlState:        dbSizer,
		logger:          logger,
		refreshInterval: defaultRefreshInterval,

		dbSizeDesc: prometheus.NewDesc(
			"omni_sqlite_db_size_bytes",
			"Total size of the SQLite database in bytes.",
			nil, nil,
		),
		subsystemSizeDesc: prometheus.NewDesc(
			"omni_sqlite_subsystem_size_bytes",
			"Size of a subsystem's tables in the SQLite database in bytes.",
			[]string{"subsystem"}, nil,
		),
		subsystemRowCountDesc: prometheus.NewDesc(
			"omni_sqlite_subsystem_row_count",
			"Total number of rows across a subsystem's tables.",
			[]string{"subsystem"}, nil,
		),
		cleanupRowsDeleted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_sqlite_cleanup_rows_deleted_total",
			Help: "Total rows deleted by cleanup.",
		}, []string{"subsystem"}),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// CleanupCallback returns a callback that increments the cleanup rows deleted counter for the given subsystem.
func (m *Metrics) CleanupCallback(subsystem string) func(int) {
	return func(count int) {
		if count > 0 {
			m.cleanupRowsDeleted.WithLabelValues(subsystem).Add(float64(count))
		}
	}
}

// Describe implements prometheus.Collector using DescribeByCollect.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(m, ch)
}

// Collect implements prometheus.Collector.
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if time.Since(m.lastRefresh) > m.refreshInterval {
		m.refresh()
	}

	ch <- prometheus.MustNewConstMetric(m.dbSizeDesc, prometheus.GaugeValue, m.cachedDBSize)

	for subsystem, size := range m.cachedSubsystemSizes {
		ch <- prometheus.MustNewConstMetric(m.subsystemSizeDesc, prometheus.GaugeValue, size, subsystem)
	}

	for subsystem, count := range m.cachedSubsystemRowCounts {
		ch <- prometheus.MustNewConstMetric(m.subsystemRowCountDesc, prometheus.GaugeValue, count, subsystem)
	}

	m.cleanupRowsDeleted.Collect(ch)
}

// refresh queries the database and updates cached values.
// On failure, lastRefresh is still updated to avoid retrying on every scrape.
func (m *Metrics) refresh() {
	// Always update lastRefresh to prevent tight retry loops when the DB is unavailable.
	defer func() { m.lastRefresh = time.Now() }()

	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	conn, err := m.db.Take(ctx)
	if err != nil {
		m.logger.Warn("failed to take connection for metrics refresh", zap.Error(err))

		return
	}

	defer m.db.Put(conn)

	existingTables, err := m.discoverTables(conn)
	if err != nil {
		m.logger.Warn("failed to discover tables for metrics", zap.Error(err))

		return
	}

	dbSize, err := m.queryDBSize(conn)
	if err != nil {
		m.logger.Warn("failed to query db size for metrics", zap.Error(err))

		return
	}

	tableSizes, err := m.queryTableSizes(conn)
	if err != nil {
		m.logger.Warn("failed to query table sizes for metrics", zap.Error(err))

		return
	}

	subsystemSizes := make(map[string]float64, len(subsystemTables)+1)
	subsystemRowCounts := make(map[string]float64, len(subsystemTables)+1)

	for subsystem, tables := range subsystemTables {
		var totalSize float64

		for _, table := range tables {
			totalSize += tableSizes[table] // 0 if table doesn't exist in dbstat
		}

		subsystemSizes[subsystem] = totalSize

		rowCount, rowCountErr := m.querySubsystemRowCount(conn, tables, existingTables)
		if rowCountErr != nil {
			m.logger.Warn("failed to query row count for subsystem", zap.String("subsystem", subsystem), zap.Error(rowCountErr))

			return
		}

		subsystemRowCounts[subsystem] = rowCount
	}

	// State subsystem: use the COSI sqlState for size if available.
	// Row count is not reported for the state subsystem because it uses a separate database.
	if m.sqlState != nil {
		stateSize, sizeErr := m.sqlState.DBSize(ctx)
		if sizeErr != nil {
			m.logger.Warn("failed to query state subsystem size", zap.Error(sizeErr))

			return
		}

		subsystemSizes[SubsystemState] = float64(stateSize)
	}

	m.cachedDBSize = dbSize
	m.cachedSubsystemSizes = subsystemSizes
	m.cachedSubsystemRowCounts = subsystemRowCounts
}

func (m *Metrics) discoverTables(conn *zombiesqlite.Conn) (map[string]struct{}, error) {
	q, err := sqlitexx.NewQuery(conn, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare table discovery query: %w", err)
	}

	tables := make(map[string]struct{})

	if err = q.QueryAll(func(stmt *zombiesqlite.Stmt) error {
		tables[stmt.ColumnText(0)] = struct{}{}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}

	return tables, nil
}

func (m *Metrics) queryDBSize(conn *zombiesqlite.Conn) (float64, error) {
	q, err := sqlitexx.NewQuery(conn, `SELECT COALESCE(SUM(pgsize), 0) FROM dbstat`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare db size query: %w", err)
	}

	var size float64

	if err = q.QueryRow(func(stmt *zombiesqlite.Stmt) error {
		size = stmt.ColumnFloat(0)

		return nil
	}); err != nil {
		return 0, fmt.Errorf("failed to query db size: %w", err)
	}

	return size, nil
}

func (m *Metrics) queryTableSizes(conn *zombiesqlite.Conn) (map[string]float64, error) {
	q, err := sqlitexx.NewQuery(conn, `SELECT name, SUM(pgsize) FROM dbstat GROUP BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare table sizes query: %w", err)
	}

	sizes := make(map[string]float64)

	if err = q.QueryAll(func(stmt *zombiesqlite.Stmt) error {
		sizes[stmt.ColumnText(0)] = stmt.ColumnFloat(1)

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to query table sizes: %w", err)
	}

	return sizes, nil
}

// querySubsystemRowCount returns the total row count across all existing tables for a subsystem.
func (m *Metrics) querySubsystemRowCount(conn *zombiesqlite.Conn, tables []string, existingTables map[string]struct{}) (float64, error) {
	var total float64

	for _, table := range tables {
		if _, ok := existingTables[table]; !ok {
			continue
		}

		// Table names come from package-level constants, so they are safe to interpolate.
		q, err := sqlitexx.NewQuery(conn, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, table))
		if err != nil {
			return 0, fmt.Errorf("failed to prepare row count query for %q: %w", table, err)
		}

		if err = q.QueryRow(func(stmt *zombiesqlite.Stmt) error {
			total += stmt.ColumnFloat(0)

			return nil
		}); err != nil {
			return 0, fmt.Errorf("failed to query row count for %q: %w", table, err)
		}
	}

	return total, nil
}
