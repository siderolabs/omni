// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"encoding/json"
	"time"
)

// Auth configures authentication.
//
//nolint:govet
type Auth struct {
	// Auth0 auth type configuration.
	Auth0 Auth0 `yaml:"auth0"`
	// WebAuthn auth type configuration.
	WebAuthn WebAuthn `yaml:"webauthn"`
	// SAML auth type configuration.
	SAML SAML `yaml:"saml"`
	// OIDC auth type configuration.
	OIDC OIDC `yaml:"oidc"`

	// InitialUsers adds the user to the account on the first Omni start up.
	InitialUsers []string `yaml:"initialUsers"`

	// KeyPruner automatically removes the unused public keys registered in Omni.
	KeyPruner KeyPrunerConfig `yaml:"keyPruner"`

	// Suspended makes the account readonly.
	Suspended bool `yaml:"suspended"`

	// InitialServiceAccount creates a service account on the first Omni start up.
	// Writes the service account key to the file defined in the keyPath param.
	//
	// This service account can be used if it is required to run omnictl or a provider using
	// some automation scripts.
	InitialServiceAccount InitialServiceAccount `yaml:"initialServiceAccount"`
}

// Auth0 holds configuration parameters for Auth0.
//
//nolint:govet
type Auth0 struct {
	// InitialUsers adds the user to the account on the first Omni start up.
	//
	// Deprecated: use config.Auth.InitialUsers instead
	InitialUsers []string `yaml:"initialUsers"`
	Domain       string   `yaml:"domain"`
	ClientID     string   `yaml:"clientID"`
	UseFormData  bool     `yaml:"useFormData"`
	Enabled      bool     `yaml:"enabled"`
}

// WebAuthn holds configuration parameters for WebAuthn.
type WebAuthn struct {
	Enabled  bool `yaml:"enabled"`
	Required bool `yaml:"required"`
}

// OIDC holds the configuration parameters for OIDC auth.
type OIDC struct {
	ProviderURL          string   `yaml:"providerURL"`
	ClientID             string   `yaml:"clientID"`
	ClientSecret         string   `yaml:"clientSecret"`
	LogoutURL            string   `yaml:"logoutURL"`
	Scopes               []string `yaml:"scopes"`
	Enabled              bool     `yaml:"enabled"`
	AllowUnverifiedEmail bool     `yaml:"allowUnverifiedEmail"`
}

// SAML holds configuration parameters for SAML auth.
type SAML struct {
	LabelRules     SAMLLabelRules     `yaml:"labelRules"`
	AttributeRules SAMLAttributeRules `yaml:"attributeRules"`
	MetadataURL    string             `yaml:"url" validate:"excluded_with=Metadata"`
	Metadata       string             `yaml:"metadata" validate:"excluded_with=MetadataURL"`
	NameIDFormat   string             `yaml:"nameIDFormat"`
	Enabled        bool               `yaml:"enabled"`
}

// KeyPrunerConfig defines key pruner configs.
type KeyPrunerConfig struct {
	Interval time.Duration `yaml:"interval"`
}

// InitialServiceAccount allows creating a service account for automated omnictl runs on the Omni service deployment.
type InitialServiceAccount struct {
	Role     string        `yaml:"role"`
	KeyPath  string        `yaml:"keyPath"`
	Name     string        `yaml:"name"`
	Lifetime time.Duration `yaml:"lifetime"`
	Enabled  bool          `yaml:"enabled"`
}

// SAMLLabelRules defines mapping of SAML assertion attributes to Omni identity labels.
//
//nolint:recvcheck
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

// SAMLAttributeRules defines mapping of SAML assertion attributes to Omni identity labels.
//
//nolint:recvcheck
type SAMLAttributeRules map[string]string

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
