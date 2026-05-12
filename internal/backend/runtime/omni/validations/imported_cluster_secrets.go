// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func importedClusterSecretValidationOptions(st state.State, clusterImportEnabled bool) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.ImportedClusterSecrets, _ ...state.CreateOption) error {
			if !clusterImportEnabled {
				return errors.New("cluster import feature is not enabled")
			}

			return validateImportedClusterSecrets(ctx, st, res)
		})),
	}
}

func validateImportedClusterSecrets(ctx context.Context, st state.State, res *omni.ImportedClusterSecrets) error {
	_, err := safe.StateGetByID[*omni.Cluster](ctx, st, res.Metadata().ID())
	if err != nil {
		if !state.IsNotFoundError(err) {
			return err
		}
	} else {
		return fmt.Errorf("cannot create an ImportedClusterSecrets, as there is already an existing cluster with name: %q", res.Metadata().ID())
	}

	bundle, err := omni.FromImportedSecretsToSecretsBundle(res)
	if err != nil {
		return fmt.Errorf("failed to unmarshal imported cluster secret: %w", err)
	}

	err = bundle.Validate()
	if err != nil {
		return fmt.Errorf("failed to validate imported cluster secret: %w", err)
	}

	return nil
}
