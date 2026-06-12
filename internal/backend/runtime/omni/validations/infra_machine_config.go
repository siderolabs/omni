// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// MaxRequestIDLength caps the byte length of the request ID fields on InfraMachineConfig.
const MaxRequestIDLength = 128

func infraMachineConfigValidationOptions(st state.State) []validated.StateOption {
	validateSpec := func(res *omni.InfraMachineConfig) error {
		if err := validateUserString("requested reboot ID", res.TypedSpec().Value.GetRequestedRebootId(), MaxRequestIDLength); err != nil {
			return err
		}

		if err := validateUserString("power-off request ID", res.TypedSpec().Value.GetPowerOffRequestId(), MaxRequestIDLength); err != nil {
			return err
		}

		if err := validateUserString("extra kernel args", res.TypedSpec().Value.GetExtraKernelArgs(), MaxExtraKernelArgsLength); err != nil {
			return err
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *omni.InfraMachineConfig, _ ...state.CreateOption) error {
			return validateSpec(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes, newRes *omni.InfraMachineConfig, _ ...state.UpdateOption) error {
			if oldRes.TypedSpec().Value.AcceptanceStatus == specs.InfraMachineConfigSpec_ACCEPTED &&
				newRes.TypedSpec().Value.AcceptanceStatus != oldRes.TypedSpec().Value.AcceptanceStatus {
				return errors.New("an accepted machine cannot be rejected or set back to pending acceptance")
			}

			return validateSpec(newRes)
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, _ resource.Pointer, res *omni.InfraMachineConfig, _ ...state.DestroyOption) error {
			if res.TypedSpec().Value.AcceptanceStatus != specs.InfraMachineConfigSpec_ACCEPTED {
				return nil
			}

			if _, err := safe.StateGetByID[*siderolink.Link](ctx, st, res.Metadata().ID()); err != nil {
				if state.IsNotFoundError(err) {
					return nil
				}

				return err
			}

			return errors.New("cannot delete the config for an already accepted machine config while it is linked to a machine")
		})),
	}
}
