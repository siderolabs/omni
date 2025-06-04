// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

// Features contains all Omni feature flags.
type Features struct {
	EnableTalosPreReleaseVersions bool `yaml:"enableTalosPreReleaseVersions"`
	EnableBreakGlassConfigs       bool `yaml:"enableBreakGlassConfigs"`
	EnableConfigDataCompression   bool `yaml:"enableConfigDataCompression"`

	DisableControllerRuntimeCache bool `yaml:"disableControllerRuntimeCache"`
}
