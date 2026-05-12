// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func exposedServiceValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *omni.ExposedService, _ ...state.CreateOption) error {
			alias, _ := res.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
			if alias == "" {
				return errors.New("alias must be set")
			}

			return nil
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, res *omni.ExposedService, newRes *omni.ExposedService, _ ...state.UpdateOption) error {
			oldAlias, _ := res.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
			newAlias, _ := newRes.Metadata().Labels().Get(omni.LabelExposedServiceAlias)

			if oldAlias != newAlias {
				return errors.New("alias cannot be changed")
			}

			return nil
		})),
	}
}
