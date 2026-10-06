// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

const redactedValue = "<redacted>"

// Secret is a config value that is left out of the JSON form of the config, which is the one the
// logger writes. YAML is untouched, so the config still round-trips and still validates.
type Secret string

// String implements fmt.Stringer. The value is still reachable with a string conversion.
func (s Secret) String() string {
	if s == "" {
		return ""
	}

	return redactedValue
}

// MarshalJSON implements json.Marshaler.
func (s Secret) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte(`""`), nil
	}

	return []byte(`"` + redactedValue + `"`), nil
}
