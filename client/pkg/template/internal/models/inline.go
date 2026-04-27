// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"bytes"
	"fmt"

	"go.yaml.in/yaml/v4"
)

// InlineContent represents inline YAML content embedded into a template.
//
// It accepts three input forms:
//   - a single YAML mapping,
//   - a sequence of YAML mappings (multi-document),
//   - a raw YAML literal block scalar (bytes).
//
//nolint:recvcheck
type InlineContent struct {
	raw  []byte
	docs []map[string]any
}

// NewInlineContent builds InlineContent from one or more YAML mappings.
func NewInlineContent(docs ...map[string]any) *InlineContent {
	return &InlineContent{docs: docs}
}

// NewInlineContentBytes builds InlineContent from raw YAML bytes.
func NewInlineContentBytes(raw []byte) *InlineContent {
	return &InlineContent{raw: raw}
}

// IsZero reports whether the InlineContent is empty.
func (i InlineContent) IsZero() bool {
	return i.raw == nil && len(i.docs) == 0
}

// Bytes returns the YAML-encoded inline content.
//
// Multi-document content is encoded with the standard YAML multi-document framing.
func (i InlineContent) Bytes() ([]byte, error) {
	if i.raw != nil {
		return i.raw, nil
	}

	if len(i.docs) == 0 {
		return nil, nil
	}

	var buf bytes.Buffer

	encoder := yaml.NewEncoder(&buf)

	for _, doc := range i.docs {
		if err := encoder.Encode(doc); err != nil {
			return nil, err
		}
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (i *InlineContent) UnmarshalYAML(node *yaml.Node) error {
	//nolint:exhaustive
	switch node.Kind {
	case yaml.ScalarNode:
		// Only allow explicit strings
		if node.Tag != "!!str" {
			return fmt.Errorf("expected string, but got %s", node.Tag)
		}

		i.raw = []byte(node.Value)

		return nil
	case yaml.MappingNode:
		var m map[string]any

		if err := node.Decode(&m); err != nil {
			return err
		}

		i.docs = []map[string]any{m}

		return nil
	case yaml.SequenceNode:
		var s []map[string]any

		if err := node.Decode(&s); err != nil {
			return err
		}

		i.docs = s

		return nil
	default:
		return fmt.Errorf("unsupported inline node kind %d", node.Kind)
	}
}

// MarshalYAML implements yaml.Marshaler.
func (i InlineContent) MarshalYAML() (any, error) {
	switch {
	case i.raw != nil:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Style: yaml.LiteralStyle,
			Value: string(i.raw),
		}, nil
	case len(i.docs) == 1:
		return i.docs[0], nil
	case len(i.docs) > 1:
		return i.docs, nil
	default:
		return nil, nil //nolint:nilnil
	}
}
