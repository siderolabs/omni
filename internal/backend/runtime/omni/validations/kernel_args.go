// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// The caps below are arbitrary, picked well above what real callers would send.
// They bound user input. Bump if needed.
const (
	// MaxKernelArgLength caps the byte length of a single entry in a repeated kernel args list.
	MaxKernelArgLength = 256

	// MaxKernelArgsCount caps the number of entries in a repeated kernel args list.
	MaxKernelArgsCount = 64

	// MaxExtraKernelArgsLength caps the byte length of a free-form kernel cmdline string, used
	// by the InfraMachineConfig extra kernel args and the InstallationMediaConfig kernel args.
	MaxExtraKernelArgsLength = 4096
)

// kernelArgsValidationOptions returns the validation options for the kernel args resource.
func kernelArgsValidationOptions() []validated.StateOption {
	validate := func(res *omni.KernelArgs) error {
		return validateUserStringSlice("args", res.TypedSpec().Value.GetArgs(), MaxKernelArgsCount, MaxKernelArgLength)
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *omni.KernelArgs, _ ...state.CreateOption) error {
			return validate(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _, newRes *omni.KernelArgs, _ ...state.UpdateOption) error {
			return validate(newRes)
		})),
	}
}
