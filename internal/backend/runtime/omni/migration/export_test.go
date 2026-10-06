// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

// SetFilter limits the migrations Run executes to the ones the filter accepts.
func (m *Manager) SetFilter(filter func(string) bool) {
	m.filter = filter
}
