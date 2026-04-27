// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
	"fmt"

	"go.yaml.in/yaml/v4"
)

// MarshalYAML implements yaml.Marshaler interface.
func (c *ClusterMachineSpec) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "kubernetes_version"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: c.KubernetesVersion},
		},
	}, nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (c *ClusterMachineSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected a mapping node, got %v", node.Kind)
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		switch keyNode.Value {
		case "kubernetes_version":
			if err := valueNode.Decode(&c.KubernetesVersion); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected field: %s", keyNode.Value)
		}
	}

	return nil
}
