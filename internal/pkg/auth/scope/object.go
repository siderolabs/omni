// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package scope

// Object represents Scope target.
type Object string

// Object constants.
const (
	// Cluster and everything below it.
	ObjectCluster = "cluster"
	// Machines.
	ObjectMachine = "machine"
	// Users.
	ObjectUser = "user"
	// Service accounts.
	ObjectServiceAccount = "service-account"
)
