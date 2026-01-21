// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import "time"

// Logs configures logging of the Omni instance.
//
//nolint:govet
type Logs struct {
	// Machine configures Talos machine logs handler.
	Machine LogsMachine `yaml:"machine"`
	// Audit configures audit logs handler.
	Audit LogsAudit `yaml:"audit"`
	// ResourceLogger configures resource logger.
	ResourceLogger ResourceLoggerConfig `yaml:"resourceLogger"`
	// Stripe enables reporting to stripe.
	Stripe LogsStripe `yaml:"stripe"`
}

// LogsMachine configures Talos machine logs handler.
type LogsMachine struct {
	// Storage configures the machine logs storage if SQLite storage is not enabled.
	Storage LogsMachineStorage `yaml:"storage"`

	// BufferInitialCapacity is the initial capacity of the in-memory buffer for logs.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	BufferInitialCapacity int `yaml:"bufferInitialCapacity"`
	// BufferMaxCapacity is the maximum capacity of the in-memory buffer for logs.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	BufferMaxCapacity int `yaml:"bufferMaxCapacity"`
	// BufferSafetyGap is the safety gap to use when trimming the buffer.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	BufferSafetyGap int `yaml:"bufferSafetyGap"`
}

// LogsMachineStorage configures the machine logs storage if SQLite storage is not enabled.
//
//nolint:govet
type LogsMachineStorage struct {
	// Enabled indicates whether the storage is enabled.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	Enabled bool `yaml:"enabled"`

	// Path to store the logs in.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	Path string `yaml:"path"`

	// FlushPeriod is the period to use to flush the logs to disk.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	FlushPeriod time.Duration `yaml:"flushPeriod"`

	// FlushJitter flush period jitter.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	FlushJitter float64 `yaml:"flushJitter"`

	// NumCompressedChunks is the count of log chunks to keep in the logs history.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	NumCompressedChunks int `yaml:"numCompressedChunks"`

	SQLiteTimeout    time.Duration `yaml:"sqliteTimeout"`
	CleanupInterval  time.Duration `yaml:"cleanupInterval"`
	CleanupOlderThan time.Duration `yaml:"cleanupOlderThan"`

	MaxLinesPerMachine int     `yaml:"maxLinesPerMachine"`
	CleanupProbability float64 `yaml:"cleanupProbability"`
}

// LogsAudit configures audit logs persistence.
type LogsAudit struct {
	// Enabled indicates whether audit logging is enabled.
	Enabled *bool `yaml:"enabled"`

	// Path to store the audit logs in.
	//
	// Deprecated: this field is kept for the SQLite migration, and will be removed in future versions.
	Path string `yaml:"path"`

	// SQLiteTimeout is the timeout for SQLite operations.
	SQLiteTimeout time.Duration `yaml:"sqliteTimeout"`
}

// ResourceLoggerConfig is the config for the Omni resource logger.
// This is the debug tool, that allows logging all resource changes to the stdout.
type ResourceLoggerConfig struct {
	// LogLevel is the level of the logs to use when writing the data.
	LogLevel string `yaml:"logLevel"`
	// Types is the list of the resource types to log to stdout.
	Types []string `yaml:"types" merge:"replace"`
}

// LogsStripe report usage metrics to stripe.
type LogsStripe struct {
	Enabled   bool   `yaml:"enabled"`
	MinCommit uint32 `yaml:"minCommit"`
}
