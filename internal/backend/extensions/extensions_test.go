// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package extensions_test

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/extensions"
)

func TestMap(t *testing.T) {
	t.Run("talos-v1.6", func(t *testing.T) {
		exts := []string{
			"siderolabs/hello-world-service",
			"siderolabs/nvidia-container-toolkit",
			"siderolabs/xe-guest-utilities",
			"siderolabs/nonfree-kmod-nvidia-lts",
			"siderolabs/zfs",
			"siderolabs/v4l-uvc",
		}

		mapped := extensions.MapNamesByVersion(exts, semver.MustParse("1.6.8"))

		require.Equal(t, []string{
			"siderolabs/hello-world-service",      // kept as-is because not in the renamed list
			"siderolabs/nvidia-container-toolkit", // kept as-is because not renamed on this version
			"siderolabs/xe-guest-utilities",       // kept as-is because not renamed on this version
			"siderolabs/nonfree-kmod-nvidia",      // mapped to the old name
			"siderolabs/zfs",                      // kept as-is because not in the renamed list
			"siderolabs/v4l-uvc-drivers",          // mapped to the correct name
		}, mapped)
	})

	t.Run("talos-v1.7", func(t *testing.T) {
		exts := []string{
			"siderolabs/nvidia-container-toolkit",
			"siderolabs/xe-guest-utilities",
			"siderolabs/nvidia-open-gpu-kernel-modules-lts",
		}

		mapped := extensions.MapNamesByVersion(exts, semver.MustParse("1.7.0"))

		require.Equal(t, []string{
			"siderolabs/nvidia-container-toolkit",       // kept as-is because not renamed on this version
			"siderolabs/xen-guest-agent",                // mapped to the new name
			"siderolabs/nvidia-open-gpu-kernel-modules", // mapped to the old name
		}, mapped)
	})

	t.Run("talos-v1.8", func(t *testing.T) {
		exts := []string{
			"siderolabs/nvidia-container-toolkit",
			"siderolabs/nvidia-open-gpu-kernel-modules",
			"siderolabs/nonfree-kmod-nvidia",
			"siderolabs/nvidia-fabricmanager",
			"siderolabs/xe-guest-utilities",
		}

		mapped := extensions.MapNamesByVersion(exts, semver.MustParse("1.8.0"))

		require.Equal(t, []string{
			"siderolabs/nvidia-container-toolkit-lts",       // mapped to the new name
			"siderolabs/nvidia-open-gpu-kernel-modules-lts", // mapped to the new name
			"siderolabs/nonfree-kmod-nvidia-lts",            // mapped to the new name
			"siderolabs/nvidia-fabric-manager-lts",          // mapped to the new name
			"siderolabs/xen-guest-agent",                    // kept as-is because not in the renamed list
		}, mapped)
	})

	t.Run("talos-v1.9", func(t *testing.T) {
		exts := []string{
			"siderolabs/i915-ucode",
			"siderolabs/amdgpu-firmware",
		}

		mapped := extensions.MapNamesByVersion(exts, semver.MustParse("1.9.0"))

		require.Equal(t, []string{
			"siderolabs/i915",   // mapped to the new name
			"siderolabs/amdgpu", // mapped to the new name
		}, mapped)
	})
}
