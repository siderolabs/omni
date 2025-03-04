// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
	"bytes"
	"errors"
	"fmt"
	"iter"

	"github.com/siderolabs/gen/xslices"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/pkg/constants"
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

	if len(x.GetCompressedPatches()) == 0 {
		return xslices.Map(x.GetPatches(), func(patch string) Buffer {
			return newNoOpBuffer([]byte(patch))
		}), nil
	}

	buffers := make([]Buffer, 0, len(x.GetCompressedPatches()))
	config := getCompressionConfig(opts)

	for _, compressed := range x.GetCompressedPatches() {
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

	if !compress || !isAnyAboveThreshold(data, config.MinThreshold) {
		x.Patches = xslices.Map(data, func(patch []byte) string { return string(patch) })
		x.CompressedPatches = nil

		return nil
	}

	x.CompressedPatches = xslices.Map(data, func(patch []byte) []byte { return doCompress(patch, config) })
	x.Patches = nil

	return nil
}

func isAnyAboveThreshold(data [][]byte, threshold int) bool {
	for _, d := range data {
		if len(d) >= threshold {
			return true
		}
	}

	return false
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

// FromConfigPatches converts a list of ConfigPatchSpec to ClusterMachineConfigPatchesSpec.
func (x *ClusterMachineConfigPatchesSpec) FromConfigPatches(
	patches iter.Seq[*ConfigPatchSpec],
	compressionEnabled bool,
) error {
	aboveThreshold, err := anyAboveThreshold(patches)
	if err != nil {
		return fmt.Errorf("failed to check if one patch is above threshold: %w", err)
	}

	if compressionEnabled || aboveThreshold {
		return x.fromConfigPatchesCompress(patches)
	}

	return x.fromConfigPatchesNoCompress(patches)
}

func (x *ClusterMachineConfigPatchesSpec) fromConfigPatchesCompress(patches iter.Seq[*ConfigPatchSpec]) error {
	for patch := range patches {
		compr, err := getCompressed(patch)
		if err != nil {
			return err
		} else if len(compr) == 0 {
			continue
		}

		if len(compr) < 1024 { // this is a small patch, decompress to check if it's all whitespace
			if isEmptyPatch(patch) {
				continue
			}
		}

		// append the patch
		x.CompressedPatches = append(x.GetCompressedPatches(), compr)
	}

	return nil
}

func getCompressed(patch *ConfigPatchSpec) ([]byte, error) {
	if isEmptyPatch(patch) {
		return nil, nil
	}

	if compressedData := patch.GetCompressedData(); len(compressedData) > 0 {
		return compressedData, nil
	}

	buffer, err := patch.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	if err = patch.SetUncompressedData(buffer.Data(), WithCompressionMinThreshold(0)); err != nil {
		return nil, err
	}

	return patch.GetCompressedData(), nil
}

func (x *ClusterMachineConfigPatchesSpec) fromConfigPatchesNoCompress(patches iter.Seq[*ConfigPatchSpec]) error {
	for patch := range patches {
		data, err := getUncompressed(patch)
		if err != nil {
			return err
		} else if len(data) == 0 {
			continue
		}

		x.Patches = append(x.GetPatches(), data)
	}

	return nil
}

func getUncompressed(patch *ConfigPatchSpec) (string, error) {
	if isEmptyPatch(patch) {
		return "", nil
	}

	if data := patch.GetData(); len(data) > 0 {
		return data, nil
	}

	buffer, err := patch.GetUncompressedData()
	if err != nil {
		return "", err
	}

	defer buffer.Free()

	return string(buffer.Data()), nil
}

func isEmptyPatch(patch *ConfigPatchSpec) bool {
	buffer, err := patch.GetUncompressedData()
	if err != nil {
		return false
	}

	defer buffer.Free()

	return len(bytes.TrimSpace(buffer.Data())) == 0
}

func anyAboveThreshold(patches iter.Seq[*ConfigPatchSpec]) (bool, error) {
	for p := range patches {
		data, err := p.GetUncompressedData()
		if err != nil {
			return false, err
		}

		total := len(data.Data())

		data.Free()

		if total >= constants.CompressionThresholdBytes {
			return true, nil
		}
	}

	return false, nil
}
