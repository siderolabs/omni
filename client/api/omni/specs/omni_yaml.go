// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
	"github.com/siderolabs/gen/xslices"
	"gopkg.in/yaml.v3"
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

// MarshalYAML implements yaml.Marshaler interface.
func (c *ClusterMachineConfigPatchesSpec) MarshalYAML() (any, error) {
	contents := xslices.Map(c.Patches, func(patch string) *yaml.Node {
		style := yaml.FlowStyle
		if len(patch) > 0 && (patch[0] == '\n' || patch[0] == ' ') {
			style = yaml.SingleQuotedStyle
		}

		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Style: style,
			Value: patch,
		}
	})

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "patches"},
			{Kind: yaml.SequenceNode, Tag: "!!seq", Content: contents},
		},
	}, nil
}
