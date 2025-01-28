// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"io"
	"time"
)

func (l *LogFile) DumpAt(data any, at time.Time) error { return l.dumpAt(data, at) }
func (l *LogFile) ReadAuditLog30Days(a time.Time) (io.ReadCloser, error) {
	return l.ReadAuditLog(a.AddDate(0, 0, -29), a)
}

var (
	GetDirFiles    = getDirFiles
	FilterLogFiles = filterLogFiles
	TruncateToDate = truncateToDate
	FilterByTime   = filterByTime
)
