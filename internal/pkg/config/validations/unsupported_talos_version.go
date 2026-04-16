// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"

	"github.com/siderolabs/omni/client/pkg/constants"
)

// EnsureNoMachinesBelowMinTalosVersion checks that no machines are running Talos versions below MinTalosVersion.
func EnsureNoMachinesBelowMinTalosVersion(ctx context.Context, st state.State) error {
	minVer := semver.MustParse(constants.MinTalosVersion)

	count, err := getMachinesBelowTalosVersion(ctx, st, minVer)
	if err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	return fmt.Errorf("detected %s running unsupported Talos versions (below %s); "+
		"please upgrade the machines",
		pluralize.NewClient().Pluralize("machine", int(count), true), constants.MinTalosVersion)
}
