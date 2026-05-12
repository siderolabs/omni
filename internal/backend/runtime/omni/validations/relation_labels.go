// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// relationLabelsValidationOptions returns the validation options for the relation labels on the resources.
func relationLabelsValidationOptions() []validated.StateOption {
	validateLabelIsSet := func(res resource.Resource, key string) error {
		val, ok := res.Metadata().Labels().Get(key)
		if !ok {
			return fmt.Errorf("label %q does not exist", key)
		}

		if val == "" {
			return fmt.Errorf("label %q has empty value", key)
		}

		return nil
	}

	validateLabelIsNotChanged := func(oldRes resource.Resource, newRes resource.Resource, key string) error {
		oldVal, _ := oldRes.Metadata().Labels().Get(key)
		val, _ := newRes.Metadata().Labels().Get(key)

		if oldVal != "" && oldVal != val {
			return fmt.Errorf("changing value of label %q from %q to %q", key, oldVal, val)
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.MachineSetNode, _ ...state.CreateOption) error {
				return validateLabelIsSet(res, omni.LabelCluster)
			}),
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.MachineSetNode, _ ...state.CreateOption) error {
				return validateLabelIsSet(res, omni.LabelMachineSet)
			}),
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.MachineSet, _ ...state.CreateOption) error {
				return validateLabelIsSet(res, omni.LabelCluster)
			}),
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.ExposedService, _ ...state.CreateOption) error {
				return validateLabelIsSet(res, omni.LabelCluster)
			}),
		),
		validated.WithUpdateValidations(
			validated.NewUpdateValidationForType(func(_ context.Context, _ *omni.MachineSetNode, newRes *omni.MachineSetNode, _ ...state.UpdateOption) error {
				return validateLabelIsSet(newRes, omni.LabelCluster)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, _ *omni.MachineSetNode, newRes *omni.MachineSetNode, _ ...state.UpdateOption) error {
				return validateLabelIsSet(newRes, omni.LabelMachineSet)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, _ *omni.MachineSet, newRes *omni.MachineSet, _ ...state.UpdateOption) error {
				return validateLabelIsSet(newRes, omni.LabelCluster)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, _ *omni.ExposedService, newRes *omni.ExposedService, _ ...state.UpdateOption) error {
				return validateLabelIsSet(newRes, omni.LabelCluster)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.MachineSetNode, newRes *omni.MachineSetNode, _ ...state.UpdateOption) error {
				return validateLabelIsNotChanged(oldRes, newRes, omni.LabelCluster)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.MachineSetNode, newRes *omni.MachineSetNode, _ ...state.UpdateOption) error {
				return validateLabelIsNotChanged(oldRes, newRes, omni.LabelMachineSet)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.MachineSet, newRes *omni.MachineSet, _ ...state.UpdateOption) error {
				return validateLabelIsNotChanged(oldRes, newRes, omni.LabelCluster)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.ExposedService, newRes *omni.ExposedService, _ ...state.UpdateOption) error {
				return validateLabelIsNotChanged(oldRes, newRes, omni.LabelCluster)
			}),
		),
	}
}
