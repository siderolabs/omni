// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"encoding/json"
)

// String implements pflag.Value.
func (s SAMLLabelRules) String() string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	return string(b)
}

// Set implements pflag.Value.
func (s *SAMLLabelRules) Set(value string) error {
	return json.Unmarshal([]byte(value), &s)
}

// Type implements pflag.Value.
func (SAMLLabelRules) Type() string {
	return "JSON encoded key/value map"
}

// String implements pflag.Value.
func (s SAMLAttributeRules) String() string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	return string(b)
}

// Set implements pflag.Value.
func (s *SAMLAttributeRules) Set(value string) error {
	return json.Unmarshal([]byte(value), &s)
}

// Type implements pflag.Value.
func (SAMLAttributeRules) Type() string {
	return "JSON encoded key/value map"
}
