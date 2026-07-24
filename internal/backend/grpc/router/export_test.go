// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import "github.com/siderolabs/omni/client/pkg/omni/resources/omni"

// ResolveMachineConnection exposes resolveMachineConnection for tests.
func ResolveMachineConnection(machineID, requestedCluster string, machineStatus *omni.MachineStatus) (string, error) {
	return resolveMachineConnection(machineID, requestedCluster, machineStatus)
}
