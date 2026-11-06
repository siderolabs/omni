// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// EnsureNoMachinesBelowTalosVersionFloor checks that no machines are running Talos versions below constants.TalosVersionFloor.
//
// The machines below agreedFloor are not counted, as the operator already agreed to disconnect them.
// An empty agreedFloor counts all machines below constants.TalosVersionFloor.
func EnsureNoMachinesBelowTalosVersionFloor(ctx context.Context, st state.State, agreedFloor string) error {
	floor := semver.MustParse(constants.TalosVersionFloor)

	var lowerBound semver.Version

	if agreedFloor != "" {
		var err error

		if lowerBound, err = semver.Parse(agreedFloor); err != nil {
			return fmt.Errorf("failed to parse the agreed Talos version floor: %w", err)
		}
	}

	statuses, err := safe.ReaderListAll[*omni.MachineStatus](ctx, st)
	if err != nil {
		return err
	}

	var count int

	for status := range statuses.All() {
		version, err := siderolink.ParseTalosVersion(status.TypedSpec().Value.TalosVersion)
		if err != nil {
			continue // the machine did not report its version yet
		}

		if version.GTE(lowerBound) && version.LT(floor) {
			count++
		}
	}

	if count == 0 {
		return nil
	}

	versions := "below " + constants.TalosVersionFloor
	if agreedFloor != "" {
		versions = fmt.Sprintf("from %s to below %s", agreedFloor, constants.TalosVersionFloor)
	}

	return fmt.Errorf("detected %s running Talos versions %s, which can no longer connect to Omni: "+
		"upgrade the machines, or set --siderolink-disconnect-talos-below=%s (services.siderolink.disconnectTalosBelow) "+
		"to start Omni and disconnect them "+
		"(https://docs.siderolabs.com/omni/getting-started/talos-version-support-policy)",
		pluralize.NewClient().Pluralize("machine", count, true), versions, constants.TalosVersionFloor)
}
