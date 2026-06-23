// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/dnslabel"
)

func infraProviderValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *infra.Provider, _ ...state.CreateOption) error {
			// The ID becomes the local part of the matching service account identity, so it has
			// to be a DNS-1123 label or PGP identity construction rejects it later.
			if err := dnslabel.Validate(res.Metadata().ID()); err != nil {
				return fmt.Errorf("invalid infra provider name: %w", err)
			}

			return nil
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, ptr resource.Pointer, res *infra.Provider, _ ...state.DestroyOption) error {
			if res == nil {
				return nil
			}

			machines, err := safe.ReaderListAll[*omni.Machine](ctx, st, state.WithLabelQuery(
				resource.LabelEqual(omni.LabelInfraProviderID, res.Metadata().ID()),
			))
			if err != nil {
				return err
			}

			var runningMachines []string

			for machine := range machines.All() {
				if machine.Metadata().Phase() == resource.PhaseRunning {
					runningMachines = append(runningMachines, machine.Metadata().ID())
				}
			}

			if len(runningMachines) > 0 {
				return fmt.Errorf("cannot delete the infra provider %q, as there are %d running machines managed by it: %v", res.Metadata().ID(), len(runningMachines), runningMachines)
			}

			return nil
		})),
	}
}
