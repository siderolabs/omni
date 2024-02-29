// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config implements the config file logic.
package config

import (
	"fmt"
	"os"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

const (
	// OmniConfigEnvVar is the environment variable to override the default config path.
	OmniConfigEnvVar = "OMNICONFIG"

	relativePath       = "omni/config"
	defaultContextName = "default"
)

var (
	// DefaultContext is the context with the default values.
	DefaultContext = Context{
		URL: "grpc://127.0.0.1:8080",
	}

	defaultConfig = Config{
		Context: defaultContextName,
		Contexts: map[string]*Context{
			defaultContextName: &DefaultContext,
		},
	}

	current *Config
)

// Init initializes the Current config and returns it.
func Init(path string, create bool) (*Config, error) {
	conf, err := load(path)

	if os.IsNotExist(err) {
		if !create {
			return nil, err
		}

		defaultConfig.Path = path

		err := defaultConfig.Save()
		if err != nil {
			return nil, err
		}

		conf = &defaultConfig
	}

	current = conf

	return current, nil
}

// load the config from the given explicit path or defaults to the known default config paths.
func load(path string) (*Config, error) {
	var err error

	if path == "" {
		path, err = defaultPath()
		if err != nil {
			return nil, err
		}
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config

	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	config.Path = path

	return &config, nil
}

// Save saves the config to the path it is configured to, or defaults to the known default config paths.
// It modifies the Path to point to the saved file.
func (c *Config) Save() error {
	var err error

	path := c.Path
	if path == "" {
		path, err = defaultPath()
		if err != nil {
			return err
		}
	}

	bytes, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, bytes, 0o600)
	if err != nil {
		return err
	}

	c.Path = path

	return err
}

// Current returns the currently targeted config.
func Current() (*Config, error) {
	if current == nil {
		return nil, fmt.Errorf("config not initialized")
	}

	return current, nil
}

// GetContext returns the context with the given name. If empty, it will return the selected context in the config file.
func (c *Config) GetContext(name string) (*Context, error) {
	if name == "" {
		name = c.Context
	}

	context, ok := c.Contexts[name]
	if !ok {
		return nil, fmt.Errorf("context not found: %s", name)
	}

	return context, nil
}

// Merge in additional contexts from another Config.
//
// Current context is overridden from passed in config.
func (c *Config) Merge(additionalConfigPath string) ([]Rename, error) {
	if additionalConfigPath == "" {
		return nil, fmt.Errorf("additional config path is empty")
	}

	cfg, err := load(additionalConfigPath)
	if err != nil {
		return nil, err
	}

	mappedContexts := map[string]string{}

	var renames []Rename

	for name, ctx := range cfg.Contexts {
		mergedName := name

		if _, exists := c.Contexts[mergedName]; exists {
			for i := 1; ; i++ {
				mergedName = fmt.Sprintf("%s-%d", name, i)

				if _, ctxExists := c.Contexts[mergedName]; !ctxExists {
					break
				}
			}
		}

		mappedContexts[name] = mergedName

		if name != mergedName {
			renames = append(renames, Rename{name, mergedName})
		}

		c.Contexts[mergedName] = ctx
	}

	if cfg.Context != "" {
		c.Context = mappedContexts[cfg.Context]
	}

	return renames, nil
}

func defaultPath() (string, error) {
	path := os.Getenv(OmniConfigEnvVar)
	if path != "" {
		return path, nil
	}

	return xdg.ConfigFile(relativePath)
}
