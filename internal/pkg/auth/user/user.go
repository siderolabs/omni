// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package user contains the auth.User and auth.Identity related operations.
package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// ErrIsServiceAccount is returned when an operation is attempted on a service account identity.
var ErrIsServiceAccount = errors.New("identity is a service account, not a user")

// EnsureInitialResources creates the auth.User and auth.Identity resources if they are not present.
func EnsureInitialResources(ctx context.Context, st state.State, logger *zap.Logger, initialUsers []string) error {
	items, err := st.List(ctx, auth.NewUser("").Metadata())
	if err != nil {
		return err
	}

	if len(items.Items) > 0 {
		logger.Info("some users already exist, skipping initial users")

		return nil
	}

	var multiErr error

	for _, email := range initialUsers {
		if err := Ensure(ctx, st, email, role.Admin, false); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	return multiErr
}

// Create creates a new user with the given email and role.
// It returns the user ID. It fails if a user with the given email already exists.
func Create(ctx context.Context, st state.State, email, userRole string) (string, error) {
	email = strings.ToLower(email)

	parsedRole, err := role.Parse(userRole)
	if err != nil {
		return "", err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	_, err = st.Get(ctx, auth.NewIdentity(email).Metadata())
	if err == nil {
		return "", fmt.Errorf("user with email %q already exists", email)
	}

	if !state.IsNotFoundError(err) {
		return "", err
	}

	return createIdentityAndUser(ctx, st, email, parsedRole)
}

// Update updates the role of the user with the given email.
func Update(ctx context.Context, st state.State, email, userRole string) error {
	email = strings.ToLower(email)

	parsedRole, err := role.Parse(userRole)
	if err != nil {
		return err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	identity, err := safe.StateGet[*auth.Identity](ctx, st, auth.NewIdentity(email).Metadata())
	if err != nil {
		return err
	}

	_, err = safe.StateUpdateWithConflicts(ctx, st,
		auth.NewUser(identity.TypedSpec().Value.UserId).Metadata(),
		func(user *auth.User) error {
			user.TypedSpec().Value.Role = string(parsedRole)

			return nil
		},
	)

	return err
}

// Destroy destroys the user with the given email, cleaning up Identity and User resources.
func Destroy(ctx context.Context, st state.State, email string) error {
	email = strings.ToLower(email)

	ctx = actor.MarkContextAsInternalActor(ctx)

	identity, err := safe.StateGet[*auth.Identity](ctx, st, auth.NewIdentity(email).Metadata())
	if err != nil {
		return err
	}

	// Ensure this is not a service account.
	_, isServiceAccount := identity.Metadata().Labels().Get(auth.LabelIdentityTypeServiceAccount)
	if isServiceAccount {
		return fmt.Errorf("identity %q: %w", email, ErrIsServiceAccount)
	}

	// Destroy public keys associated with the user.
	pubKeys, err := st.List(
		ctx,
		auth.NewPublicKey("").Metadata(),
		state.WithLabelQuery(resource.LabelEqual(auth.LabelPublicKeyUserID, identity.TypedSpec().Value.UserId)),
	)
	if err != nil {
		return err
	}

	var destroyErr error

	for _, pubKey := range pubKeys.Items {
		if err = st.TeardownAndDestroy(ctx, pubKey.Metadata()); err != nil && !state.IsNotFoundError(err) {
			destroyErr = multierror.Append(destroyErr, err)
		}
	}

	if err = st.TeardownAndDestroy(ctx, identity.Metadata()); err != nil && !state.IsNotFoundError(err) {
		destroyErr = multierror.Append(destroyErr, err)
	}

	if err = st.TeardownAndDestroy(ctx, auth.NewUser(identity.TypedSpec().Value.UserId).Metadata()); err != nil && !state.IsNotFoundError(err) {
		destroyErr = multierror.Append(destroyErr, err)
	}

	return destroyErr
}

// Ensure creates the auth.User and auth.Identity resources with the given role if they are not already present.
// If the user already exists and updateRole is true, it updates the role.
func Ensure(ctx context.Context, st state.State, email string, r role.Role, updateRole bool) error {
	email = strings.ToLower(email)

	identity, err := safe.StateGet[*auth.Identity](ctx, st, auth.NewIdentity(email).Metadata())
	if err != nil {
		if !state.IsNotFoundError(err) {
			return err
		}

		// User does not exist, create it.
		_, createErr := createIdentityAndUser(ctx, st, email, r)

		return createErr
	}

	// User already exists.
	if !updateRole {
		return nil
	}

	return safe.StateModify(ctx, st, auth.NewUser(identity.TypedSpec().Value.UserId), func(res *auth.User) error {
		res.TypedSpec().Value.Role = string(r)

		return nil
	})
}

// createIdentityAndUser creates the Identity and User resources for a new user.
// It cleans up the Identity if User creation fails.
func createIdentityAndUser(ctx context.Context, st state.State, email string, r role.Role) (string, error) {
	newUserID := uuid.NewString()

	identity := auth.NewIdentity(email)
	identity.TypedSpec().Value.UserId = newUserID
	identity.Metadata().Labels().Set(auth.LabelIdentityUserID, newUserID)

	if err := st.Create(ctx, identity); err != nil {
		return "", fmt.Errorf("failed to create Identity resource %s: %w", identity.Metadata().ID(), err)
	}

	user := auth.NewUser(newUserID)
	user.TypedSpec().Value.Role = string(r)

	if err := st.Create(ctx, user); err != nil {
		_ = st.Destroy(ctx, identity.Metadata()) //nolint:errcheck // best-effort cleanup

		return "", err
	}

	if err := st.Create(ctx, auth.NewIdentityLastActive(email)); err != nil {
		return "", err
	}

	return newUserID, nil
}
