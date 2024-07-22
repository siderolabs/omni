// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit_test

import (
	"embed"
	"fmt"
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
	dir := must.Value(os.MkdirTemp("", "log_file_test"))(t)

	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck

	entries := []entry{
		{shift: time.Second, data: audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.1", Email: "random_email1@example.com"}},
		{shift: time.Minute, data: audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.2", Email: "random_email2@example.com"}},
		{shift: 30 * time.Minute, data: audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.3", Email: "random_email3@example.com"}},
	}

	start := time.Date(2012, 1, 1, 23, 0, 0, 0, time.Local)
	now := start
	file := audit.NewLogFile(dir)

	for _, e := range entries {
		now = now.Add(e.shift)

		require.NoError(t, file.DumpAt(e.data, now))
	}

	checkFiles(t, basicLoader(dir), fsSub(t, currentDay, "currentday"))
}

//go:embed testdata/nextday
var nextDay embed.FS

func TestLogFile_CurrentAndNewDay(t *testing.T) {
	dir := must.Value(os.MkdirTemp("", "log_file_test"))(t)

	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck

	entries := []entry{
		{shift: 0, data: audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.1", Email: "random_email1@example.com"}},
		{shift: 55 * time.Minute, data: audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.2", Email: "random_email2@example.com"}},
		{shift: 5 * time.Minute, data: audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.3", Email: "random_email3@example.com"}},
	}

	start := time.Date(2012, 1, 1, 23, 0, 0, 0, time.Local)
	now := start
	file := audit.NewLogFile(dir)

	for _, e := range entries {
		now = now.Add(e.shift)

		require.NoError(t, file.DumpAt(e.data, now))
	}

	checkFiles(t, basicLoader(dir), fsSub(t, nextDay, "nextday"))
}

//go:embed testdata/concurrent
var concurrent embed.FS

func TestLogFile_CurrentDayConcurrent(t *testing.T) {
	dir := must.Value(os.MkdirTemp("", "log_file_test"))(t)

	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck

	entries := make([]entry, 0, 250)

	for i := range 250 {
		address := fmt.Sprintf("10.10.0.%d", i+1)
		email := fmt.Sprintf("random_email_%d@example.com", i+1)

		entries = append(entries, entry{shift: time.Second, data: audit.Data{UserAgent: "Mozilla/5.0", IPAddress: address, Email: email}})
	}

	start := time.Date(2012, 1, 1, 23, 0, 0, 0, time.Local)
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

	checkFiles(t, sortedLoader(basicLoader(dir)), fsSub(t, concurrent, "concurrent"))
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

func checkFiles(t *testing.T, loader fileLoader, expectedFS subFS) {
	expectedFiles := must.Value(expectedFS.ReadDir("."))(t)

	for _, expectedFile := range expectedFiles {
		if expectedFile.IsDir() {
			t.Fatal("unexpected directory", expectedFile.Name())
		}

		expectedData := string(must.Value(expectedFS.ReadFile(expectedFile.Name()))(t))
		actualData := loader(t, expectedFile.Name())

		require.Equal(t, expectedData, actualData, "file %s", expectedFile.Name())
	}
}

func fsSub(t *testing.T, subFs subFS, folder string) subFS {
	return must.Value(fs.Sub(subFs, filepath.Join("testdata", folder)))(t).(subFS) //nolint:forcetypeassert
}

type fileLoader func(t *testing.T, filename string) string

func basicLoader(dir string) func(t *testing.T, filename string) string {
	return func(t *testing.T, filename string) string {
		return string(must.Value(os.ReadFile(filepath.Join(dir, filename)))(t))
	}
}

func sortedLoader(loader fileLoader) fileLoader {
	return func(t *testing.T, filename string) string {
		data := strings.TrimRight(loader(t, filename), "\n")
		slc := strings.Split(data, "\n")

		slices.Sort(slc)

		return strings.Join(slc, "\n") + "\n"
	}
}
