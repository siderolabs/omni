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

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// EnsureNoNonImageFactoryMachines checks that no machines have an invalid schematic (provisioned without ImageFactory).
func EnsureNoNonImageFactoryMachines(ctx context.Context, st state.State) error {
	statuses, err := safe.ReaderListAll[*omni.MachineStatus](ctx, st)
	if err != nil {
		return err
	}

	var count int

	for status := range statuses.All() {
		if status.TypedSpec().Value.SchematicReady() && status.TypedSpec().Value.GetSchematic().GetInvalid() {
			count++
		}
	}

	if count == 0 {
		return nil
	}

	return fmt.Errorf("detected %s provisioned without ImageFactory; "+
		"please re-provision them using ImageFactory",
		pluralize.NewClient().Pluralize("machine", count, true))
}
