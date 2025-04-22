// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit_test

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
)

//go:embed testdata/currentday
var currentDay embed.FS

func TestLogFile_CurrentDay(t *testing.T) {
	dir := t.TempDir()

	entries := []entry{
		{shift: time.Second, data: makeAuditData("Mozilla/5.0", "10.10.0.1", "random_email1@example.com")},
		{shift: time.Minute, data: makeAuditData("Mozilla/5.0", "10.10.0.2", "random_email2@example.com")},
		{shift: 30 * time.Minute, data: makeAuditData("Mozilla/5.0", "10.10.0.3", "random_email3@example.com")},
	}

	start := time.Date(2012, 1, 1, 23, 0, 0, 0, time.Local) //nolint:gosmopolitan
	now := start
	file := audit.NewLogFile(dir)

	for _, e := range entries {
		now = now.Add(e.shift)

		require.NoError(t, file.DumpAt(e.data, now))
	}

	equalDirs(
		t,
		fsSub(t, currentDay, "currentday"),
		os.DirFS(dir).(subFS), //nolint:forcetypeassert,errcheck
		defaultCmp,
	)
}

//go:embed testdata/nextday
var nextDay embed.FS

func TestLogFile_CurrentAndNewDay(t *testing.T) {
	dir := t.TempDir()

	entries := []entry{
		{shift: 0, data: makeAuditData("Mozilla/5.0", "10.10.0.1", "random_email1@example.com")},
		{shift: 55 * time.Minute, data: makeAuditData("Mozilla/5.0", "10.10.0.2", "random_email2@example.com")},
		{shift: 5 * time.Minute, data: makeAuditData("Mozilla/5.0", "10.10.0.3", "random_email3@example.com")},
	}

	start := time.Date(2012, 1, 1, 23, 0, 0, 0, time.Local) //nolint:gosmopolitan
	now := start
	file := audit.NewLogFile(dir)

	for _, e := range entries {
		now = now.Add(e.shift)

		require.NoError(t, file.DumpAt(e.data, now))
	}

	equalDirs(
		t,
		fsSub(t, nextDay, "nextday"),
		os.DirFS(dir).(subFS), //nolint:forcetypeassert,errcheck
		defaultCmp,
	)
}

//go:embed testdata/concurrent
var concurrent embed.FS

func TestLogFile_CurrentDayConcurrent(t *testing.T) {
	dir := t.TempDir()

	entries := make([]entry, 0, 250)

	for i := range 250 {
		address := fmt.Sprintf("10.10.0.%d", i+1)
		email := fmt.Sprintf("random_email_%d@example.com", i+1)

		entries = append(entries, entry{shift: time.Second, data: makeAuditData("Mozilla/5.0", address, email)})
	}

	start := time.Date(2012, 1, 1, 23, 0, 0, 0, time.Local) //nolint:gosmopolitan
	now := start
	file := audit.NewLogFile(dir)

	t.Run("concurrent", func(t *testing.T) {
		for _, e := range entries {
			now = now.Add(e.shift)
			nowCopy := now

			t.Run("", func(t *testing.T) {
				t.Parallel()

				require.NoError(t, file.DumpAt(e.data, nowCopy))
			})
		}
	})

	equalDirs(
		t,
		fsSub(t, concurrent, "concurrent"),
		&sortedFileFS{
			subFS: os.DirFS(dir).(subFS), //nolint:forcetypeassert,errcheck
		},
		defaultCmp,
	)
}

//nolint:govet
type entry struct {
	shift time.Duration
	data  audit.Data
}

type subFS interface {
	fs.ReadFileFS
	fs.ReadDirFS
}

func fsSub(t *testing.T, subFs subFS, folder string) subFS {
	return must.Value(fs.Sub(subFs, filepath.Join("testdata", folder)))(t).(subFS) //nolint:forcetypeassert,errcheck
}

func equalDirs(t *testing.T, expected, actual subFS, cmpFn func(t *testing.T, expected, actual string)) {
	expectedFiles := must.Value(expected.ReadDir("."))(t)
	actualFiles := must.Value(actual.ReadDir("."))(t)

	if len(expectedFiles) != len(actualFiles) {
		t.Fatalf("expected %v files, got %v", expectedFiles, actualFiles)
	}

	for _, actualFile := range actualFiles {
		if actualFile.IsDir() {
			t.Fatal("unexpected directory", actualFile.Name())
		}

		name := actualFile.Name()

		expectedContent := must.Value(expected.ReadFile(name))(t)
		actualContent := must.Value(actual.ReadFile(name))(t)

		cmpFn(t, string(expectedContent), string(actualContent))
	}
}

type sortedFileFS struct{ subFS }

func (s *sortedFileFS) ReadFile(name string) ([]byte, error) {
	b, err := s.subFS.ReadFile(name)
	if err != nil {
		return nil, err
	}

	slc := strings.Split(strings.TrimRight(string(b), "\n"), "\n")

	slices.Sort(slc)

	return []byte(strings.Join(slc, "\n") + "\n"), nil
}

func defaultCmp(t *testing.T, expected string, actual string) {
	require.Equal(t, expected, actual)
}

//nolint:unparam
func makeAuditData(agent, _, email string) audit.Data {
	return audit.Data{
		Session: audit.Session{
			UserAgent: agent,
			Email:     email,
		},
	}
}

//go:embed testdata/multifile
var multifile embed.FS

func TestStreamLogFiles(t *testing.T) {
	tempDir := t.TempDir()
	fullpath := filepath.Join(tempDir, "testdata", "multifile")

	require.NoError(t, os.CopyFS(tempDir, multifile))

	var builder strings.Builder

	at := time.Date(2012, 1, 30, 0, 0, 0, 0, time.Local) //nolint:gosmopolitan

	_, err := io.Copy(&builder, must.Value(audit.NewLogFile(fullpath).ReadAuditLog30Days(at))(t))
	require.NoError(t, err)

	require.Equal(t, "Hello\nWorld\n!!!\n", builder.String())
}
