// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auditlogfile

import (
	"time"
)

func (l *LogFile) DumpAt(data any, at time.Time) error { return l.dumpAt(data, at) }

var (
	GetDirFiles    = getDirFiles
	FilterLogFiles = filterLogFiles
	TruncateToDate = truncateToDate
	FilterByTime   = filterByTime
)
