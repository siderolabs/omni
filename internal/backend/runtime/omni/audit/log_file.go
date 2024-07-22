// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

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
