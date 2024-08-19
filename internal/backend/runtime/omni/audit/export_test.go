// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

//nolint:gci
import (
	"io/fs"
	"iter"
	"time"
)

func (l *LogFile) DumpAt(data any, at time.Time) error {
	return l.dumpAt(data, at)
}

func GetDirFiles(dir fs.ReadDirFS) (iter.Seq[fs.DirEntry], error) { return getDirFiles(dir) }

func FilterLogFiles(it iter.Seq[fs.DirEntry]) iter.Seq2[LogEntry, error] {
	return filterLogFiles(it)
}

func FilterOlderThan(it iter.Seq2[LogEntry, error], threshold time.Time) iter.Seq2[LogEntry, error] {
	return filterOlderThan(it, threshold)
}

func TruncateToDate(d time.Time) time.Time {
	return truncateToDate(d)
}
