// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package jsonschema implements tools for validating YAMLs using JSON schema.
package jsonschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var defaultPrinter = message.NewPrinter(language.English)

// Schema wraps a compiled JSON schema with additional metadata for human-friendly error reporting.
type Schema struct {
	Schema        *jsonschema.Schema
	nanRegexp     *regexp.Regexp
	flagMap       map[string]string // maps slash-delimited instance paths to CLI flag names
	patternMsgMap map[string]string // maps slash-delimited instance paths to custom pattern error messages
}

// ConfigValidationError is a human-friendly validation error wrapping the original jsonschema.ValidationError.
type ConfigValidationError struct {
	Original *jsonschema.ValidationError
	Messages []string
}

func (e *ConfigValidationError) Error() string {
	var sb strings.Builder

	for i, msg := range e.Messages {
		if i > 0 {
			sb.WriteString("\n")
		}

		sb.WriteString("- ")
		sb.WriteString(msg)
	}

	return sb.String()
}

func (e *ConfigValidationError) Unwrap() error {
	return e.Original
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

	flagMap, patternMsgMap, err := buildExtensionMaps(data)
	if err != nil {
		return nil, fmt.Errorf("failed to build extension maps from schema: %w", err)
	}

	return &Schema{
		Schema:        sch,
		nanRegexp:     regexp.MustCompile(`(?i)\.nan`),
		flagMap:       flagMap,
		patternMsgMap: patternMsgMap,
	}, nil
}

// Validate validates the given YAML data against the schema, returning human-friendly errors.
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

	err := schema.Schema.Validate(v)
	if err == nil {
		return nil
	}

	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return err
	}

	return schema.humanizeError(validationErr)
}

// Description returns the description of a field in the schema by its dot-delimited path.
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

// FlagName returns the CLI flag name for the given slash-delimited instance path,
// or empty string if no flag is defined.
func (schema *Schema) FlagName(instancePath string) string {
	return schema.flagMap[instancePath]
}

// PatternMessage returns the custom pattern error message for the given slash-delimited instance path,
// or empty string if no custom message is defined.
func (schema *Schema) PatternMessage(instancePath string) string {
	return schema.patternMsgMap[instancePath]
}

func (schema *Schema) humanizeError(verr *jsonschema.ValidationError) *ConfigValidationError {
	leaves := collectLeafErrors(verr)

	messages := make([]string, 0, len(leaves))

	for _, leaf := range leaves {
		messages = append(messages, schema.formatLeafError(leaf))
	}

	return &ConfigValidationError{
		Messages: messages,
		Original: verr,
	}
}

// collectLeafErrors walks the validation error tree and collects leaf errors
// (those with concrete validation kinds, not wrapper/grouping kinds).
func collectLeafErrors(validationErr *jsonschema.ValidationError) []*jsonschema.ValidationError {
	switch validationErr.ErrorKind.(type) {
	case *kind.Schema, *kind.Reference, *kind.Group, *kind.AllOf, *kind.AnyOf, *kind.OneOf:
		leaves := make([]*jsonschema.ValidationError, 0, len(validationErr.Causes))

		for _, cause := range validationErr.Causes {
			leaves = append(leaves, collectLeafErrors(cause)...)
		}

		return leaves
	default:
		return []*jsonschema.ValidationError{validationErr}
	}
}

func (schema *Schema) formatLeafError(leaf *jsonschema.ValidationError) string {
	switch ek := leaf.ErrorKind.(type) {
	case *kind.Required:
		return schema.formatRequiredError(leaf, ek)
	case *kind.Type:
		return schema.formatFieldError(leaf.InstanceLocation, formatTypeMessage(ek))
	case *kind.MinLength:
		return schema.formatFieldError(leaf.InstanceLocation, formatMinLengthMessage(ek))
	case *kind.MaxLength:
		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("must have at most %d characters", ek.Want))
	case *kind.Minimum:
		return schema.formatFieldError(leaf.InstanceLocation, formatMinimumMessage(ek))
	case *kind.Maximum:
		want, _ := ek.Want.Float64()

		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("must be at most %v", want))
	case *kind.ExclusiveMinimum:
		want, _ := ek.Want.Float64()

		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("must be greater than %v", want))
	case *kind.ExclusiveMaximum:
		want, _ := ek.Want.Float64()

		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("must be less than %v", want))
	case *kind.MultipleOf:
		want, _ := ek.Want.Float64()

		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("must be a multiple of %v", want))
	case *kind.Pattern:
		return schema.formatFieldError(leaf.InstanceLocation, schema.formatPatternMessage(leaf.InstanceLocation, ek))
	case *kind.Format:
		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("must be a valid %s", ek.Want))
	case *kind.Enum:
		return schema.formatFieldError(leaf.InstanceLocation, formatEnumMessage(ek))
	case *kind.Const:
		return schema.formatFieldError(leaf.InstanceLocation, formatConstMessage(ek))
	case *kind.AdditionalProperties:
		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("has unknown properties: %s", strings.Join(ek.Properties, ", ")))
	case *kind.MinItems:
		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("must have at least %d items", ek.Want))
	case *kind.MaxItems:
		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("must have at most %d items", ek.Want))
	case *kind.UniqueItems:
		return schema.formatFieldError(leaf.InstanceLocation, fmt.Sprintf("items at index %d and %d must be unique", ek.Duplicates[0], ek.Duplicates[1]))
	default:
		return formatOriginal(leaf)
	}
}

func (schema *Schema) formatRequiredError(leaf *jsonschema.ValidationError, ek *kind.Required) string {
	messages := make([]string, 0, len(ek.Missing))

	for _, missing := range ek.Missing {
		childLocation := append(slices.Clone(leaf.InstanceLocation), missing)
		messages = append(messages, schema.formatFieldError(childLocation, "is required"))
	}

	return strings.Join(messages, "; ")
}

func (schema *Schema) formatFieldError(instanceLocation []string, message string) string {
	dottedPath := "." + strings.Join(instanceLocation, ".")
	slashPath := "/" + strings.Join(instanceLocation, "/")
	flagName := schema.FlagName(slashPath)

	if flagName != "" {
		return fmt.Sprintf("config value %q or flag \"--%s\": %s", dottedPath, flagName, message)
	}

	return fmt.Sprintf("config value %q: %s", dottedPath, message)
}

func formatTypeMessage(ek *kind.Type) string {
	if ek.Got == "null" {
		return "is required but was not set"
	}

	return fmt.Sprintf("must be of type %s, but got %s", strings.Join(ek.Want, " or "), ek.Got)
}

func formatMinLengthMessage(ek *kind.MinLength) string {
	if ek.Want == 1 {
		return "must not be empty"
	}

	return fmt.Sprintf("must have at least %d characters", ek.Want)
}

func formatMinimumMessage(ek *kind.Minimum) string {
	want, _ := ek.Want.Float64()

	return fmt.Sprintf("must be at least %v", want)
}

func (schema *Schema) formatPatternMessage(instanceLocation []string, ek *kind.Pattern) string {
	slashPath := "/" + strings.Join(instanceLocation, "/")

	if msg := schema.PatternMessage(slashPath); msg != "" {
		return msg
	}

	return fmt.Sprintf("does not match the expected pattern '%s'", ek.Want)
}

func formatEnumMessage(ek *kind.Enum) string {
	values := make([]string, 0, len(ek.Want))
	for _, v := range ek.Want {
		values = append(values, fmt.Sprintf("'%v'", v))
	}

	return fmt.Sprintf("must be one of %s", strings.Join(values, ", "))
}

func formatConstMessage(ek *kind.Const) string {
	return fmt.Sprintf("must be %v", ek.Want)
}

func formatOriginal(leaf *jsonschema.ValidationError) string {
	path := "/" + strings.Join(leaf.InstanceLocation, "/")
	msg := leaf.ErrorKind.LocalizedString(defaultPrinter)

	return fmt.Sprintf("at '%s': %s", path, msg)
}

// buildExtensionMaps parses the raw JSON schema and collects x-cli-flag and x-pattern-message
// annotations into maps from slash-delimited paths to their values.
func buildExtensionMaps(data string) (flagMap, patternMsgMap map[string]string, err error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal schema JSON: %w", err)
	}

	definitions, ok := raw["definitions"].(map[string]any)
	if !ok {
		definitions = map[string]any{}
	}

	flagMap = map[string]string{}
	patternMsgMap = map[string]string{}

	if props, ok := raw["properties"].(map[string]any); ok {
		walkProperties(props, definitions, "", flagMap, patternMsgMap)
	}

	return flagMap, patternMsgMap, nil
}

func walkProperties(props, definitions map[string]any, pathPrefix string, flagMap, patternMsgMap map[string]string) {
	for name, propRaw := range props {
		propObj, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}

		currentPath := pathPrefix + "/" + name

		// Check extensions on the local property (takes precedence over resolved $ref)
		collectExtensions(propObj, currentPath, flagMap, patternMsgMap)

		resolved := resolveRef(propObj, definitions)

		// Check extensions on the resolved definition (only if not already set locally)
		collectExtensions(resolved, currentPath, flagMap, patternMsgMap)

		// Walk sub-properties from the resolved definition
		if subProps, ok := resolved["properties"].(map[string]any); ok {
			walkProperties(subProps, definitions, currentPath, flagMap, patternMsgMap)
		}

		// Walk local property overrides (e.g., x-cli-flag on $ref sites), local takes precedence
		if localProps, ok := propObj["properties"].(map[string]any); ok {
			walkProperties(localProps, definitions, currentPath, flagMap, patternMsgMap)
		}
	}
}

func collectExtensions(obj map[string]any, path string, flagMap, patternMsgMap map[string]string) {
	if flag, ok := obj["x-cli-flag"].(string); ok {
		if _, exists := flagMap[path]; !exists {
			flagMap[path] = flag
		}
	}

	if msg, ok := obj["x-pattern-message"].(string); ok {
		if _, exists := patternMsgMap[path]; !exists {
			patternMsgMap[path] = msg
		}
	}
}

func resolveRef(obj, definitions map[string]any) map[string]any {
	ref, ok := obj["$ref"].(string)
	if !ok {
		return obj
	}

	// Parse "#/definitions/X"
	const prefix = "#/definitions/"
	if !strings.HasPrefix(ref, prefix) {
		return obj
	}

	defName := ref[len(prefix):]

	def, ok := definitions[defName].(map[string]any)
	if !ok {
		return obj
	}

	// Recurse in case the definition itself has a $ref
	return resolveRef(def, definitions)
}
