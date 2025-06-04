// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

// Debug configures debugging tools of the Omni instance.
type Debug struct {
	Server DebugServer `yaml:"server"`
	Pprof  DebugPprof  `yaml:"pprof"`
}

// DebugServer enables the debug server.
type DebugServer struct {
	Endpoint string `yaml:"endpoint"`
}

// DebugPprof enables pprof server.
type DebugPprof struct {
	Endpoint string `yaml:"endpoint"`
}
