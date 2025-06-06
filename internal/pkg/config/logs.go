// Copyright (c) 2025 Sidero Labs, Inc.
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
	// Storage configures persistent machine log storage of the Omni instance.
	Storage LogsMachineStorage `yaml:"storage"`

	BufferInitialCapacity int `yaml:"bufferInitialCapacity"`
	BufferMaxCapacity     int `yaml:"bufferMaxCapacity"`
	BufferSafetyGap       int `yaml:"bufferSafetyGap"`
}

// LogsMachineStorage configures the machine logs storage.
//
//nolint:govet
type LogsMachineStorage struct {
	Enabled bool `yaml:"enabled"`
	// Path to store the logs in.
	Path string `yaml:"path"`
	// FlushPeriod is the period to use to flush the logs to disk.
	FlushPeriod time.Duration `yaml:"flushPeriod"`
	// FlushJitter flush period jitter.
	FlushJitter float64 `yaml:"flushJitter"`
	// NumCompressedChunks is the count of log chunks to keep in the logs history.
	NumCompressedChunks int `yaml:"numCompressedChunks"`
}

// LogsAudit configures audit logs peristence.
type LogsAudit struct {
	// Path to store the audit logs in.
	Path string `yaml:"path"`
}

// ResourceLoggerConfig is the config for the Omni resource logger.
// This is the debug tool, that allows logging all resource changes to the stdout.
type ResourceLoggerConfig struct {
	// LogLevel is the level of the logs to use when writing the data.
	LogLevel string `yaml:"logLevel"`
	// Types is the list of the resource types to log to stdout.
	Types []string `yaml:"types"`
}

// LogsStripe report usage metrics to stripe.
type LogsStripe struct {
	Enabled bool `yaml:"enabled"`
}
