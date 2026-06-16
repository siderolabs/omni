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

func extensionsConfigurationValidationOptions(st state.State) []validated.StateOption {
	validate := func(ctx context.Context, res *omni.ExtensionsConfiguration) error {
		extensions := res.TypedSpec().Value.GetExtensions()
		if len(extensions) == 0 {
			return nil
		}

		clusterID, ok := res.Metadata().Labels().Get(omni.LabelCluster)
		if !ok || clusterID == "" {
			return errors.New("extensions configuration with a non-empty extensions list must target a cluster via the cluster label")
		}

		cluster, err := safe.StateGet[*omni.Cluster](ctx, st, omni.NewCluster(clusterID).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return fmt.Errorf("cluster %q does not exist", clusterID)
			}

			return fmt.Errorf("failed to look up cluster %q: %w", clusterID, err)
		}

		return validateExtensions(ctx, st, cluster.TypedSpec().Value.GetTalosVersion(), extensions)
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.ExtensionsConfiguration, _ ...state.CreateOption) error {
			return validate(ctx, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, _, newRes *omni.ExtensionsConfiguration, _ ...state.UpdateOption) error {
			return validate(ctx, newRes)
		})),
	}
}
