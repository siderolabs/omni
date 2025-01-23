// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"io/fs"
	"iter"
	"path/filepath"
	"time"
)

func getDirFiles(dir fs.ReadDirFS) (iter.Seq[fs.DirEntry], error) {
	result, err := dir.ReadDir(".")
	if err != nil {
		return nil, err
	}

	return func(yield func(entry fs.DirEntry) bool) {
		for _, file := range result {
			if file.IsDir() || !file.Type().IsRegular() {
				continue
			}

			if !yield(file) {
				return
			}
		}
	}, nil
}

func filterLogFiles(it iter.Seq[fs.DirEntry]) iter.Seq2[LogEntry, error] {
	return func(yield func(LogEntry, error) bool) {
		for file := range it {
			if filepath.Ext(file.Name()) != ".jsonlog" {
				continue
			}

			name := file.Name()
			name = name[:len(name)-len(".jsonlog")]

			parsedName, err := time.ParseInLocation(time.DateOnly, name, time.Local) //nolint:gosmopolitan
			if err != nil {
				if !yield(LogEntry{}, err) {
					return
				}

				continue
			}

			if !yield(LogEntry{File: file, Time: parsedName}, nil) {
				return
			}
		}
	}
}

// LogEntry represents a log entry file with file data and parsed time.
type LogEntry struct {
	File fs.DirEntry
	Time time.Time
}

func filterByTime(it iter.Seq2[LogEntry, error], start, end time.Time) iter.Seq2[LogEntry, error] {
	return func(yield func(LogEntry, error) bool) {
		for entry, err := range it {
			if err != nil {
				if !yield(LogEntry{}, err) {
					return
				}

				continue
			}

			if (entry.Time.After(start) || entry.Time.Equal(start)) && (entry.Time.Before(end) || entry.Time.Equal(end)) {
				if !yield(entry, nil) {
					return
				}
			}
		}
	}
}

func truncateToDate(d time.Time) time.Time {
	year, month, day := d.Date()

	return time.Date(year, month, day, 0, 0, 0, 0, d.Location())
}
