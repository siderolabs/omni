// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package specs

import (
	"errors"

	"go.yaml.in/yaml/v4"
)

// MarshalJSON marshals the ConfigPatchSpec to JSON, representing the compressed data as the regular data field.
func (x *ConfigPatchSpec) MarshalJSON() ([]byte, error) {
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

// UnmarshalJSON unmarshals the ConfigPatchSpec from JSON.
func (x *ConfigPatchSpec) UnmarshalJSON(data []byte) error {
	return unmarshalJSON(x, data)
}

// MarshalYAML implements yaml.Marshaler interface.
func (x *ConfigPatchSpec) MarshalYAML() (any, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.Data = string(buffer.Data())
	obj.CompressedData = nil

	type alias *ConfigPatchSpec // prevent recursion

	return alias(obj), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (x *ConfigPatchSpec) UnmarshalYAML(node *yaml.Node) error {
	type alias ConfigPatchSpec // prevent recursion

	aux := (*alias)(x)

	return unmarshalYAML(x, aux, node)
}

// GetUncompressedData returns the patch data from the ConfigPatchSpec, decompressing it if necessary.
func (x *ConfigPatchSpec) GetUncompressedData(opts ...CompressionOption) (Buffer, error) {
	if x == nil {
		return newNoOpBuffer(nil), nil
	}

	if len(x.GetCompressedData()) == 0 {
		return newNoOpBuffer([]byte(x.GetData())), nil
	}

	config := getCompressionConfig(opts)

	return doDecompress(x.GetCompressedData(), config)
}

// SetUncompressedData sets the patch data in the ConfigPatchSpec, compressing it if requested.
func (x *ConfigPatchSpec) SetUncompressedData(data []byte, opts ...CompressionOption) error {
	if x == nil {
		return errors.New("ConfigPatchSpec is nil")
	}

	config := getCompressionConfig(opts)
	compress := config.Enabled

	if !compress || len(data) < config.MinThreshold {
		x.Data = string(data)
		x.CompressedData = nil

		return nil
	}

	compressedData := doCompress(data, config)

	x.Data = ""
	x.CompressedData = compressedData

	return nil
}
