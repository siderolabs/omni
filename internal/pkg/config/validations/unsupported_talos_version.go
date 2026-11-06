// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// EnsureNoMachinesBelowTalosVersionFloor checks that no machines are running Talos versions below constants.TalosVersionFloor.
func EnsureNoMachinesBelowTalosVersionFloor(ctx context.Context, st state.State) error {
	statuses, err := safe.ReaderListAll[*omni.MachineStatus](ctx, st)
	if err != nil {
		return err
	}

	var count int

	for status := range statuses.All() {
		if siderolink.IsBelowTalosVersionFloor(status.TypedSpec().Value.TalosVersion) {
			count++
		}
	}

	if count == 0 {
		return nil
	}

	return fmt.Errorf("detected %s running Talos versions below %s, which can no longer connect to Omni: "+
		"upgrade the machines, or set --siderolink-disconnect-talos-below-1-6 (services.siderolink.disconnectTalosBelow1_6) "+
		"to start Omni and disconnect them "+
		"(https://docs.siderolabs.com/omni/getting-started/talos-version-support-policy)",
		pluralize.NewClient().Pluralize("machine", count, true), constants.TalosVersionFloor)
}
