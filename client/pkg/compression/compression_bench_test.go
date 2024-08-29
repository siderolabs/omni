// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compression_test

import (
	_ "embed"
	"testing"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/compression"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

var (
	//go:embed testdata/config-small.yaml
	testConfigSmall []byte

	//go:embed testdata/config-large.yaml
	testConfigLarge []byte
)

func BenchmarkAccess(b *testing.B) {
	b.Run("small_no-compression_no-pool_no-dict", func(b *testing.B) {
		benchmark(b, false, false, false, false)
	})

	b.Run("large_no-compression_no-pool_no-dict", func(b *testing.B) {
		benchmark(b, true, false, false, false)
	})

	b.Run("small_with-compression_no-pool_no-dict", func(b *testing.B) {
		benchmark(b, false, true, false, false)
	})

	b.Run("large_with-compression_no-pool_no-dict", func(b *testing.B) {
		benchmark(b, true, true, false, false)
	})

	b.Run("small_with-compression_with-pool_no-dict", func(b *testing.B) {
		benchmark(b, false, true, true, false)
	})

	b.Run("large_with-compression_with-pool_no-dict", func(b *testing.B) {
		benchmark(b, true, true, true, false)
	})

	// The dictionary is expected to make a big difference, as it works better on smaller inputs.
	b.Run("small_with-compression_with-pool_with-dict", func(b *testing.B) {
		benchmark(b, false, true, true, true)
	})

	// The dictionary is expected to make a small difference, as it works better on smaller inputs.
	b.Run("large_with-compression_with-pool_with-dict", func(b *testing.B) {
		benchmark(b, true, true, true, true)
	})
}

func TestDictionaryCompressionSize(t *testing.T) {
	noDictCompressionConfig, err := compression.BuildConfig(true, false, false)
	require.NoError(t, err)

	withDictCompressionConfig, err := compression.BuildConfig(true, true, false)
	require.NoError(t, err)

	smallInputNoDict := buildConfig(t, false, noDictCompressionConfig)
	smallInputWithDict := buildConfig(t, false, withDictCompressionConfig)
	largeInputNoDict := buildConfig(t, true, noDictCompressionConfig)
	largeInputWithDict := buildConfig(t, true, withDictCompressionConfig)

	t.Logf("small input no dict: %s", humanize.IBytes(uint64(len(smallInputNoDict.TypedSpec().Value.GetCompressedData()))))
	t.Logf("small input with dict: %s", humanize.IBytes(uint64(len(smallInputWithDict.TypedSpec().Value.GetCompressedData()))))
	t.Logf("large input no dict: %s", humanize.IBytes(uint64(len(largeInputNoDict.TypedSpec().Value.GetCompressedData()))))
	t.Logf("large input with dict: %s", humanize.IBytes(uint64(len(largeInputWithDict.TypedSpec().Value.GetCompressedData()))))
}

func buildConfig(tb testing.TB, large bool, compressionConfig specs.CompressionConfig) *omni.ClusterMachineConfig {
	config := omni.NewClusterMachineConfig(resources.DefaultNamespace, "test")

	configData := testConfigSmall
	if large {
		configData = testConfigLarge
	}

	require.NoError(tb, config.TypedSpec().Value.SetUncompressedData(configData, specs.WithConfigCompressionOption(compressionConfig)))

	return config
}

func benchmark(b *testing.B, large, compress, pool, dict bool) {
	compressionConfig, configErr := compression.BuildConfig(compress, dict, pool)
	require.NoError(b, configErr)

	config := buildConfig(b, large, compressionConfig)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buffer, err := config.TypedSpec().Value.GetUncompressedData(specs.WithConfigCompressionOption(compressionConfig))
		require.NoError(b, err)

		buffer.Free()
	}
}
