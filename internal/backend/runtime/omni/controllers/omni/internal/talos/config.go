// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// HasCompleteConfig reports whether the machine has an active config which is complete for boot. Talos leaves maintenance mode with such a config.
func HasCompleteConfig(ctx context.Context, st state.State) (bool, error) {
	active, err := safe.ReaderGetByID[*configres.MachineConfig](ctx, st, configres.ActiveID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get the active config: %w", err)
	}

	return active.Provider().CompleteForBoot(), nil
}
