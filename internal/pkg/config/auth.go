// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import "encoding/json"

// AuthParams configures authentication.
//
//nolint:govet
type AuthParams struct {
	Auth0    Auth0Params    `yaml:"auth0"`
	WebAuthn WebAuthnParams `yaml:"webauthn"`
	SAML     SAMLParams     `yaml:"saml"`

	Suspended bool `yaml:"suspended"`
}

// Auth0Params holds configuration parameters for Auth0.
type Auth0Params struct {
	Domain   string `yaml:"domain"`
	ClientID string `yaml:"clientID"`
	Enabled  bool   `yaml:"enabled"`
}

// WebAuthnParams holds configuration parameters for WebAuthn.
type WebAuthnParams struct {
	Enabled  bool `yaml:"enabled"`
	Required bool `yaml:"required"`
}

// SAMLParams holds configuration parameters for SAML auth.
type SAMLParams struct {
	LabelRules SAMLLabelRules `yaml:"labelRules"`
	URL        string         `yaml:"url"`
	Metadata   string         `yaml:"metadata"`
	Enabled    bool           `yaml:"enabled"`
}

// SAMLLabelRules defines mapping of SAML assertion attributes to Omni identity labels.
type SAMLLabelRules map[string]string

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
