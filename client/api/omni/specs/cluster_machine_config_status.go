// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
	"errors"

	"go.yaml.in/yaml/v4"
)

// MarshalJSON implements json.Marshaler interface.
//
// It represents compressed fields as uncompressed in the output.
func (x *ClusterMachineConfigStatusSpec) MarshalJSON() ([]byte, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.RedactedCurrentMachineConfig = string(buffer.Data())
	obj.CompressedRedactedMachineConfig = nil

	return jsonMarshaler.Marshal(obj)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (x *ClusterMachineConfigStatusSpec) UnmarshalJSON(data []byte) error {
	return unmarshalJSON(x, data)
}

// MarshalYAML implements yaml.Marshaler interface.
//
// It represents compressed fields as uncompressed in the output.
func (x *ClusterMachineConfigStatusSpec) MarshalYAML() (any, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.RedactedCurrentMachineConfig = string(buffer.Data())
	obj.CompressedRedactedMachineConfig = nil

	type alias *ClusterMachineConfigStatusSpec // prevent recursion

	return alias(obj), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (x *ClusterMachineConfigStatusSpec) UnmarshalYAML(node *yaml.Node) error {
	type alias ClusterMachineConfigStatusSpec // prevent recursion

	aux := (*alias)(x)

	return unmarshalYAML(x, aux, node)
}

// GetUncompressedData returns the config data from the ClusterMachineConfigStatusSpec, decompressing it if necessary.
func (x *ClusterMachineConfigStatusSpec) GetUncompressedData(opts ...CompressionOption) (Buffer, error) {
	if x == nil {
		return newNoOpBuffer(nil), nil
	}

	if len(x.GetCompressedRedactedMachineConfig()) == 0 {
		return newNoOpBuffer([]byte(x.GetRedactedCurrentMachineConfig())), nil
	}

	return doDecompress(x.GetCompressedRedactedMachineConfig(), getCompressionConfig(opts))
}

// SetUncompressedData sets the config data in the ClusterMachineConfigStatusSpec, compressing it if requested.
func (x *ClusterMachineConfigStatusSpec) SetUncompressedData(data []byte, opts ...CompressionOption) error {
	if x == nil {
		return errors.New("ClusterMachineConfigStatusSpec is nil")
	}

	config := getCompressionConfig(opts)
	compress := config.Enabled

	if !compress || len(data) < config.MinThreshold {
		x.RedactedCurrentMachineConfig = string(data)
		x.CompressedRedactedMachineConfig = nil

		return nil
	}

	compressed := doCompress(data, config)

	x.CompressedRedactedMachineConfig = compressed
	x.RedactedCurrentMachineConfig = ""

	return nil
}
