// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset

// ReconcileTearingDown removes all machines from the machine set without any checks.
func ReconcileTearingDown(rc *ReconciliationContext) []Operation {
	operations := make([]Operation, 0, len(rc.GetClusterMachines()))

	for _, id := range rc.GetMachinesToDestroy() {
		operations = append(operations, &Destroy{ID: id})
	}

	for _, id := range rc.GetMachinesToTeardown() {
		operations = append(operations, &Teardown{ID: id})
	}

	return operations
}
