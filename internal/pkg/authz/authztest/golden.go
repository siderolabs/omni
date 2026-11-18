// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authztest

import (
	"flag"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// CompareGoldenFiles fails when the files differ from the golden files in the directory. With the -update flag it rewrites them.
//
// The content is compared and not the bytes, so the formatting of a golden file does not matter.
func CompareGoldenFiles(t *testing.T, dir string, files map[string][]byte) {
	if *update {
		require.NoError(t, os.RemoveAll(dir))
		require.NoError(t, os.MkdirAll(dir, 0o755))

		for name, content := range files {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o644))
		}

		return
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "golden files are missing, create them with -update")

	// a file which no subject writes anymore fails the test too
	for _, entry := range entries {
		require.Contains(t, files, entry.Name(), "golden file %s has no subject, rewrite the golden files with -update", entry.Name())
	}

	for _, name := range slices.Sorted(maps.Keys(files)) {
		golden, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "golden file %s is missing, create it with -update", name)

		var expected, actual any

		require.NoError(t, yaml.Unmarshal(golden, &expected), "golden file %s", name)
		require.NoError(t, yaml.Unmarshal(files[name], &actual))

		require.Equal(t, expected, actual, "authorization changed in %s, review the diff and rewrite the golden files with -update", name)
	}
}
