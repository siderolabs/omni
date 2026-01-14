// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template //nolint:testpackage

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverTemplateFiles(t *testing.T) {
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
			name: "single template file",
			setupFiles: map[string]string{
				"cluster.yaml": "kind: Cluster\nname: test",
			},
			inputPath: "cluster.yaml",
			expected:  []string{"cluster.yaml"},
		},
		{
			name: "directory with multiple templates",
			setupFiles: map[string]string{
				"cluster1.yaml": "kind: Cluster\nname: test1",
				"cluster2.yaml": "kind: Cluster\nname: test2",
				"cluster3.yml":  "kind: Cluster\nname: test3",
			},
			inputPath: ".",
			expected:  []string{"cluster1.yaml", "cluster2.yaml", "cluster3.yml"},
		},
		{
			name: "templates in subdirectories",
			setupFiles: map[string]string{
				"prod/cluster.yaml":   "kind: Cluster\nname: prod",
				"dev/cluster.yaml":    "kind: Cluster\nname: dev",
				"staging/cluster.yml": "kind: Cluster\nname: staging",
			},
			inputPath: ".",
			expected: []string{
				"prod/cluster.yaml",
				"dev/cluster.yaml",
				"staging/cluster.yml",
			},
		},
		{
			name: "deeply nested templates",
			setupFiles: map[string]string{
				"region1/dc1/cluster.yaml":      "kind: Cluster\nname: r1dc1",
				"region1/dc2/cluster.yaml":      "kind: Cluster\nname: r1dc2",
				"region2/dc1/prod/cluster.yaml": "kind: Cluster\nname: r2dc1prod",
			},
			inputPath: ".",
			expected: []string{
				"region1/dc1/cluster.yaml",
				"region1/dc2/cluster.yaml",
				"region2/dc1/prod/cluster.yaml",
			},
		},
		{
			name: "mixed yaml and non-yaml files",
			setupFiles: map[string]string{
				"cluster.yaml": "kind: Cluster\nname: test",
				"README.md":    "# Templates",
				"Makefile":     "all:",
				"config.json":  "{}",
				".gitignore":   "*.tmp",
			},
			inputPath: ".",
			expected:  []string{"cluster.yaml"},
		},
		{
			name: "empty directory",
			setupFiles: map[string]string{
				"empty/.gitkeep": "",
			},
			inputPath:     "empty",
			expectError:   true,
			errorContains: "no YAML files found",
		},
		{
			name: "case insensitive extensions",
			setupFiles: map[string]string{
				"cluster1.YAML": "kind: Cluster\nname: c1",
				"cluster2.Yaml": "kind: Cluster\nname: c2",
				"cluster3.YML":  "kind: Cluster\nname: c3",
				"cluster4.yml":  "kind: Cluster\nname: c4",
			},
			inputPath: ".",
			expected:  []string{"cluster1.YAML", "cluster2.Yaml", "cluster3.YML", "cluster4.yml"},
		},
		{
			name:          "non-existent path",
			setupFiles:    map[string]string{},
			inputPath:     "does-not-exist.yaml",
			expectError:   true,
			errorContains: "no such file or directory",
		},
		{
			name: "directory with only subdirectories",
			setupFiles: map[string]string{
				"empty1/.gitkeep": "",
				"empty2/.gitkeep": "",
			},
			inputPath:     ".",
			expectError:   true,
			errorContains: "no YAML files found",
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

			result, err := discoverTemplateFiles(inputPath)

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

func TestDiscoverTemplateFiles_MultipleClusterTemplates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create multiple cluster templates that should be processed independently
	templates := map[string]string{
		"production.yaml": `kind: Cluster
name: production
kubernetes:
  version: v1.30.0
talos:
  version: v1.8.0
---
kind: ControlPlane
machines: []
`,
		"development.yaml": `kind: Cluster
name: development
kubernetes:
  version: v1.30.0
talos:
  version: v1.8.0
---
kind: ControlPlane
machines: []
`,
		"staging.yaml": `kind: Cluster
name: staging
kubernetes:
  version: v1.30.0
talos:
  version: v1.8.0
---
kind: ControlPlane
machines: []
`,
	}

	for name, content := range templates {
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, name),
			[]byte(content),
			0o644,
		))
	}

	result, err := discoverTemplateFiles(tmpDir)
	require.NoError(t, err)
	assert.Len(t, result, 3, "Should discover all 3 template files")

	// Verify each template file exists
	for _, path := range result {
		_, err := os.Stat(path)
		assert.NoError(t, err, "Template file should exist: %s", path)
	}
}

func TestDiscoverTemplateFiles_IgnoresNonYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	files := map[string]string{
		"cluster.yaml":    "kind: Cluster\nname: test",
		"README.md":       "# Documentation",
		"script.sh":       "#!/bin/bash\necho 'test'",
		"config.json":     `{"test": true}`,
		"data.txt":        "plain text",
		"Makefile":        "all:\n\techo 'build'",
		".gitignore":      "*.tmp\n*.log",
		"template.yaml~":  "backup file",
		"#template.yaml#": "emacs backup",
	}

	for name, content := range files {
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, name),
			[]byte(content),
			0o644,
		))
	}

	result, err := discoverTemplateFiles(tmpDir)
	require.NoError(t, err)

	assert.Len(t, result, 1)
	assert.Contains(t, result[0], "cluster.yaml")
}

func TestDiscoverTemplateFiles_SingleFileInput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create multiple files
	files := map[string]string{
		"cluster1.yaml": "kind: Cluster\nname: c1",
		"cluster2.yaml": "kind: Cluster\nname: c2",
		"cluster3.yaml": "kind: Cluster\nname: c3",
	}

	for name, content := range files {
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, name),
			[]byte(content),
			0o644,
		))
	}

	// When given a single file path, should only return that file
	singleFile := filepath.Join(tmpDir, "cluster2.yaml")

	result, err := discoverTemplateFiles(singleFile)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, singleFile, result[0])
}

func TestDiscoverTemplateFiles_RespectsDirStructure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create structure mimicking real-world usage
	structure := map[string]string{
		"environments/production/cluster.yaml":  "kind: Cluster\nname: prod",
		"environments/production/patches.yaml":  "patches:\n  - name: test",
		"environments/development/cluster.yaml": "kind: Cluster\nname: dev",
		"environments/staging/cluster.yaml":     "kind: Cluster\nname: staging",
		"base/common.yaml":                      "kind: Cluster\nname: common",
	}

	for path, content := range structure {
		fullPath := filepath.Join(tmpDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}

	result, err := discoverTemplateFiles(tmpDir)
	require.NoError(t, err)
	assert.Len(t, result, 5, "Should find all 5 YAML files recursively")

	// Verify all files are found
	foundFiles := make(map[string]bool)

	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)

		foundFiles[relPath] = true
	}

	for expectedPath := range structure {
		assert.True(t, foundFiles[expectedPath], "Should find %s", expectedPath)
	}
}
