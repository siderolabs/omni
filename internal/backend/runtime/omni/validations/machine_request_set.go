// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func machineRequestSetValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.MachineRequestSet, _ ...state.CreateOption) error {
			return validateMachineRequestSet(ctx, st, nil, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, oldRes *omni.MachineRequestSet, newRes *omni.MachineRequestSet, _ ...state.UpdateOption) error {
			return validateMachineRequestSet(ctx, st, oldRes, newRes)
		})),
	}
}

func validateMachineRequestSet(ctx context.Context, st state.State, oldRes, res *omni.MachineRequestSet) error {
	if res.TypedSpec().Value.ProviderId == "" {
		return fmt.Errorf("provider id can not be empty")
	}

	if oldRes == nil || oldRes.TypedSpec().Value.ProviderData != res.TypedSpec().Value.ProviderData {
		if err := validateProviderData(ctx, st, res.TypedSpec().Value.ProviderId, res.TypedSpec().Value.ProviderData); err != nil {
			return err
		}
	}

	if err := validateUserStringSlice("kernel args", res.TypedSpec().Value.GetKernelArgs(), MaxKernelArgsCount, MaxKernelArgLength); err != nil {
		return err
	}

	if err := validateTalosVersion(ctx, st, "", res.TypedSpec().Value.TalosVersion); err != nil {
		return err
	}

	return validateExtensions(ctx, st, res.TypedSpec().Value.TalosVersion, res.TypedSpec().Value.GetExtensions())
}
