// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package jsonschema implements tools for validating YAMLs using JSON schema.
package jsonschema

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// Parse parses and compiles the JSON schema file.
func Parse(name string, data string) (*jsonschema.Schema, error) {
	filename := fmt.Sprintf("%s-schema.json", name)

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(data))
	if err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource(filename, schema); err != nil {
		return nil, err
	}

	return compiler.Compile(filename)
}

var nanRegexp = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`(?i)\.nan`) })

// Validate validates the YAML file against the JSON schema.
func Validate(data string, schema *jsonschema.Schema) error {
	// NaN type causes jsonschema validator to crash with nil reference error
	data = nanRegexp().ReplaceAllString(data, "null")

	var v interface{}
	if err := yaml.Unmarshal([]byte(data), &v); err != nil {
		return fmt.Errorf("failed to unmarshal YAML %w", err)
	}

	if v == nil {
		v = map[string]any{}
	}

	return schema.Validate(v)
}
