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

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func nodeForceDestroyRequestValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.NodeForceDestroyRequest, _ ...state.CreateOption) error {
			_, err := safe.StateGetByID[*omni.ClusterMachine](ctx, st, res.Metadata().ID())
			if err != nil {
				if state.IsNotFoundError(err) {
					return fmt.Errorf("cannot create/update a NodeForceDestroyRequest for node %q, as there is no matching cluster machine", res.Metadata().ID())
				}

				return err
			}

			return nil
		})),
	}
}
