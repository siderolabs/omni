// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"strings"

	"go.yaml.in/yaml/v4"
)

// ImageLabels describes structure that is stored in the Talos metadata and keeps machine labels
// that are initially assigned to the machine when it connects to Omni.
type ImageLabels struct {
	Labels       map[string]string `yaml:"machineLabels"`
	LegacyLabels map[string]string `yaml:"machineInitialLabels,omitempty"`
}

// Encode converts labels to the serialized value to be stored in the meta partition.
func (l ImageLabels) Encode() ([]byte, error) {
	return yaml.Marshal(l)
}

// ParseLabels reads label from the encoded metadata value.
func ParseLabels(data []byte) (*ImageLabels, error) {
	labels := &ImageLabels{}

	if err := yaml.Unmarshal(data, &labels); err != nil {
		return nil, err
	}

	// fallback to the legacy labels
	if labels.Labels == nil {
		labels.Labels = labels.LegacyLabels
	}

	if labels.Labels == nil {
		return labels, nil
	}

	// trim spaces on both keys and values and overwrite the final result
	finalLabels := make(map[string]string, len(labels.Labels))

	for key, value := range labels.Labels {
		finalLabels[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	labels.Labels = finalLabels

	return labels, nil
}
