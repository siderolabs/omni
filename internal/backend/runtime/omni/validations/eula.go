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

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// eulaValidationOptions returns validation options for the EulaAcceptance resource.
// The EULA can only be accepted once (Create), and never updated or destroyed.
func eulaValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *authres.EulaAcceptance, _ ...state.CreateOption) error {
			existing, err := safe.StateGetByID[*authres.EulaAcceptance](ctx, st, authres.EulaAcceptanceID)
			if err != nil && !state.IsNotFoundError(err) {
				return err
			}

			if existing != nil {
				return fmt.Errorf("EULA has already been accepted")
			}

			if res.Metadata().ID() != authres.EulaAcceptanceID {
				return fmt.Errorf("resource ID must be eula")
			}

			if res.TypedSpec().Value.GetAcceptedByName() == "" {
				return fmt.Errorf("name is required when accepting the EULA")
			}

			if res.TypedSpec().Value.GetAcceptedByEmail() == "" {
				return fmt.Errorf("email is required when accepting the EULA")
			}

			return nil
		})),
	}
}
