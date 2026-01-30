// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package specs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	fmt "fmt"
	io "io"

	"go.yaml.in/yaml/v4"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

// MarshalJSON marshals the KubernetesManifestSpec to JSON, representing the compressed data as the regular data field.
func (x *KubernetesManifestSpec) MarshalJSON() ([]byte, error) {
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

// UnmarshalJSON unmarshals the KubernetesManifestSpec from JSON.
func (x *KubernetesManifestSpec) UnmarshalJSON(data []byte) error {
	return unmarshalJSON(x, data)
}

// MarshalYAML implements yaml.Marshaler interface.
func (x *KubernetesManifestSpec) MarshalYAML() (any, error) {
	obj := x.CloneVT()

	buffer, err := obj.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	obj.Data = string(buffer.Data())
	obj.CompressedData = nil

	type alias *KubernetesManifestSpec // prevent recursion

	return alias(obj), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (x *KubernetesManifestSpec) UnmarshalYAML(node *yaml.Node) error {
	type alias KubernetesManifestSpec // prevent recursion

	aux := (*alias)(x)

	return unmarshalYAML(x, aux, node)
}

// GetUncompressedData returns the patch data from the KubernetesManifestSpec, decompressing it if necessary.
func (x *KubernetesManifestSpec) GetUncompressedData(opts ...CompressionOption) (Buffer, error) {
	if x == nil {
		return newNoOpBuffer(nil), nil
	}

	if len(x.GetCompressedData()) == 0 {
		return newNoOpBuffer([]byte(x.GetData())), nil
	}

	config := getCompressionConfig(opts)

	return doDecompress(x.GetCompressedData(), config)
}

// SetUncompressedData sets the patch data in the KubernetesManifestSpec, compressing it if requested.
func (x *KubernetesManifestSpec) SetUncompressedData(data []byte, opts ...CompressionOption) error {
	if x == nil {
		return errors.New("KubernetesManifestSpec is nil")
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

// GetManifests reads all manifests stored in the resource.
func (x *KubernetesManifestSpec) GetManifests() ([]*unstructured.Unstructured, error) {
	buffer, err := x.GetUncompressedData()
	if err != nil {
		return nil, err
	}

	defer buffer.Free()

	decoder := k8syaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(buffer.Data())))

	var manifests []*unstructured.Unstructured

	for {
		var yamlManifest []byte

		yamlManifest, err = decoder.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		yamlManifest = bytes.TrimSpace(yamlManifest)

		if len(yamlManifest) == 0 {
			continue
		}

		jsonManifest, err := k8syaml.ToJSON(yamlManifest)
		if err != nil {
			return nil, fmt.Errorf("error converting manifest to JSON: %w", err)
		}

		if bytes.Equal(jsonManifest, []byte("null")) || bytes.Equal(jsonManifest, []byte("{}")) {
			// skip YAML docs which contain only comments
			continue
		}

		var obj *unstructured.Unstructured

		if err = json.Unmarshal(jsonManifest, &obj); err != nil {
			return nil, fmt.Errorf("error loading JSON manifest into unstructured: %w", err)
		}

		manifests = append(manifests, obj)
	}

	return manifests, nil
}
