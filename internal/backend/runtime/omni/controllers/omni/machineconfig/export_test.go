// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// UpgradeBlockedByOwnHealthForTest exposes upgradeBlockedByOwnHealth to external tests.
func UpgradeBlockedByOwnHealthForTest(snapshot *omni.MachineStatusSnapshot) bool {
	rc := &ReconciliationContext{machineStatusSnapshot: snapshot}

	return rc.upgradeBlockedByOwnHealth()
}
