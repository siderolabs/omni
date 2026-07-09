// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

func ComputeHighPriorityConfigChanges(oldConfig, newConfig []byte) (bool, error) {
	return computeHighPriorityConfigChanges(oldConfig, newConfig)
}
