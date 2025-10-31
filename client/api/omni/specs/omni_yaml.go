// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
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
