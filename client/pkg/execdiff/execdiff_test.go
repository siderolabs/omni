// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package execdiff_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/execdiff"
)

func init() {
	// Disable colorization to keep golden-string comparisons stable.
	color.NoColor = true
}

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		parts []string
		want  string
	}{
		{"simple", []string{"Cluster", "prod"}, "Cluster-prod"},
		{"slash in id", []string{"Config", "prod/access-policies"}, "Config-prod_access-policies"},
		{"backslash in id", []string{"Config", `prod\bad`}, "Config-prod_bad"},
		{"parent traversal", []string{"Config", ".."}, "Config-__"},
		{"nested traversal", []string{"Config", "a/../b"}, "Config-a____b"},
		{"empty part", []string{"Config", ""}, "Config-_"},
		{"null byte", []string{"Config", "a\x00b"}, "Config-a_b"},
		{"colon", []string{"Config", "ns:name"}, "Config-ns_name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, execdiff.SanitizeFilename(tc.parts...))
		})
	}
}

func TestBuiltinNoDiff(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	d := execdiff.New(&buf)

	yaml := []byte("a: 1\nb: 2\n")
	require.NoError(t, d.AddDiff("R(one)", "r-one.yaml", yaml, yaml))

	hasDiff, err := d.Flush()
	require.NoError(t, err)
	assert.False(t, hasDiff)
	assert.Empty(t, buf.String())
}

func TestBuiltinDiffRendered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	d := execdiff.New(&buf)

	oldYAML := []byte("a: 1\nb: 2\n")
	newYAML := []byte("a: 1\nb: 3\n")

	require.NoError(t, d.AddDiff("R(one)", "r-one.yaml", oldYAML, newYAML))

	hasDiff, err := d.Flush()
	require.NoError(t, err)
	assert.True(t, hasDiff)

	got := buf.String()
	assert.Contains(t, got, "--- R(one)")
	assert.Contains(t, got, "+++ R(one)")
	assert.Contains(t, got, "-b: 2")
	assert.Contains(t, got, "+b: 3")
	assert.NotContains(t, got, "--- a\n+++ b\n", "generic diff header should be stripped")
}

func TestBuiltinCreateAndDelete(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	d := execdiff.New(&buf)

	require.NoError(t, d.AddDiff("R(new)", "r-new.yaml", nil, []byte("a: 1\n")))
	require.NoError(t, d.AddDiff("R(old)", "r-old.yaml", []byte("a: 1\n"), nil))

	hasDiff, err := d.Flush()
	require.NoError(t, err)
	assert.True(t, hasDiff)

	got := buf.String()
	assert.Contains(t, got, "--- R(new)")
	assert.Contains(t, got, "+a: 1")
	assert.Contains(t, got, "--- R(old)")
	assert.Contains(t, got, "-a: 1")
}

func TestAddDiffRejectsUnsafeFilenamesInExternalMode(t *testing.T) {
	t.Setenv(execdiff.EnvExternalDiff, "diff -u")

	d := execdiff.New(&bytes.Buffer{})

	for _, unsafe := range []string{
		"",
		"../escape.yaml",
		"sub/dir.yaml",
		".",
		"..",
	} {
		t.Run(unsafe, func(t *testing.T) {
			err := d.AddDiff("R", unsafe, []byte("a: 1"), []byte("a: 2"))
			assert.Error(t, err, "expected rejection for %q", unsafe)
		})
	}
}

func TestAddDiffRejectsAbsolutePathsInExternalMode(t *testing.T) {
	t.Setenv(execdiff.EnvExternalDiff, "diff -u")

	d := execdiff.New(&bytes.Buffer{})

	abs := "/tmp/escape.yaml"
	if runtime.GOOS == "windows" {
		abs = `C:\tmp\escape.yaml`
	}

	err := d.AddDiff("R", abs, []byte("a: 1"), []byte("a: 2"))
	assert.Error(t, err)
}

// TestExternalDiffExitCodes verifies the exit-code contract with a stub script
// that we generate at runtime: 0 => no diff, 1 => diff found, 2 => error.
func TestExternalDiffExitCodes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub not supported on windows")
	}

	stub := writeShellStub(t)

	for _, tc := range []struct {
		name    string
		exit    string
		wantErr bool
		want    bool
	}{
		{"no diff", "0", false, false},
		{"diff found", "1", false, true},
		{"error", "2", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(execdiff.EnvExternalDiff, stub+" "+tc.exit)

			var buf bytes.Buffer

			d := execdiff.New(&buf)
			require.NoError(t, d.AddDiff("R(x)", "r-x.yaml", []byte("a: 1\n"), []byte("a: 2\n")))

			hasDiff, err := d.Flush()
			if tc.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, hasDiff)
		})
	}
}

func TestExternalDiffReceivesFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub not supported on windows")
	}

	stub := writeRecordingStub(t)
	recordFile := stub + ".out"

	t.Setenv(execdiff.EnvExternalDiff, stub+" "+recordFile)

	var buf bytes.Buffer

	d := execdiff.New(&buf)
	require.NoError(t, d.AddDiff("R(x)", "r-x.yaml", []byte("old-content"), []byte("new-content")))

	// Exit 1 => diff found. runExternal translates that to hasDiff=true with no error.
	hasDiff, err := d.Flush()
	require.NoError(t, err)
	assert.True(t, hasDiff)

	rec, readErr := os.ReadFile(recordFile)
	require.NoError(t, readErr)

	recStr := string(rec)
	assert.Contains(t, recStr, "old-content")
	assert.Contains(t, recStr, "new-content")
}

func TestExternalModeRejectsEmptyCommand(t *testing.T) {
	t.Setenv(execdiff.EnvExternalDiff, "   ")

	d := execdiff.New(&bytes.Buffer{})
	require.NoError(t, d.AddDiff("R(x)", "r-x.yaml", []byte("a"), []byte("b")))

	_, err := d.Flush()
	assert.Error(t, err)
}

func TestFlushNoEntriesExternalMode(t *testing.T) {
	t.Setenv(execdiff.EnvExternalDiff, "false-command")

	d := execdiff.New(&bytes.Buffer{})

	hasDiff, err := d.Flush()
	require.NoError(t, err)
	assert.False(t, hasDiff)
}

// writeShellStub writes a shell script that exits with the status code
// passed as its final argument. External diff receives live+merged dirs as
// its last two args; the test passes an extra exit-code arg before those,
// so $1 is the desired exit code.
func writeShellStub(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "stub.sh")

	body := "#!/bin/sh\nexit \"$1\"\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))

	return path
}

// writeRecordingStub writes a shell script that concatenates the contents of
// both temp dirs into the file passed as $1, then exits 1 (diff found).
func writeRecordingStub(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "record.sh")

	body := strings.Join([]string{
		"#!/bin/sh",
		"out=\"$1\"",
		"shift",
		"live=\"$1\"",
		"merged=\"$2\"",
		"{ cat \"$live\"/* 2>/dev/null; echo ---; cat \"$merged\"/* 2>/dev/null; } > \"$out\"",
		"exit 1",
	}, "\n") + "\n"

	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))

	return path
}
