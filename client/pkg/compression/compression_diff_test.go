// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:staticcheck // we are ok with accessing the deprecated fields in these tests.
package compression_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/compression"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// diffLine is a single line of a payload shaped like a unified machine config diff.
const diffLine = "+  key: value\n"

// diffString builds a diff-like payload of at least the requested length.
func diffString(minLength int) string {
	return strings.Repeat(diffLine, minLength/len(diffLine)+1)
}

func TestMachinePendingUpdatesYAML(t *testing.T) {
	res := omni.NewMachinePendingUpdates("test")

	configDiff := diffString(specs.GetCompressionConfig().MinThreshold)

	res.TypedSpec().Value.Upgrade = &specs.MachinePendingUpdatesSpec_Upgrade{
		FromVersion: "v1.9.0",
		ToVersion:   "v1.10.0",
	}

	err := res.TypedSpec().Value.SetUncompressedData([]byte(configDiff))
	require.NoError(t, err)

	// assert that the diff is compressed

	require.Empty(t, res.TypedSpec().Value.ConfigDiff)
	require.NotEmpty(t, res.TypedSpec().Value.CompressedConfigDiff)
	require.True(t, res.TypedSpec().Value.HasConfigDiff())

	// marshal the spec to yaml

	specYAML, err := yaml.Marshal(res.TypedSpec().Value)
	require.NoError(t, err)

	// assert that the diff is in the YAML in uncompressed form, and the compressed field is emitted empty

	require.Contains(t, string(specYAML), strings.TrimSuffix(diffLine, "\n"))
	require.Contains(t, string(specYAML), "compressedconfigdiff: []")

	// assert that the sibling fields survive the round-trip through the marshaler

	require.Contains(t, string(specYAML), "v1.10.0")

	t.Logf("yaml:\n%s", string(specYAML))

	// unmarshal the spec from the yaml

	var newSpec specs.MachinePendingUpdatesSpec

	err = yaml.Unmarshal(specYAML, &newSpec)
	require.NoError(t, err)

	// assert that the diff got compressed again and still decompresses to the original

	require.Empty(t, newSpec.ConfigDiff)
	require.NotEmpty(t, newSpec.CompressedConfigDiff)
	require.Equal(t, "v1.10.0", newSpec.GetUpgrade().GetToVersion())

	buffer, err := newSpec.GetUncompressedData()
	require.NoError(t, err)

	defer buffer.Free()

	require.Equal(t, configDiff, string(buffer.Data()))
}

// assertDiffSpecJSON round-trips a diff-carrying spec through JSON, asserting that the payload stays
// compressed in memory while being emitted uncompressed on the wire.
func assertDiffSpecJSON[T any, S interface {
	*T
	specs.FieldCompressor[specs.Buffer, []byte]
}](t *testing.T, plain func(S) string, compressed func(S) []byte) {
	t.Helper()

	spec := S(new(T))

	diff := diffString(specs.GetCompressionConfig().MinThreshold)

	require.NoError(t, spec.SetUncompressedData([]byte(diff)))

	// assert that the diff is compressed

	require.Empty(t, plain(spec))
	require.NotEmpty(t, compressed(spec))

	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)

	// protojson omits the empty bytes field, so the compressed form is absent entirely

	require.Contains(t, string(specJSON), "key: value")
	require.NotContains(t, string(specJSON), "compressed")

	t.Logf("json:\n%s", string(specJSON))

	// unmarshalling re-compresses, and the payload still round-trips to the original

	newSpec := S(new(T))

	require.NoError(t, json.Unmarshal(specJSON, newSpec))
	require.Empty(t, plain(newSpec))
	require.NotEmpty(t, compressed(newSpec))

	buffer, err := newSpec.GetUncompressedData()
	require.NoError(t, err)

	defer buffer.Free()

	require.Equal(t, diff, string(buffer.Data()))
}

func TestDiffSpecsJSON(t *testing.T) {
	t.Run("MachinePendingUpdates", func(t *testing.T) {
		assertDiffSpecJSON(
			t,
			func(s *specs.MachinePendingUpdatesSpec) string { return s.ConfigDiff },
			func(s *specs.MachinePendingUpdatesSpec) []byte { return s.CompressedConfigDiff },
		)
	})

	t.Run("MachineConfigDiff", func(t *testing.T) {
		assertDiffSpecJSON(
			t,
			func(s *specs.MachineConfigDiffSpec) string { return s.Diff },
			func(s *specs.MachineConfigDiffSpec) []byte { return s.CompressedDiff },
		)
	})
}

func TestMachinePendingUpdatesThreshold(t *testing.T) {
	res := omni.NewMachinePendingUpdates("test")

	// a diff below the threshold is stored uncompressed

	err := res.TypedSpec().Value.SetUncompressedData([]byte(diffLine))
	require.NoError(t, err)

	require.Equal(t, diffLine, res.TypedSpec().Value.ConfigDiff)
	require.Empty(t, res.TypedSpec().Value.CompressedConfigDiff)
	require.True(t, res.TypedSpec().Value.HasConfigDiff())

	// growing above the threshold moves the data into the compressed field

	err = res.TypedSpec().Value.SetUncompressedData([]byte(diffString(specs.GetCompressionConfig().MinThreshold)))
	require.NoError(t, err)

	require.Empty(t, res.TypedSpec().Value.ConfigDiff)
	require.NotEmpty(t, res.TypedSpec().Value.CompressedConfigDiff)

	// shrinking back below the threshold clears the compressed field again

	err = res.TypedSpec().Value.SetUncompressedData([]byte(diffLine))
	require.NoError(t, err)

	require.Equal(t, diffLine, res.TypedSpec().Value.ConfigDiff)
	require.Empty(t, res.TypedSpec().Value.CompressedConfigDiff)

	// an empty diff clears both fields

	err = res.TypedSpec().Value.SetUncompressedData(nil)
	require.NoError(t, err)

	require.Empty(t, res.TypedSpec().Value.ConfigDiff)
	require.Empty(t, res.TypedSpec().Value.CompressedConfigDiff)
	require.False(t, res.TypedSpec().Value.HasConfigDiff())
}

func TestMachineConfigDiffYAML(t *testing.T) {
	res := omni.NewMachineConfigDiff("test")

	diff := diffString(specs.GetCompressionConfig().MinThreshold)

	err := res.TypedSpec().Value.SetUncompressedData([]byte(diff))
	require.NoError(t, err)

	// assert that the diff is compressed

	require.Empty(t, res.TypedSpec().Value.Diff)
	require.NotEmpty(t, res.TypedSpec().Value.CompressedDiff)

	// marshal the spec to yaml

	specYAML, err := yaml.Marshal(res.TypedSpec().Value)
	require.NoError(t, err)

	require.Contains(t, string(specYAML), strings.TrimSuffix(diffLine, "\n"))
	require.Contains(t, string(specYAML), "compresseddiff: []")

	t.Logf("yaml:\n%s", string(specYAML))

	// unmarshal the spec from the yaml

	var newSpec specs.MachineConfigDiffSpec

	err = yaml.Unmarshal(specYAML, &newSpec)
	require.NoError(t, err)

	require.Empty(t, newSpec.Diff)
	require.NotEmpty(t, newSpec.CompressedDiff)

	buffer, err := newSpec.GetUncompressedData()
	require.NoError(t, err)

	defer buffer.Free()

	require.Equal(t, diff, string(buffer.Data()))
}

func TestMachineConfigDiffThreshold(t *testing.T) {
	res := omni.NewMachineConfigDiff("test")

	err := res.TypedSpec().Value.SetUncompressedData([]byte(diffLine))
	require.NoError(t, err)

	require.Equal(t, diffLine, res.TypedSpec().Value.Diff)
	require.Empty(t, res.TypedSpec().Value.CompressedDiff)

	err = res.TypedSpec().Value.SetUncompressedData([]byte(diffString(specs.GetCompressionConfig().MinThreshold)))
	require.NoError(t, err)

	require.Empty(t, res.TypedSpec().Value.Diff)
	require.NotEmpty(t, res.TypedSpec().Value.CompressedDiff)

	// setting a small value on an already-compressed spec drops the compressed field

	err = res.TypedSpec().Value.SetUncompressedData([]byte(diffLine))
	require.NoError(t, err)

	require.Equal(t, diffLine, res.TypedSpec().Value.Diff)
	require.Empty(t, res.TypedSpec().Value.CompressedDiff)
}

func TestDiffSpecsCompressionDisabled(t *testing.T) {
	config, err := compression.BuildConfig(false, true, false)
	require.NoError(t, err)

	opt := specs.WithConfigCompressionOption(config)

	diff := diffString(specs.GetCompressionConfig().MinThreshold)

	pendingUpdates := omni.NewMachinePendingUpdates("test")

	require.NoError(t, pendingUpdates.TypedSpec().Value.SetUncompressedData([]byte(diff), opt))
	require.Equal(t, diff, pendingUpdates.TypedSpec().Value.ConfigDiff)
	require.Empty(t, pendingUpdates.TypedSpec().Value.CompressedConfigDiff)

	configDiff := omni.NewMachineConfigDiff("test")

	require.NoError(t, configDiff.TypedSpec().Value.SetUncompressedData([]byte(diff), opt))
	require.Equal(t, diff, configDiff.TypedSpec().Value.Diff)
	require.Empty(t, configDiff.TypedSpec().Value.CompressedDiff)
}

func TestDiffSpecsUncompressedReadCompatibility(t *testing.T) {
	// resources written before compression was introduced keep the plain field set, reads must be transparent
	diff := diffString(specs.GetCompressionConfig().MinThreshold)

	pendingUpdates := &specs.MachinePendingUpdatesSpec{ConfigDiff: diff}

	require.True(t, pendingUpdates.HasConfigDiff())

	buffer, err := pendingUpdates.GetUncompressedData()
	require.NoError(t, err)

	defer buffer.Free()

	require.Equal(t, diff, string(buffer.Data()))

	configDiff := &specs.MachineConfigDiffSpec{Diff: diff}

	diffBuffer, err := configDiff.GetUncompressedData()
	require.NoError(t, err)

	defer diffBuffer.Free()

	require.Equal(t, diff, string(diffBuffer.Data()))
}
