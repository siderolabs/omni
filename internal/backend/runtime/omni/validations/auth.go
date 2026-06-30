// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"unicode"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// accessPolicyValidationOptions returns the validation options for the access policy resource.
func accessPolicyValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.AccessPolicy, _ ...state.CreateOption) error {
			return accesspolicy.Validate(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _ *authres.AccessPolicy, newRes *authres.AccessPolicy, _ ...state.UpdateOption) error {
			return accesspolicy.Validate(newRes)
		})),
	}
}

// roleValidationOptions returns the validation options for the user and public key resources, ensuring that their roles are valid.
func roleValidationOptions() []validated.StateOption {
	validateRole := func(roleStr string) error {
		_, err := role.Parse(roleStr)

		return err
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.User, _ ...state.CreateOption) error {
			return validateRole(res.TypedSpec().Value.GetRole())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _ *authres.User, newRes *authres.User, _ ...state.UpdateOption) error {
			return validateRole(newRes.TypedSpec().Value.GetRole())
		})),
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.PublicKey, _ ...state.CreateOption) error {
			return validateRole(res.TypedSpec().Value.GetRole())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _ *authres.PublicKey, newRes *authres.PublicKey, _ ...state.UpdateOption) error {
			return validateRole(newRes.TypedSpec().Value.GetRole())
		})),
	}
}

func hasUppercaseLetters(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) && unicode.IsLetter(r) {
			return true
		}
	}

	return false
}

func identityValidationOptions(samlConfig config.SAML) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.Identity, _ ...state.CreateOption) error {
			var errs error

			if hasUppercaseLetters(res.Metadata().ID()) {
				errs = multierror.Append(errs, errors.New("must be lowercase"))
			}

			// allow non-email identities for service accounts and for users coming from the SAML provider
			_, isServiceAccount := res.Metadata().Labels().Get(authres.LabelIdentityTypeServiceAccount)
			if samlConfig.GetEnabled() || isServiceAccount {
				return errs
			}

			if _, err := mail.ParseAddress(res.Metadata().ID()); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("not a valid email address: %s", res.Metadata().ID()))
			}

			return errs
		})),
	}
}

func accountLimitsValidationOptions(st state.State, limits config.AuthLimits) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *authres.Identity, _ ...state.CreateOption) error {
			_, isServiceAccount := res.Metadata().Labels().Get(authres.LabelIdentityTypeServiceAccount)

			if isServiceAccount {
				maxServiceAccounts := limits.GetMaxServiceAccounts()
				if maxServiceAccounts == 0 {
					return nil
				}

				existing, err := safe.StateListAll[*authres.Identity](
					ctx, st,
					state.WithLabelQuery(resource.LabelExists(authres.LabelIdentityTypeServiceAccount)),
				)
				if err != nil {
					return fmt.Errorf("failed to list service accounts: %w", err)
				}

				if uint32(existing.Len()) >= maxServiceAccounts {
					return fmt.Errorf("maximum number of service accounts (%d) reached", maxServiceAccounts)
				}

				return nil
			}

			maxUsers := limits.GetMaxUsers()
			if maxUsers == 0 {
				return nil
			}

			existing, err := safe.StateListAll[*authres.Identity](
				ctx, st,
				state.WithLabelQuery(resource.LabelExists(authres.LabelIdentityTypeServiceAccount, resource.NotMatches)),
			)
			if err != nil {
				return fmt.Errorf("failed to list users: %w", err)
			}

			if uint32(existing.Len()) >= maxUsers {
				return fmt.Errorf("maximum number of users (%d) reached", maxUsers)
			}

			return nil
		})),
	}
}

// TODO: maybe move the role validation into roleValidationOptions and create a "matchLabelsValidationOptions" function.
func samlLabelRuleValidationOptions() []validated.StateOption {
	validate := func(res *authres.SAMLLabelRule) error {
		var multiErr error

		if res.TypedSpec().Value.AssignRoleOnRegistration != "" { //nolint:staticcheck
			return fmt.Errorf("assignroleonregistration is deprecated, please use assignrole instead")
		}

		if _, err := role.Parse(res.TypedSpec().Value.AssignRole); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}

		if _, err := labels.ParseSelectors(res.TypedSpec().Value.GetMatchLabels()); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("invalid match labels: %w", err))
		}

		return multiErr
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.SAMLLabelRule, _ ...state.CreateOption) error {
			return validate(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _ *authres.SAMLLabelRule, newRes *authres.SAMLLabelRule, _ ...state.UpdateOption) error {
			return validate(newRes)
		})),
	}
}
