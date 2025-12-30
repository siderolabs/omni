// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package user contains the auth.User and auth.Identity related operations.
package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

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

// Ensure creates the auth.User and auth.Identity resources with the given role if they are not already present.
func Ensure(ctx context.Context, st state.State, email string, role role.Role, updateRole bool) error {
	email = strings.ToLower(email)

	identity, err := safe.StateGet[*auth.Identity](ctx, st, auth.NewIdentity(email).Metadata())
	if err != nil {
		if !state.IsNotFoundError(err) {
			return err
		}

		newUserID := uuid.New().String()

		identity = auth.NewIdentity(email)
		identity.TypedSpec().Value.UserId = newUserID
		identity.Metadata().Labels().Set(auth.LabelIdentityUserID, newUserID)

		err = st.Create(ctx, identity)
		if err != nil {
			return fmt.Errorf("failed to create Identity resource %s: %w", identity.Metadata().ID(), err)
		}
	}

	user := auth.NewUser(identity.TypedSpec().Value.UserId)

	user.TypedSpec().Value.Role = string(role)

	if updateRole {
		return safe.StateModify(ctx, st, user, func(res *auth.User) error {
			res.TypedSpec().Value = user.TypedSpec().Value

			return nil
		})
	}

	err = st.Create(ctx, user)
	if err != nil && !state.IsConflictError(err) {
		return err
	}

	return nil
}
