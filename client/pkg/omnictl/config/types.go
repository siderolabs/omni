// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// Config represents the omni configuration.
type Config struct {
	Contexts map[string]*Context `yaml:"contexts"`
	Context  string              `yaml:"context"`
	Path     string              `yaml:"-"`
}

// Context represents a context in the config.
type Context struct {
	URL  string `yaml:"url"`
	Auth Auth   `yaml:"auth,omitempty"`
}

// Auth contains the authentication configuration for a context.
type Auth struct {
	// Deprecated: basic auth is not supported, and setting this field has no effect.
	Basic    string   `yaml:"basic,omitempty"`
	SideroV1 SideroV1 `yaml:"siderov1,omitempty"`
}

// SideroV1 is the auth configuration v1.
type SideroV1 struct {
	Identity string `yaml:"identity,omitempty"`
}

// PlaceholderURL is a placeholder url.
const PlaceholderURL = "<placeholder_url>"
