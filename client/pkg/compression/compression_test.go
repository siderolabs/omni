// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:staticcheck // we are ok with accesssing the deprecated fields in these tests.
package compression_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func TestClusterMachineConfigPatchesYAML(t *testing.T) {
	res := omni.NewClusterMachineConfigPatches("test", "test")

	// set some patches

	aString := strings.Repeat("a", constants.CompressionThresholdBytes)
	bString := strings.Repeat("b", constants.CompressionThresholdBytes)

	err := res.TypedSpec().Value.SetUncompressedPatches([]string{aString, bString})
	require.NoError(t, err)

	// assert that the patches are compressed

	require.Empty(t, res.TypedSpec().Value.Patches)
	require.NotEmpty(t, res.TypedSpec().Value.CompressedPatches)

	// marshal the spec to yaml

	spec := res.TypedSpec().Value

	specYAML, err := yaml.Marshal(spec)
	require.NoError(t, err)

	// assert that the patches are in the YAML in uncompressed form, and do not contain the compressed form

	require.Contains(t, string(specYAML), aString)
	require.Contains(t, string(specYAML), bString)
	require.NotContains(t, string(specYAML), "compressed")

	t.Logf("yaml:\n%s", string(specYAML))

	// unmarshal the spec from the yaml

	var newSpec specs.ClusterMachineConfigPatchesSpec

	err = yaml.Unmarshal(specYAML, &newSpec)
	require.NoError(t, err)

	// assert that the patches got compressed again

	require.Empty(t, newSpec.Patches)
	require.NotEmpty(t, newSpec.CompressedPatches)
}

func TestClusterMachineConfigPatchesJSON(t *testing.T) {
	res := omni.NewClusterMachineConfigPatches("test", "test")

	aString := strings.Repeat("a", constants.CompressionThresholdBytes)
	bString := strings.Repeat("b", constants.CompressionThresholdBytes)

	// set some patches

	err := res.TypedSpec().Value.SetUncompressedPatches([]string{aString, bString})
	require.NoError(t, err)

	// assert that the patches are compressed

	require.Empty(t, res.TypedSpec().Value.Patches)
	require.NotEmpty(t, res.TypedSpec().Value.CompressedPatches)

	// marshal the spec to json

	spec := res.TypedSpec().Value

	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)

	// assert that the patches are in the JSON in uncompressed form, and do not contain the compressed form

	require.Contains(t, string(specJSON), aString)
	require.Contains(t, string(specJSON), bString)
	require.NotContains(t, string(specJSON), "compressed")

	t.Logf("json:\n%s", string(specJSON))

	// unmarshal the spec from the json

	var newSpec *specs.ClusterMachineConfigPatchesSpec

	err = json.Unmarshal(specJSON, &newSpec)
	require.NoError(t, err)

	// assert that the patches got compressed again

	require.Empty(t, newSpec.Patches)
	require.NotEmpty(t, newSpec.CompressedPatches)
}
