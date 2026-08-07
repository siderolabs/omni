// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// machineInstallDiskConfigValidationOptions returns the validation options for the machine install disk config resource.
func machineInstallDiskConfigValidationOptions() []validated.StateOption {
	validate := func(res *omni.MachineInstallDiskConfig) error {
		disk := res.TypedSpec().Value.Disk
		selector := res.TypedSpec().Value.DiskSelector

		switch {
		case disk != "" && selector != "":
			return errors.New("disk and disk selector are mutually exclusive")
		case disk == "" && selector == "":
			return errors.New("either disk or disk selector must be set")
		}

		if selector != "" {
			if _, err := cel.ParseBooleanExpression(selector, celenv.DiskLocator()); err != nil {
				return fmt.Errorf("disk selector is not a valid boolean disk expression: %w", err)
			}
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *omni.MachineInstallDiskConfig, _ ...state.CreateOption) error {
			return validate(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _, newRes *omni.MachineInstallDiskConfig, _ ...state.UpdateOption) error {
			return validate(newRes)
		})),
	}
}
