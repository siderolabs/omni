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
func (x *MachinePendingUpdatesSpec) MarshalJSON() ([]byte, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.ConfigDiff = string(buffer.Data())
	obj.CompressedConfigDiff = nil

	return jsonMarshaler.Marshal(obj)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (x *MachinePendingUpdatesSpec) UnmarshalJSON(data []byte) error {
	return unmarshalJSON(x, data)
}

// MarshalYAML implements yaml.Marshaler interface.
//
// It represents compressed fields as uncompressed in the output.
func (x *MachinePendingUpdatesSpec) MarshalYAML() (any, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.ConfigDiff = string(buffer.Data())
	obj.CompressedConfigDiff = nil

	type alias *MachinePendingUpdatesSpec // prevent recursion

	return alias(obj), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (x *MachinePendingUpdatesSpec) UnmarshalYAML(node *yaml.Node) error {
	type alias MachinePendingUpdatesSpec // prevent recursion

	aux := (*alias)(x)

	return unmarshalYAML(x, aux, node)
}

// GetUncompressedData returns the config diff from the MachinePendingUpdatesSpec, decompressing it if necessary.
func (x *MachinePendingUpdatesSpec) GetUncompressedData(opts ...CompressionOption) (Buffer, error) {
	if x == nil {
		return newNoOpBuffer(nil), nil
	}

	if len(x.GetCompressedConfigDiff()) == 0 {
		return newNoOpBuffer([]byte(x.GetConfigDiff())), nil
	}

	return doDecompress(x.GetCompressedConfigDiff(), getCompressionConfig(opts))
}

// SetUncompressedData sets the config diff in the MachinePendingUpdatesSpec, compressing it if requested.
func (x *MachinePendingUpdatesSpec) SetUncompressedData(data []byte, opts ...CompressionOption) error {
	if x == nil {
		return errors.New("MachinePendingUpdatesSpec is nil")
	}

	config := getCompressionConfig(opts)
	compress := config.Enabled

	if !compress || len(data) < config.MinThreshold {
		x.ConfigDiff = string(data)
		x.CompressedConfigDiff = nil

		return nil
	}

	compressed := doCompress(data, config)

	x.ConfigDiff = ""
	x.CompressedConfigDiff = compressed

	return nil
}

// HasConfigDiff reports whether the config diff is set, without decompressing it.
func (x *MachinePendingUpdatesSpec) HasConfigDiff() bool {
	return x.GetConfigDiff() != "" || len(x.GetCompressedConfigDiff()) > 0
}
