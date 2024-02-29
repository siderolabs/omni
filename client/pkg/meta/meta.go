// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package meta keeps Talos meta partition utils.
package meta

// StateEncryptionConfig is github.com/siderolabs/talos/internal/pkg/meta.StateEncryptionConfig.
const StateEncryptionConfig = 9

// MetalNetworkPlatformConfig is github.com/siderolabs/talos/internal/pkg/meta.MetalNetworkPlatformConfig.
// tsgen:MetalNetworkPlatformConfig
const MetalNetworkPlatformConfig = 10

// LabelsMeta is github.com/siderolabs/talos/internal/pkg/meta.UserReserved1.
// Omni stores initial machine labels under that key.
// tsgen:LabelsMeta
const LabelsMeta = 12

// UserReserved2 is github.com/siderolabs/talos/internal/pkg/meta.UserReserved2.
const UserReserved2 = 13

// UserReserved3 is github.com/siderolabs/talos/internal/pkg/meta.UserReserved3.
const UserReserved3 = 14

// CanSetMetaKey checks if the meta key can be set using Omni/Image Factory.
// To avoid messing up things which are internal to Talos.
func CanSetMetaKey(key int) bool {
	switch key {
	case LabelsMeta,
		MetalNetworkPlatformConfig,
		UserReserved2,
		UserReserved3:
		return true
	}

	return false
}
