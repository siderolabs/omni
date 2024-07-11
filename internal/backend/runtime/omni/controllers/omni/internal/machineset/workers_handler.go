// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset

// ReconcileWorkers gets the reconciliation context and produces the list of changes to apply on the machine set.
func ReconcileWorkers(rc *ReconciliationContext) []Operation {
	quota := rc.CalculateQuota()

	toCreate := rc.GetMachinesToCreate()
	toTeardown := rc.GetMachinesToTeardown()
	toUpdate := rc.GetMachinesToUpdate()
	toDestroy := rc.GetMachinesToDestroy()

	operations := make([]Operation, 0, len(toCreate)+len(toTeardown)+len(toUpdate)+len(toDestroy))

	for _, id := range toDestroy {
		operations = append(operations, &Destroy{ID: id})
	}

	for _, id := range toCreate {
		operations = append(operations, &Create{ID: id})
	}

	for _, id := range toTeardown {
		operations = append(operations, &Teardown{ID: id, Quota: &quota})
	}

	for _, id := range toUpdate {
		operations = append(operations, &Update{ID: id, Quota: &quota})
	}

	return operations
}
