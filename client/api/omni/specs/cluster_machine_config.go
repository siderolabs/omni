// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// MarshalJSON implements json.Marshaler interface.
//
// It represents compressed fields as uncompressed in the output.
func (x *ClusterMachineConfigSpec) MarshalJSON() ([]byte, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.Data = buffer.Data()
	obj.CompressedData = nil

	return jsonMarshaler.Marshal(obj)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (x *ClusterMachineConfigSpec) UnmarshalJSON(data []byte) error {
	return unmarshalJSON(x, data)
}

// MarshalYAML implements yaml.Marshaler interface.
//
// It represents compressed fields as uncompressed in the output.
func (x *ClusterMachineConfigSpec) MarshalYAML() (any, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.Data = buffer.Data()
	obj.CompressedData = nil

	type alias *ClusterMachineConfigSpec // prevent recursion

	return alias(obj), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (x *ClusterMachineConfigSpec) UnmarshalYAML(node *yaml.Node) error {
	type alias ClusterMachineConfigSpec // prevent recursion

	aux := (*alias)(x)

	return unmarshalYAML(x, aux, node)
}

// GetUncompressedData returns the config data from the ClusterMachineConfigSpec, decompressing it if necessary.
func (x *ClusterMachineConfigSpec) GetUncompressedData(opts ...CompressionOption) (Buffer, error) {
	if x == nil {
		return newNoOpBuffer(nil), nil
	}

	if len(x.GetCompressedData()) == 0 {
		return newNoOpBuffer(x.GetData()), nil
	}

	return doDecompress(x.GetCompressedData(), getCompressionConfig(opts))
}

// SetUncompressedData sets the config data in the ClusterMachineConfigSpec, compressing it if requested.
func (x *ClusterMachineConfigSpec) SetUncompressedData(data []byte, opts ...CompressionOption) error {
	if x == nil {
		return errors.New("ClusterMachineConfigSpec is nil")
	}

	config := getCompressionConfig(opts)
	compress := config.Enabled

	if !compress || len(data) < config.MinThreshold {
		x.Data = data
		x.CompressedData = nil

		return nil
	}

	compressed := doCompress(data, config)

	x.CompressedData = compressed
	x.Data = nil

	return nil
}
