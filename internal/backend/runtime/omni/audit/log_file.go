// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/pair/ordered"

	"github.com/siderolabs/omni/internal/pkg/pool"
)

// LogFile is a rotating log file.
//
//nolint:govet
type LogFile struct {
	dir string

	mu        sync.Mutex
	f         *os.File
	lastWrite time.Time

	pool pool.Pool[bytes.Buffer]
}

// NewLogFile creates a new rotating log file.
func NewLogFile(dir string) *LogFile {
	return &LogFile{
		dir: dir,
		pool: pool.Pool[bytes.Buffer]{
			New: func() *bytes.Buffer {
				return &bytes.Buffer{}
			},
		},
	}
}

// Dump writes data to the log file, creating new one on demand.
func (l *LogFile) Dump(data any) error {
	return l.dumpAt(data, time.Time{})
}

func (l *LogFile) dumpAt(data any, at time.Time) error {
	b := l.pool.Get()
	defer func() { b.Reset(); l.pool.Put(b) }()

	err := json.NewEncoder(b).Encode(data)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if at.IsZero() {
		at = time.Now()
	}

	f, err := l.openFile(at)
	if err != nil {
		return err
	}

	_, err = b.WriteTo(f)
	if err != nil {
		return err
	}

	l.lastWrite = at

	return nil
}

// openFile opens a file for the given date. It returns the file is date for at matches
// the last write date. Otherwise, it opens a new file.
func (l *LogFile) openFile(at time.Time) (*os.File, error) {
	if l.f != nil && ordered.MakeTriple(at.Date()).Compare(ordered.MakeTriple(l.lastWrite.Date())) <= 0 {
		return l.f, nil
	}

	if l.f != nil {
		// Ignore the error, we can't do anything about it anyway
		l.f.Close() //nolint:errcheck

		l.f = nil
	}

	logPath := filepath.Join(l.dir, at.Format("2006-01-02")) + ".jsonlog"

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	l.f = f

	return f, nil
}

// RemoveFiles removes log files in the given time range (included). Time range is truncated to date.
func (l *LogFile) RemoveFiles(start, end time.Time) error {
	start, end = truncateToDate(start), truncateToDate(end)

	if end.Before(start) {
		return fmt.Errorf("end time is before start time")
	}

	dirFiles, err := getDirFiles(os.DirFS(l.dir).(fs.ReadDirFS)) //nolint:errcheck
	if err != nil {
		return err
	}

	for file, err := range filterByTime(filterLogFiles(dirFiles), start, end) {
		if err != nil {
			return err
		}

		err = os.Remove(filepath.Join(l.dir, file.File.Name()))
		if err != nil {
			return err
		}
	}

	return nil
}

// ReadAuditLog reads the audit log file by file, oldest to newest within the given time range. The time range
// is inclusive, and truncated to the day.
func (l *LogFile) ReadAuditLog(start, end time.Time) (io.ReadCloser, error) {
	start, end = truncateToDate(start), truncateToDate(end)

	if end.Before(start) {
		return nil, fmt.Errorf("end time is before start time")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	dirFiles, err := getDirFiles(os.DirFS(l.dir).(fs.ReadDirFS)) //nolint:errcheck
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log directory: %w", err)
	}

	logFiles := filterByTime(filterLogFiles(dirFiles), start, end)

	//nolint:prealloc
	var (
		multiCloser multiCloser
		readers     []io.Reader
	)

	for file, err := range logFiles {
		if err != nil {
			multiCloser.Close() //nolint:errcheck

			return nil, err
		}

		rdr, err := os.Open(filepath.Join(l.dir, file.File.Name()))
		if err != nil {
			multiCloser.Close() //nolint:errcheck

			return nil, err
		}

		multiCloser.closers = append(multiCloser.closers, rdr)
		readers = append(readers, rdr)
	}

	return struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(readers...),
		Closer: &multiCloser,
	}, nil
}

type multiCloser struct {
	closers []io.Closer
}

func (m *multiCloser) Close() error {
	var result error

	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result
}
