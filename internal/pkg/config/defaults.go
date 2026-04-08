// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"go.yaml.in/yaml/v4"
)

// schemaNode is a minimal representation of a JSON Schema node, used for extracting defaults.
type schemaNode struct {
	Default              any                    `json:"default"`
	Properties           map[string]*schemaNode `json:"properties"`
	Definitions          map[string]*schemaNode `json:"definitions"`
	AdditionalProperties *schemaNode            `json:"additionalProperties"`
	Ref                  string                 `json:"$ref"`
	Type                 string                 `json:"type"`
}

// defaultsFromSchema parses the embedded JSON schema and extracts all default values
// into a Params struct. It resolves $ref references and handles inline property overrides.
func defaultsFromSchema() (*Params, error) {
	var root schemaNode
	if err := json.Unmarshal([]byte(schemaData), &root); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	defaults := extractObjectDefaults(&root, root.Definitions)
	if defaults == nil {
		return &Params{}, nil
	}

	yamlData, err := yaml.Marshal(defaults)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal defaults to YAML: %w", err)
	}

	var params Params
	if err := yaml.Unmarshal(yamlData, &params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal defaults into Params: %w", err)
	}

	return &params, nil
}

// extractObjectDefaults recursively walks a schema object node and collects
// default values from its properties into a nested map.
func extractObjectDefaults(node *schemaNode, defs map[string]*schemaNode) map[string]any {
	if node == nil {
		return nil
	}

	result := map[string]any{}

	for propName, prop := range node.Properties {
		if prop.Default != nil {
			result[propName] = prop.Default

			continue
		}

		// Resolve $ref to get the base definition.
		base := resolveSchemaRef(prop.Ref, defs)

		if base != nil {
			// Merge: base definition provides the structure, inline properties may add defaults.
			merged := mergeNodeProperties(base, prop)
			sub := extractObjectDefaults(merged, defs)

			if len(sub) > 0 {
				result[propName] = sub
			}

			continue
		}

		// Inline object without $ref.
		if prop.Type == "object" && prop.AdditionalProperties == nil && len(prop.Properties) > 0 {
			sub := extractObjectDefaults(prop, defs)
			if len(sub) > 0 {
				result[propName] = sub
			}
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// mergeNodeProperties creates a merged schema node where inline property overrides
// (e.g., defaults added next to a $ref) take precedence over base definition properties.
func mergeNodeProperties(base, overlay *schemaNode) *schemaNode {
	merged := &schemaNode{
		Type:       base.Type,
		Properties: make(map[string]*schemaNode, len(base.Properties)),
	}

	maps.Copy(merged.Properties, base.Properties)

	for k, v := range overlay.Properties {
		if existing, ok := merged.Properties[k]; ok {
			// Overlay adds defaults or other annotations to existing base properties.
			combined := *existing
			if v.Default != nil {
				combined.Default = v.Default
			}

			merged.Properties[k] = &combined
		} else {
			merged.Properties[k] = v
		}
	}

	return merged
}

func resolveSchemaRef(ref string, defs map[string]*schemaNode) *schemaNode {
	const prefix = "#/definitions/"

	if !strings.HasPrefix(ref, prefix) {
		return nil
	}

	return defs[strings.TrimPrefix(ref, prefix)]
}
