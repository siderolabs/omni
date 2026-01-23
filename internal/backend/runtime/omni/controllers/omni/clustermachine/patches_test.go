// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package clustermachine_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

func TestFromConfigPatches(t *testing.T) {
	// DO NOT ADD t.Parallel() HERE, as this test modifies global compression config.
	originalConfig := specs.GetCompressionConfig()
	defer specs.SetCompressionConfig(originalConfig)

	largePatchData := strings.Repeat("A", originalConfig.MinThreshold+100)
	smallPatchData := "small-patch-data"

	t.Run("compression disabled with large patches does not compress", func(t *testing.T) {
		disabledConfig := originalConfig
		disabledConfig.Enabled = false
		specs.SetCompressionConfig(disabledConfig)

		largePatch := &specs.ConfigPatchSpec{Data: largePatchData}
		smallPatch := &specs.ConfigPatchSpec{Data: smallPatchData}

		target := &specs.ClusterMachineConfigPatchesSpec{}
		patches := slices.Values([]*specs.ConfigPatchSpec{largePatch, smallPatch})

		err := target.FromConfigPatches(patches, disabledConfig)
		require.NoError(t, err)

		// When compression is disabled, patches are stored uncompressed even if large.
		assert.Empty(t, target.GetCompressedPatches(), "Expected no compressed patches when compression is disabled")
		assert.Len(t, target.GetPatches(), 2, "Expected 2 uncompressed patches")
		assert.Equal(t, largePatchData, target.GetPatches()[0])
		assert.Equal(t, smallPatchData, target.GetPatches()[1])
	})

	t.Run("compression enabled with large patches compresses all", func(t *testing.T) {
		enabledConfig := originalConfig
		enabledConfig.Enabled = true
		specs.SetCompressionConfig(enabledConfig)

		largePatch := &specs.ConfigPatchSpec{Data: largePatchData}
		smallPatch := &specs.ConfigPatchSpec{Data: smallPatchData}

		target := &specs.ClusterMachineConfigPatchesSpec{}
		patches := slices.Values([]*specs.ConfigPatchSpec{largePatch, smallPatch})

		err := target.FromConfigPatches(patches, enabledConfig)
		require.NoError(t, err)

		// When compression is enabled and at least one patch is above threshold, all patches are compressed.
		assert.Len(t, target.GetCompressedPatches(), 2, "Expected 2 compressed patches")
		assert.Empty(t, target.GetPatches(), "Expected no uncompressed patches")

		patchesData, err := target.GetUncompressedPatches()
		require.NoError(t, err)

		assert.Len(t, patchesData, 2)
		assert.Equal(t, largePatchData, patchesData[0])
		assert.Equal(t, smallPatchData, patchesData[1])
	})

	t.Run("compression enabled with only small patches does not compress", func(t *testing.T) {
		enabledConfig := originalConfig
		enabledConfig.Enabled = true
		specs.SetCompressionConfig(enabledConfig)

		smallPatch1 := &specs.ConfigPatchSpec{Data: "small-patch-1"}
		smallPatch2 := &specs.ConfigPatchSpec{Data: "small-patch-2"}

		target := &specs.ClusterMachineConfigPatchesSpec{}
		patches := slices.Values([]*specs.ConfigPatchSpec{smallPatch1, smallPatch2})

		err := target.FromConfigPatches(patches, enabledConfig)
		require.NoError(t, err)

		// When compression is enabled but no patch is above threshold, patches are stored uncompressed.
		assert.Empty(t, target.GetCompressedPatches(), "Expected no compressed patches when all patches are small")
		assert.Len(t, target.GetPatches(), 2, "Expected 2 uncompressed patches")
		assert.Equal(t, "small-patch-1", target.GetPatches()[0])
		assert.Equal(t, "small-patch-2", target.GetPatches()[1])
	})
}
