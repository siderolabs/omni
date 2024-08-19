// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit_test

import (
	_ "embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
)

//go:embed testdata/log/2012-01-01.jsonlog
var logFile []byte

func TestTruncateToDate(t *testing.T) {
	truncated := audit.TruncateToDate(time.Date(2012, 1, 1, 23, 33, 0, 0, time.UTC))

	require.Equal(t, "2012-01-01", truncated.Format("2006-01-02"))
}

func TestFindOldFiles(t *testing.T) {
	mapFS := fstest.MapFS{
		"logdir/2011-12-31.jsonlog": &fstest.MapFile{
			Data:    logFile,
			Mode:    fs.ModePerm,
			ModTime: time.Date(2011, 12, 31, 23, 59, 59, 999, time.UTC),
		},
		"logdir/2012-01-01.jsonlog": &fstest.MapFile{
			Data:    logFile,
			Mode:    fs.ModePerm,
			ModTime: time.Date(2012, 1, 2, 11, 0, 0, 0, time.UTC),
		},
		"logdir/2012-01-02.jsonlog": &fstest.MapFile{
			Data:    logFile,
			Mode:    fs.ModePerm,
			ModTime: time.Date(2012, 1, 2, 23, 0, 0, 0, time.UTC),
		},
		"somedir-we-should-ignore/2012-01-02.jsonlog": &fstest.MapFile{
			Data:    []byte("Hello world!"),
			Mode:    fs.ModePerm,
			ModTime: time.Date(2012, 1, 2, 23, 0, 0, 0, time.UTC),
		},
	}

	now := time.Date(2012, 1, 30, 0, 0, 0, 1, time.UTC)
	thirtyDays := audit.TruncateToDate(now.AddDate(0, 0, -29))

	dirFiles := must.Value(audit.GetDirFiles(must.Value(mapFS.Sub("logdir"))(t).(fs.ReadDirFS)))(t) //nolint:forcetypeassert
	logFiles := audit.FilterLogFiles(dirFiles)
	olderFiles := audit.FilterOlderThan(logFiles, thirtyDays)

	var entries []audit.LogEntry //nolint:prealloc

	for entry, err := range olderFiles {
		require.NoError(t, err)

		entries = append(entries, entry)
	}

	// Here we should see only one file we need to remove
	require.Len(t, entries, 1)
	require.Equal(t, "2011-12-31.jsonlog", entries[0].File.Name())
	require.Equal(t, time.Date(2011, 12, 31, 0, 0, 0, 0, time.UTC), entries[0].Time)
}

func TestActualFS(t *testing.T) {
	formatFile := func(d time.Time) string {
		return d.Format("2006-01-02") + ".jsonlog"
	}

	now := time.Now()

	thirtyDaysOld := "logdir/" + formatFile(now.AddDate(0, 0, -30))

	mapFS := fstest.MapFS{
		thirtyDaysOld: &fstest.MapFile{
			Data:    logFile,
			Mode:    fs.ModePerm,
			ModTime: time.Date(2011, 12, 31, 23, 59, 59, 999, time.UTC),
		},
		"logdir/" + formatFile(now.AddDate(0, 0, -29)): &fstest.MapFile{
			Data:    logFile,
			Mode:    fs.ModePerm,
			ModTime: time.Date(2012, 1, 2, 11, 0, 0, 0, time.UTC),
		},
		"logdir/" + formatFile(now.AddDate(0, 0, -28)): &fstest.MapFile{
			Data:    logFile,
			Mode:    fs.ModePerm,
			ModTime: time.Date(2012, 1, 2, 23, 0, 0, 0, time.UTC),
		},
	}

	tempDir := t.TempDir()

	require.NoError(t, os.CopyFS(tempDir, mapFS))

	require.NoError(
		t,
		audit.NewLogFile(filepath.Join(tempDir, "logdir")).CleanupOldFiles(now.AddDate(0, 0, -29)),
	)

	delete(mapFS, thirtyDaysOld)

	for file := range mapFS {
		_, err := os.Stat(filepath.Join(tempDir, file))

		require.NoError(t, err)
	}
}
