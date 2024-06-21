// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cloudprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func validationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *cloud.MachineRequest, _ ...state.CreateOption) error {
			var errs error

			if _, err := semver.ParseTolerant(res.TypedSpec().Value.TalosVersion); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("invalid talos version format: %q", res.TypedSpec().Value.TalosVersion))
			}

			if !isSHA256Hex(res.TypedSpec().Value.SchematicId) {
				errs = multierror.Append(errs, fmt.Errorf("invalid schematic ID format: %q", res.TypedSpec().Value.SchematicId))
			}

			return errs
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes *cloud.MachineRequest, newRes *cloud.MachineRequest, _ ...state.UpdateOption) error {
			if !oldRes.TypedSpec().Value.EqualVT(newRes.TypedSpec().Value) {
				return errors.New("machine request spec is immutable")
			}

			return nil
		})),
	}
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}

	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}
