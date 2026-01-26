// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package jsonschema implements tools for validating YAMLs using JSON schema.
package jsonschema

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"
)

type Schema struct {
	Schema    *jsonschema.Schema
	nanRegexp *regexp.Regexp
}

// Parse parses and compiles the JSON schema file.
func Parse(name string, data string) (*Schema, error) {
	filename := fmt.Sprintf("%s-schema.json", name)

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(data))
	if err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource(filename, schema); err != nil {
		return nil, err
	}

	sch, err := compiler.Compile(filename)
	if err != nil {
		return nil, err
	}

	return &Schema{
		Schema:    sch,
		nanRegexp: regexp.MustCompile(`(?i)\.nan`),
	}, nil
}

func (schema *Schema) Validate(data string) error {
	// NaN type causes jsonschema validator to crash with nil reference error
	data = schema.nanRegexp.ReplaceAllString(data, "null")

	var v any
	if err := yaml.Unmarshal([]byte(data), &v); err != nil {
		return fmt.Errorf("failed to unmarshal YAML %w", err)
	}

	if v == nil {
		v = map[string]any{}
	}

	return schema.Schema.Validate(v)
}

func (schema *Schema) Description(fieldPath string) string {
	fields := strings.Split(fieldPath, ".")

	s := schema.Schema
	for _, field := range fields {
		for s.Ref != nil { // expand the refs
			s = s.Ref
		}

		prop, ok := s.Properties[field]
		if !ok {
			return ""
		}

		s = prop
	}

	return s.Description
}
