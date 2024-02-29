// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build tools

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_extractTsGenDirective(t *testing.T) {
	directive, ok := extractTsGenDirective("//tsgen:expected")
	require.True(t, ok)
	require.Equal(t, "expected", directive)
	directive, ok = extractTsGenDirective("//tsgen:")
	require.True(t, ok)
	require.Equal(t, "", directive)
	directive, ok = extractTsGenDirective("//tsgena:")
	require.False(t, ok)
	require.Equal(t, "", directive)
}

func Test_run(t *testing.T) {
	t.Run("should return error if input file is not found", func(t *testing.T) {
		err := run([]string{"./testdata/not-found-dir"}, "tools", "testdata/out")
		require.Error(t, err)
	})

	t.Run("should properly parse pkg and give output", func(t *testing.T) {
		err := run([]string{"./testdata/good/pkg/."}, "tools", "testdata/good/out/resources.ts")
		require.NoError(t, err)
		t.Cleanup(func() {
			err := os.RemoveAll("testdata/good/out")
			require.NoError(t, err)
		})

		actual, err := os.ReadFile("testdata/good/out/resources.ts")
		require.NoError(t, err)
		expected, err := os.ReadFile("testdata/good/expected/resources.ts")
		require.NoError(t, err)
		require.Equal(t, string(expected), string(actual))
	})

	t.Run("should fail on incorrect input", func(t *testing.T) {
		err := run([]string{"./testdata/bad/pkg"}, "tools", "testdata/bad/resources.ts")
		require.EqualError(t, err, "empty directive in MyConst")
	})
}
