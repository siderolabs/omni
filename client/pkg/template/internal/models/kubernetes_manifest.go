// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

type KubernetesManifestMode specs.KubernetesManifestGroupSpec_Mode

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *KubernetesManifestMode) UnmarshalYAML(unmarshal func(any) error) error {
	var value string

	if err := unmarshal(&value); err != nil {
		return err
	}

	switch value {
	case "one-time":
		*c = KubernetesManifestMode(specs.KubernetesManifestGroupSpec_ONE_TIME)
	case "full":
		*c = KubernetesManifestMode(specs.KubernetesManifestGroupSpec_FULL)
	default:
		return fmt.Errorf("unknown mode name %q", value)
	}

	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (c KubernetesManifestMode) MarshalYAML() (any, error) {
	switch c {
	case KubernetesManifestMode(specs.KubernetesManifestGroupSpec_FULL):
		return "full", nil
	case KubernetesManifestMode(specs.KubernetesManifestGroupSpec_ONE_TIME):
		return "one-time", nil
	default:
		return "", fmt.Errorf("unknown mode %q", c)
	}
}

// KubernetesManifest represents the kubernetes manifest to be applied in the cluster.
// The manifests are applied on the server side.
type KubernetesManifest struct {
	Descriptors Descriptors            `yaml:",inline"`
	Name        string                 `yaml:"name"`
	File        string                 `yaml:"file,omitempty"`
	Inline      []map[string]any       `yaml:"inline,omitempty"`
	Mode        KubernetesManifestMode `yaml:"mode"`
}

// Validate a kubernetes manifest.
func (km *KubernetesManifest) Validate() error {
	var errs error

	name := km.Name
	if name == "" {
		name = km.File
	}

	if name == "" {
		errs = errors.Join(errs, errors.New("kubernetes manifest name is empty"))
	}

	if km.Mode == KubernetesManifestMode(specs.KubernetesManifestGroupSpec_UNKNOWN) {
		errs = errors.Join(errs, fmt.Errorf("kubernetes manifest %q mode is not set", name))
	}

	if km.File != "" && km.Inline != nil {
		errs = errors.Join(errs, fmt.Errorf("path and inline are mutually exclusive"))
	}

	if km.File == "" && km.Inline == nil {
		errs = errors.Join(errs, fmt.Errorf("path or inline is required"))
	}

	if err := km.Descriptors.Validate(); err != nil {
		errs = errors.Join(errs, fmt.Errorf("kubernetes manifest %q descriptors validation failed: %w", name, err))
	}

	switch {
	case km.File != "":
		_, err := os.Stat(km.File)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to access %q: %w", km.File, err))
		}
	case km.Inline != nil:
		_, err := yaml.Marshal(km.Inline)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to marshal inline manifest %q: %w", name, err))
		}
	}

	return errs
}

// Translate the model into a resource.
func (km *KubernetesManifest) Translate(prefix string, weight int, labels ...pair.Pair[string, string]) (*omni.KubernetesManifestGroup, error) {
	name := km.Name
	if name == "" {
		name = km.File
	}

	id := fmt.Sprintf("%03d-%s-%s", weight, prefix, name)

	var (
		raw []byte
		err error
	)

	switch {
	case km.File != "":
		raw, err = os.ReadFile(km.File)
	case km.Inline != nil:
		var buf bytes.Buffer

		encoder := yaml.NewEncoder(&buf)
		for _, document := range km.Inline {
			if err = encoder.Encode(document); err != nil {
				break
			}
		}

		defer encoder.Close() //nolint:errcheck

		raw = buf.Bytes()
	default:
		return nil, fmt.Errorf("missing manifests contents")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read manifest %q: %w", name, err)
	}

	resource := omni.NewKubernetesManifestGroup(id)

	resource.Metadata().Labels().Do(func(temp kvutils.TempKV) {
		for _, label := range labels {
			temp.Set(label.F1, label.F2)
		}
	})

	resource.Metadata().Annotations().Set(omni.KubernetesManifestName, name)

	resource.TypedSpec().Value.Mode = specs.KubernetesManifestGroupSpec_Mode(km.Mode)

	km.Descriptors.Apply(resource)

	if err = resource.TypedSpec().Value.SetUncompressedData(raw); err != nil {
		return nil, err
	}

	return resource, nil
}

// KubernetesManifestsList is the list of KubernetesManifest structs.
type KubernetesManifestsList []KubernetesManifest

// Validate the manifests in the list.
func (k KubernetesManifestsList) Validate() error {
	names := make(map[string]struct{}, len(k))

	return errors.Join(xslices.Map(k, func(m KubernetesManifest) error {
		if err := m.Validate(); err != nil {
			return err
		}

		name := m.Name
		if name == "" {
			name = m.File
		}

		if _, exists := names[name]; exists {
			return fmt.Errorf("duplicate manifest name %q", name)
		}

		names[name] = struct{}{}

		return nil
	})...)
}

// Translate the list of KubernetesManifests into a list of resources.
func (l KubernetesManifestsList) Translate(prefix string, baseWeight int, labels ...pair.Pair[string, string]) ([]resource.Resource, error) {
	resources := make([]resource.Resource, 0, len(l))

	for i, manifest := range l {
		r, err := manifest.Translate(prefix, baseWeight+i, labels...)
		if err != nil {
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}
