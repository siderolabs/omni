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

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func rotateSecretsValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(
			func(ctx context.Context, _ *omni.RotateTalosCA, newRes *omni.RotateTalosCA, _ ...state.UpdateOption) error {
				return validateRotateSecretModify(ctx, st, newRes)
			},
		)),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(
			func(ctx context.Context, _ *omni.RotateKubernetesCA, newRes *omni.RotateKubernetesCA, _ ...state.UpdateOption) error {
				return validateRotateSecretModify(ctx, st, newRes)
			},
		)),

		validated.WithDestroyValidations(validated.NewDestroyValidationForType(
			func(ctx context.Context, ptr resource.Pointer, existingRes *omni.RotateTalosCA, _ ...state.DestroyOption) error {
				return validateRotateSecretDestroy(ctx, st, existingRes)
			},
		)),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(
			func(ctx context.Context, ptr resource.Pointer, existingRes *omni.RotateKubernetesCA, _ ...state.DestroyOption) error {
				return validateRotateSecretDestroy(ctx, st, existingRes)
			},
		)),
	}
}

func validateRotateSecretModify(ctx context.Context, st state.State, res resource.Resource) error {
	if res.Metadata().Phase() == resource.PhaseTearingDown {
		return nil
	}

	rotationStatus, err := safe.ReaderGetByID[*omni.ClusterSecretsRotationStatus](ctx, st, res.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if rotationStatus.TypedSpec().Value.Phase != specs.SecretRotationSpec_OK {
		return fmt.Errorf("cannot modify the %s %q while a secret rotation is in progress", res.Metadata().Type(), res.Metadata().ID())
	}

	return nil
}

func validateRotateSecretDestroy(ctx context.Context, st state.State, res resource.Resource) error {
	if res == nil {
		return nil
	}

	rotationStatus, err := safe.ReaderGetByID[*omni.ClusterSecretsRotationStatus](ctx, st, res.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if rotationStatus.TypedSpec().Value.Phase != specs.SecretRotationSpec_OK {
		return fmt.Errorf("cannot delete the %s %q while a secret rotation is in progress", res.Metadata().Type(), res.Metadata().ID())
	}

	return nil
}
