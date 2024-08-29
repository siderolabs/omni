// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
	"errors"

	"github.com/siderolabs/gen/xslices"
	"gopkg.in/yaml.v3"
)

// MarshalJSON implements json.Marshaler interface.
//
// It represents compressed fields as uncompressed in the output.
func (x *ClusterMachineConfigPatchesSpec) MarshalJSON() ([]byte, error) {
	obj := x.CloneVT()

	patches, err := obj.GetUncompressedPatches()
	if err != nil {
		return nil, err
	}

	obj.Patches = patches
	obj.CompressedPatches = nil

	return jsonMarshaler.Marshal(obj)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (x *ClusterMachineConfigPatchesSpec) UnmarshalJSON(data []byte) error {
	return unmarshalJSONMultiple(x, data, GetCompressionConfig())
}

// MarshalYAML implements yaml.Marshaler interface.
func (x *ClusterMachineConfigPatchesSpec) MarshalYAML() (any, error) {
	patches, err := x.GetUncompressedPatches()
	if err != nil {
		return nil, err
	}

	contents := xslices.Map(patches, func(patch string) *yaml.Node {
		style := yaml.FlowStyle
		if len(patch) > 0 && (patch[0] == '\n' || patch[0] == ' ') {
			style = yaml.SingleQuotedStyle
		}

		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Style: style,
			Value: patch,
		}
	})

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "patches"},
			{Kind: yaml.SequenceNode, Tag: "!!seq", Content: contents},
		},
	}, nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (x *ClusterMachineConfigPatchesSpec) UnmarshalYAML(node *yaml.Node) error {
	type alias ClusterMachineConfigPatchesSpec // prevent recursion

	aux := (*alias)(x)

	return unmarshalYAMLMultiple(x, aux, node, GetCompressionConfig())
}

// GetUncompressedData returns the patches from the ClusterMachineConfigPatchesSpec, decompressing them if necessary.
func (x *ClusterMachineConfigPatchesSpec) GetUncompressedData(opts ...CompressionOption) ([]Buffer, error) {
	if x == nil {
		return nil, nil
	}

	if x.CompressedPatches == nil {
		return xslices.Map(x.Patches, func(patch string) Buffer {
			return newNoOpBuffer([]byte(patch))
		}), nil
	}

	buffers := make([]Buffer, 0, len(x.CompressedPatches))
	config := getCompressionConfig(opts)

	for _, compressed := range x.CompressedPatches {
		buffer, err := doDecompress(compressed, config)
		if err != nil {
			return nil, err
		}

		buffers = append(buffers, buffer)
	}

	return buffers, nil
}

// SetUncompressedData sets the patches in the ClusterMachineConfigPatchesSpec, compressing them if requested.
func (x *ClusterMachineConfigPatchesSpec) SetUncompressedData(data [][]byte, opts ...CompressionOption) error {
	if x == nil {
		return errors.New("ClusterMachineConfigPatchesSpec is nil")
	}

	config := getCompressionConfig(opts)
	compress := config.Enabled

	if !compress {
		x.Patches = xslices.Map(data, func(patch []byte) string { return string(patch) })
		x.CompressedPatches = nil

		return nil
	}

	x.CompressedPatches = xslices.Map(data, func(patch []byte) []byte { return doCompress(patch, config) })
	x.Patches = nil

	return nil
}

// GetUncompressedPatches returns the patches from the ClusterMachineConfigPatchesSpec, decompressing them if necessary.
func (x *ClusterMachineConfigPatchesSpec) GetUncompressedPatches(opts ...CompressionOption) ([]string, error) {
	buffers, err := x.GetUncompressedData(opts...)
	if err != nil {
		return nil, err
	}

	return xslices.Map(buffers, func(buffer Buffer) string {
		return string(buffer.Data())
	}), nil
}

// SetUncompressedPatches sets the patches in the ClusterMachineConfigPatchesSpec, compressing them if requested.
func (x *ClusterMachineConfigPatchesSpec) SetUncompressedPatches(patches []string, opts ...CompressionOption) error {
	data := make([][]byte, 0, len(patches))

	for _, patch := range patches {
		data = append(data, []byte(patch))
	}

	return x.SetUncompressedData(data, opts...)
}
