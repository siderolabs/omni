// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import "time"

func (l *LogFile) DumpAt(data any, at time.Time) error {
	return l.dumpAt(data, at)
}
