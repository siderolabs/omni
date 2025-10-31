// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package specs

import (
	"errors"

	"go.yaml.in/yaml/v4"
)

// MarshalJSON implements json.Marshaler interface.
func (x *RedactedClusterMachineConfigSpec) MarshalJSON() ([]byte, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.Data = string(buffer.Data())
	obj.CompressedData = nil

	return jsonMarshaler.Marshal(obj)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (x *RedactedClusterMachineConfigSpec) UnmarshalJSON(data []byte) error {
	return unmarshalJSON(x, data)
}

// MarshalYAML implements yaml.Marshaler interface.
func (x *RedactedClusterMachineConfigSpec) MarshalYAML() (any, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.Data = string(buffer.Data())
	obj.CompressedData = nil

	type alias *RedactedClusterMachineConfigSpec // prevent recursion

	return alias(obj), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (x *RedactedClusterMachineConfigSpec) UnmarshalYAML(node *yaml.Node) error {
	type alias RedactedClusterMachineConfigSpec // prevent recursion

	aux := (*alias)(x)

	return unmarshalYAML(x, aux, node)
}

// GetUncompressedData returns the config data from the RedactedClusterMachineConfigSpec, decompressing it if necessary.
func (x *RedactedClusterMachineConfigSpec) GetUncompressedData(opts ...CompressionOption) (Buffer, error) {
	if x == nil {
		return newNoOpBuffer(nil), nil
	}

	if x.GetCompressedData() == nil {
		return newNoOpBuffer([]byte(x.GetData())), nil
	}

	config := getCompressionConfig(opts)

	return doDecompress(x.GetCompressedData(), config)
}

// SetUncompressedData sets the config data in the RedactedClusterMachineConfigSpec, compressing it if requested.
func (x *RedactedClusterMachineConfigSpec) SetUncompressedData(data []byte, opts ...CompressionOption) error {
	if x == nil {
		return errors.New("RedactedClusterMachineConfigSpec is nil")
	}

	config := getCompressionConfig(opts)
	compress := config.Enabled

	if !compress || len(data) < config.MinThreshold {
		x.Data = string(data)
		x.CompressedData = nil

		return nil
	}

	compressed := doCompress(data, config)

	x.Data = ""
	x.CompressedData = compressed

	return nil
}
