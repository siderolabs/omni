// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import "go.yaml.in/yaml/v4"

// OptionalList is a list of strings that tells an unset list apart from an explicitly empty one.
//
// An unset list is omitted from YAML, while a defined list is rendered as a sequence, including the
// empty `[]`. This lets a template express "leave this as it is" separately from "clear it".
//
//nolint:recvcheck
type OptionalList struct {
	value   []string
	defined bool
}

// NewOptionalList returns a defined OptionalList holding the given values.
func NewOptionalList(values []string) OptionalList {
	return OptionalList{value: values, defined: true}
}

// Get returns the list and whether it is defined.
func (l OptionalList) Get() (values []string, defined bool) {
	return l.value, l.defined
}

// Set sets the list, marking it as defined.
func (l *OptionalList) Set(values []string) {
	l.value = values
	l.defined = true
}

// IsZero reports whether the list is unset, so that it is omitted from YAML.
func (l OptionalList) IsZero() bool {
	return !l.defined
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (l *OptionalList) UnmarshalYAML(node *yaml.Node) error {
	var values []string

	if err := node.Decode(&values); err != nil {
		return err
	}

	l.value = values
	l.defined = true

	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (l OptionalList) MarshalYAML() (any, error) {
	return l.value, nil
}
