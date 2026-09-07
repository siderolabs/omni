// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tokenfile

import "time"

// SetPollInterval sets how often the file is re-read regardless of notifications.
func (t *Token) SetPollInterval(interval time.Duration) {
	t.pollInterval = interval
}

// Reload re-reads the file, the way Run does on an event or a tick.
func (t *Token) Reload() error {
	return t.reload()
}
