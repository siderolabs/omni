// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config implements the config file logic.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/siderolabs/go-api-signature/pkg/fileutils"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"gopkg.in/yaml.v3"
)

const (
	// OmniConfigEnvVar is the environment variable to override the default config path.
	OmniConfigEnvVar = "OMNICONFIG"

	omniConfigDirectory = "omni"
	omniConfigFile      = "config"
	OmniRelativePath    = omniConfigDirectory + "/" + omniConfigFile
	defaultContextName  = "default"
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

	switch {
	case os.IsNotExist(err):
		if !create {
			return nil, err
		}

		defaultConfig.Path = path

		err = defaultConfig.Save()
		if err != nil {
			return nil, err
		}

		conf = &defaultConfig
	case err != nil:
		return nil, err
	}

	current = conf

	return current, nil
}

// load the config from the given explicit path or defaults to the known default config paths.
func load(path string) (*Config, error) {
	var err error

	if path == "" {
		path, err = defaultPath(true)
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
		path, err = defaultPath(false)
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
		return nil, errors.New("config not initialized")
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

		if c.Contexts == nil {
			c.Contexts = map[string]*Context{}
		}

		c.Contexts[mergedName] = ctx
	}

	if cfg.Context != "" {
		c.Context = mappedContexts[cfg.Context]
	}

	return renames, nil
}

//nolint:gocognit
func defaultPath(readOnly bool) (string, error) {
	path := os.Getenv(OmniConfigEnvVar)
	if path != "" {
		return path, nil
	}

	baseDir, err := config.GetTalosDirectory()
	if err != nil {
		if readOnly && fileutils.FileExists(filepath.Join(xdg.ConfigHome, OmniRelativePath)) {
			fmt.Fprintf(os.Stderr, "WARN: Failed to determine Talos directory, falling back to deprecated Omni config location: '%s' for reading.\n",
				filepath.Join(xdg.ConfigHome, OmniRelativePath))

			return xdg.ConfigFile(OmniRelativePath)
		}

		return "", err
	}

	return evaluatePath(readOnly, baseDir)
}

func evaluatePath(readOnly bool, baseDir string) (string, error) {
	if !fileutils.FileExists(filepath.Join(baseDir, OmniRelativePath)) && fileutils.FileExists(filepath.Join(xdg.ConfigHome, OmniRelativePath)) {
		// Ensure the Talos home directory exists and is writable.
		if _, err := ensurePath(filepath.Join(baseDir, omniConfigDirectory)); err != nil {
			// Talos home directory is not writable, but we can still read the config from XDG config directory
			if readOnly {
				fmt.Fprintf(os.Stderr, "WARN: Default Omni config location: '%s' is not writable, falling back to deprecated Omni config location: '%s' for reading.\n",
					filepath.Join(baseDir, OmniRelativePath), filepath.Join(xdg.ConfigHome, OmniRelativePath))

				return xdg.ConfigFile(OmniRelativePath)
			}

			return "", err
		}

		if fileutils.IsWritable(filepath.Join(baseDir, omniConfigDirectory)) {
			// Attempt to copy the config from XDG config directory to Talos home directory.
			// This normally shouldn't fail, but if it does then fail with error instead of falling back to XDG config directory.
			return copyConfig(filepath.Join(xdg.ConfigHome, OmniRelativePath), filepath.Join(baseDir, omniConfigDirectory))
		}
	}

	return ensurePath(filepath.Join(baseDir, omniConfigDirectory))
}

func ensurePath(path string) (string, error) {
	if err := os.MkdirAll(path, os.ModeDir|0o700); err != nil {
		return "", err
	}

	return filepath.Join(path, omniConfigFile), nil
}

func copyConfig(src, dstDir string) (string, error) {
	dst, err := ensurePath(dstDir)
	if err != nil {
		return "", fmt.Errorf("failed to ensure path %q: %w", dst, err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("failed to read source config file %q: %w", src, err)
	}

	err = os.WriteFile(dst, data, 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to write destination config file %q: %w", dst, err)
	}

	fmt.Fprintf(os.Stderr, "INFO: Omni config was copied from deprecated default location: '%s' to new default location: '%s'\n", src, dst)

	return dst, nil
}

// CustomSideroV1KeysDirPath returns the custom SideroV1 auth keys directory path if it's provided as command line flag or with environment variable.
func CustomSideroV1KeysDirPath(path string) string {
	if path != "" {
		return path
	}

	return os.Getenv(constants.SideroV1KeysDirEnvVar)
}
