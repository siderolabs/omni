// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/template/internal/models"
)

type config struct {
	Inline models.InlineContent `yaml:"inline"`
}

func TestInlineContent_UnmarshalSingleMap(t *testing.T) {
	t.Parallel()

	src := `
inline:
  machine:
    network:
      kubespan:
        enabled: true
`

	var got config

	require.NoError(t, yaml.Unmarshal([]byte(src), &got))

	raw, err := got.Inline.Bytes()
	require.NoError(t, err)

	assert.Contains(t, string(raw), "kubespan:")
	assert.Contains(t, string(raw), "enabled: true")
}

func TestInlineContent_UnmarshalSliceOfMaps(t *testing.T) {
	t.Parallel()

	src := `
inline:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: a
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: b
`

	var got config

	require.NoError(t, yaml.Unmarshal([]byte(src), &got))

	raw, err := got.Inline.Bytes()
	require.NoError(t, err)

	docs := splitYAMLDocs(t, raw)

	assert.Len(t, docs, 2)
	assert.Contains(t, docs[0], "name: a")
	assert.Contains(t, docs[1], "name: b")
}

func TestInlineContent_UnmarshalRawBytes(t *testing.T) {
	t.Parallel()

	src := `
inline: |
  machine:
    network:
      kubespan:
        enabled: true
`

	var got config

	require.NoError(t, yaml.Unmarshal([]byte(src), &got))

	raw, err := got.Inline.Bytes()
	require.NoError(t, err)

	expected := `machine:
  network:
    kubespan:
      enabled: true
`

	assert.Equal(t, expected, string(raw))
}

func TestInlineContent_UnmarshalRawBytesMultiDoc(t *testing.T) {
	t.Parallel()

	src := `inline: |
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: a
  ---
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: b
`

	var got config

	require.NoError(t, yaml.Unmarshal([]byte(src), &got))

	raw, err := got.Inline.Bytes()
	require.NoError(t, err)

	assert.Contains(t, string(raw), "name: a")
	assert.Contains(t, string(raw), "name: b")
	assert.Contains(t, string(raw), "---")
}

func TestInlineContent_RoundtripSingleMap(t *testing.T) {
	t.Parallel()

	src := `inline:
  machine:
    network:
      kubespan:
        enabled: true
`

	var got config

	require.NoError(t, yaml.Unmarshal([]byte(src), &got))

	out, err := yaml.Marshal(got)
	require.NoError(t, err)

	// re-parse and compare structurally
	var roundtrip config

	require.NoError(t, yaml.Unmarshal(out, &roundtrip))

	rawIn, err := got.Inline.Bytes()
	require.NoError(t, err)

	rawOut, err := roundtrip.Inline.Bytes()
	require.NoError(t, err)

	assert.Equal(t, string(rawIn), string(rawOut))
}

func TestInlineContent_RoundtripSliceOfMaps(t *testing.T) {
	t.Parallel()

	src := `inline:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: a
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: b
`

	var got config

	require.NoError(t, yaml.Unmarshal([]byte(src), &got))

	out, err := yaml.Marshal(&got)
	require.NoError(t, err)

	var roundtrip config

	require.NoError(t, yaml.Unmarshal(out, &roundtrip))

	rawIn, err := got.Inline.Bytes()
	require.NoError(t, err)

	rawOut, err := roundtrip.Inline.Bytes()
	require.NoError(t, err)

	assert.Equal(t, string(rawIn), string(rawOut))
}

func TestInlineContent_RoundtripRawBytes(t *testing.T) {
	t.Parallel()

	src := `inline: |
  machine:
    network:
      kubespan:
        enabled: true
`

	var got config

	require.NoError(t, yaml.Unmarshal([]byte(src), &got))

	out, err := yaml.Marshal(&got)
	require.NoError(t, err)

	// raw bytes are re-emitted as a literal block scalar
	assert.True(t, strings.HasPrefix(string(out), "inline: |"))
	assert.Contains(t, string(out), "kubespan:")
}

func TestInlineContent_OmitEmpty(t *testing.T) {
	t.Parallel()

	type wrapper struct {
		Name   string               `yaml:"name"`
		Inline models.InlineContent `yaml:"inline,omitempty"`
	}

	out, err := yaml.Marshal(wrapper{Name: "alpha"})
	require.NoError(t, err)

	assert.Equal(t, "name: alpha\n", string(out))
}

func TestInlineContent_Constructors(t *testing.T) {
	t.Parallel()

	mapInline := models.NewInlineContent(map[string]any{"a": 1})

	mapBytes, err := mapInline.Bytes()
	require.NoError(t, err)
	assert.Contains(t, string(mapBytes), "a: 1")

	listInline := models.NewInlineContent(map[string]any{"a": 1}, map[string]any{"b": 2})

	listBytes, err := listInline.Bytes()
	require.NoError(t, err)
	assert.Contains(t, string(listBytes), "a: 1")
	assert.Contains(t, string(listBytes), "b: 2")

	rawInline := models.NewInlineContentBytes([]byte("a: 1\n"))

	rawBytes, err := rawInline.Bytes()
	require.NoError(t, err)
	assert.Equal(t, "a: 1\n", string(rawBytes))
}

func splitYAMLDocs(t *testing.T, raw []byte) []string {
	t.Helper()

	dec := yaml.NewDecoder(strings.NewReader(string(raw)))

	var docs []string

	for {
		var node yaml.Node

		err := dec.Decode(&node)
		if err != nil {
			break
		}

		out, marshalErr := yaml.Marshal(&node)
		require.NoError(t, marshalErr)

		docs = append(docs, string(out))
	}

	return docs
}
