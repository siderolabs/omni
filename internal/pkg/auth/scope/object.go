// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package scope

// Object represents Scope target.
type Object string

// Object constants.
const (
	// ObjectCluster holds Cluster and everything below it.
	ObjectCluster = "cluster"
	// ObjectMachine is the constant for Machines.
	ObjectMachine = "machine"
	// ObjectUser is the constant for Users.
	ObjectUser = "user"
	// ObjectServiceAccount is the constant for Service accounts.
	ObjectServiceAccount = "service-account"
)
