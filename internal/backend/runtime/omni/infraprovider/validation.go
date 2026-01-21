// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package infraprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func validationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *infra.MachineRequest, _ ...state.CreateOption) error {
			var errs error

			if _, err := semver.ParseTolerant(res.TypedSpec().Value.TalosVersion); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("invalid talos version format: %q", res.TypedSpec().Value.TalosVersion))
			}

			return errs
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes *infra.MachineRequest, newRes *infra.MachineRequest, _ ...state.UpdateOption) error {
			if !oldRes.TypedSpec().Value.EqualVT(newRes.TypedSpec().Value) {
				return errors.New("machine request spec is immutable")
			}

			return nil
		})),
	}
}
