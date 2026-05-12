// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

const (
	// MaxJoinTokenNameLength is the maximum length of the join token name.
	// tsgen:MaxJoinTokenNameLength
	MaxJoinTokenNameLength = 16
)

func joinTokenValidationOptions(st state.State) []validated.StateOption {
	validateJoinTokenName := func(res *siderolink.JoinToken) error {
		if res.TypedSpec().Value.Name == "" {
			return errors.New("the join token name cannot be empty")
		}

		if len(res.TypedSpec().Value.Name) > MaxJoinTokenNameLength {
			return fmt.Errorf("join token name cannot be longer than %d symbols", MaxJoinTokenNameLength)
		}

		return nil
	}

	checkDefault := func(ctx context.Context, id string) (bool, error) {
		defaultJoinToken, err := safe.ReaderGetByID[*siderolink.DefaultJoinToken](ctx, st, siderolink.DefaultJoinTokenID)
		if err != nil && !state.IsNotFoundError(err) {
			return false, err
		}

		if defaultJoinToken == nil {
			return false, nil
		}

		return defaultJoinToken.TypedSpec().Value.TokenId == id, nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *siderolink.JoinToken, _ ...state.CreateOption) error {
			return validateJoinTokenName(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, old, res *siderolink.JoinToken, _ ...state.UpdateOption) error {
			if old.TypedSpec().Value.Name == res.TypedSpec().Value.Name {
				return nil
			}

			return validateJoinTokenName(res)
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(
			func(ctx context.Context, _ resource.Pointer, res *siderolink.JoinToken, _ ...state.DestroyOption) error {
				isDefault, err := checkDefault(ctx, res.Metadata().ID())
				if err != nil {
					return err
				}

				if isDefault {
					return fmt.Errorf("deleting default join token is not possible")
				}

				return nil
			},
		)),
	}
}

func defaultJoinTokenValidationOptions(st state.State) []validated.StateOption {
	validateToken := func(ctx context.Context, id string) error {
		_, err := safe.ReaderGetByID[*siderolink.JoinToken](ctx, st, id)
		if err != nil {
			if state.IsNotFoundError(err) {
				return fmt.Errorf("no token with id %q exists", id)
			}

			return err
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, _, res *siderolink.DefaultJoinToken, _ ...state.UpdateOption) error {
			if err := validateToken(ctx, res.TypedSpec().Value.TokenId); err != nil {
				return err
			}

			if res.Metadata().Phase() == resource.PhaseTearingDown {
				if res.Metadata().ID() != siderolink.DefaultJoinTokenID {
					return nil
				}

				return errors.New("destroying the default join token resource is not allowed")
			}

			return nil
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(
			func(ctx context.Context, _ resource.Pointer, res *siderolink.DefaultJoinToken, _ ...state.DestroyOption) error {
				if err := validateToken(ctx, res.TypedSpec().Value.TokenId); err != nil {
					return err
				}

				if res.Metadata().ID() != siderolink.DefaultJoinTokenID {
					return nil
				}

				return errors.New("destroying the default join token resource is not allowed")
			},
		)),
	}
}
