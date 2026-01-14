// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl //nolint:testpackage

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setupFiles    map[string]string
		name          string
		inputPath     string
		errorContains string
		expected      []string
		expectError   bool
	}{
		{
			name: "single yaml file",
			setupFiles: map[string]string{
				"config.yaml": "metadata:\n  type: test",
			},
			inputPath: "config.yaml",
			expected:  []string{"config.yaml"},
		},
		{
			name: "single yml file",
			setupFiles: map[string]string{
				"config.yml": "metadata:\n  type: test",
			},
			inputPath: "config.yml",
			expected:  []string{"config.yml"},
		},
		{
			name: "directory with multiple yaml files",
			setupFiles: map[string]string{
				"config1.yaml": "metadata:\n  type: test1",
				"config2.yaml": "metadata:\n  type: test2",
				"config3.yml":  "metadata:\n  type: test3",
			},
			inputPath: ".",
			expected:  []string{"config1.yaml", "config2.yaml", "config3.yml"},
		},
		{
			name: "directory with subdirectories",
			setupFiles: map[string]string{
				"config1.yaml":             "metadata:\n  type: test1",
				"subdir/config2.yaml":      "metadata:\n  type: test2",
				"subdir/config3.yml":       "metadata:\n  type: test3",
				"deep/nested/config4.yaml": "metadata:\n  type: test4",
			},
			inputPath: ".",
			expected: []string{
				"config1.yaml",
				"subdir/config2.yaml",
				"subdir/config3.yml",
				"deep/nested/config4.yaml",
			},
		},
		{
			name: "directory with non-yaml files",
			setupFiles: map[string]string{
				"config.yaml": "metadata:\n  type: test",
				"readme.md":   "# README",
				"script.sh":   "#!/bin/bash",
				"data.json":   "{}",
				"config.txt":  "some config",
				"Makefile":    "all:",
			},
			inputPath: ".",
			expected:  []string{"config.yaml"},
		},
		{
			name: "empty directory",
			setupFiles: map[string]string{
				".gitkeep": "",
			},
			inputPath:     ".",
			expectError:   true,
			errorContains: "no YAML files found",
		},
		{
			name: "directory with only subdirectories, no yaml files",
			setupFiles: map[string]string{
				"subdir1/.gitkeep": "",
				"subdir2/.gitkeep": "",
			},
			inputPath:     ".",
			expectError:   true,
			errorContains: "no YAML files found",
		},
		{
			name: "mixed case extensions",
			setupFiles: map[string]string{
				"config1.YAML": "metadata:\n  type: test1",
				"config2.YML":  "metadata:\n  type: test2",
				"config3.Yaml": "metadata:\n  type: test3",
				"config4.yaml": "metadata:\n  type: test4",
			},
			inputPath: ".",
			expected:  []string{"config1.YAML", "config2.YML", "config3.Yaml", "config4.yaml"},
		},
		{
			name:          "non-existent path",
			setupFiles:    map[string]string{},
			inputPath:     "does-not-exist",
			expectError:   true,
			errorContains: "no such file or directory",
		},
		{
			name: "deeply nested structure",
			setupFiles: map[string]string{
				"a/b/c/d/e/config.yaml": "metadata:\n  type: deep",
				"config.yaml":           "metadata:\n  type: root",
			},
			inputPath: ".",
			expected:  []string{"a/b/c/d/e/config.yaml", "config.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()

			// Setup files
			for path, content := range tt.setupFiles {
				fullPath := filepath.Join(tmpDir, path)
				require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
				require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
			}

			// Determine input path
			inputPath := filepath.Join(tmpDir, tt.inputPath)

			result, err := discoverFiles(inputPath)

			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)

			// Convert result to relative paths for comparison
			var relPaths []string

			for _, absPath := range result {
				relPath, err := filepath.Rel(tmpDir, absPath)
				require.NoError(t, err)

				relPaths = append(relPaths, relPath)
			}

			sort.Strings(relPaths)
			sort.Strings(tt.expected)

			assert.Equal(t, tt.expected, relPaths)
		})
	}
}
