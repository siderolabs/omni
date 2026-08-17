// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package user_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/user"
)

func assertUsers(ctx context.Context, t *testing.T, st state.State, expectedUsers []string) {
	var userIDs []string

	rtestutils.AssertResources(ctx, t, st, expectedUsers, func(identity *auth.Identity, assert *assert.Assertions) {
		userIDs = append(userIDs, identity.TypedSpec().Value.UserId)

		label, ok := identity.Metadata().Labels().Get(auth.LabelIdentityUserID)

		assert.True(ok)
		assert.Equal(identity.TypedSpec().Value.UserId, label)
	})

	assert.Len(t, userIDs, len(expectedUsers))

	rtestutils.AssertResources(ctx, t, st, userIDs, func(*auth.User, *assert.Assertions) {})
}

func TestInitialUsers(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)
	ctx := t.Context()

	const (
		john    = "john@example.com"
		richard = "richard@example.com"
		timothy = "timothy@example.com"
	)

	require.NoError(t, user.EnsureInitialResources(
		ctx, st, logger,
		[]string{
			john,
			richard,
		},
	))

	assertUsers(ctx, t, st, []string{john, richard})

	require.NoError(t, user.EnsureInitialResources(
		ctx, st, logger,
		[]string{
			richard,
			john,
		},
	))

	assertUsers(ctx, t, st, []string{john, richard})

	require.NoError(t, user.EnsureInitialResources(
		ctx, st, logger,
		[]string{
			richard,
			timothy,
		},
	))

	// new user doesn't get created, as the state is already initialized
	assertUsers(ctx, t, st, []string{john, richard})
}

// roleOf reads the role of the auth.User behind the given email.
func roleOf(ctx context.Context, t *testing.T, st state.State, email string) string {
	identity, err := safe.StateGetByID[*auth.Identity](ctx, st, email)
	require.NoError(t, err)

	usr, err := safe.StateGetByID[*auth.User](ctx, st, identity.TypedSpec().Value.UserId)
	require.NoError(t, err)

	return usr.TypedSpec().Value.Role
}

func TestElevateRecoveryAdmin(t *testing.T) {
	t.Parallel()

	const (
		locked      = "locked-out@example.com"
		untouched   = "untouched@example.com"
		absent      = "absent@example.com"
		automation  = "automation" + access.ServiceAccountNameSuffix
		roleAdmin   = string(role.Admin)
		roleReader  = string(role.Reader)
		roleNone    = string(role.None)
		roleUnknown = "Nonsense"
	)

	setup := func(t *testing.T) (context.Context, state.State, *zap.Logger) {
		t.Helper()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		ctx := t.Context()

		require.NoError(t, user.Ensure(ctx, st, locked, role.None, false))
		require.NoError(t, user.Ensure(ctx, st, untouched, role.Reader, false))

		return ctx, st, zaptest.NewLogger(t)
	}

	t.Run("raises the configured user and leaves the rest alone", func(t *testing.T) {
		t.Parallel()

		ctx, st, logger := setup(t)

		require.NoError(t, user.ElevateRecoveryAdmin(ctx, st, logger, locked))

		assert.Equal(t, roleAdmin, roleOf(ctx, t, st, locked))
		assert.Equal(t, roleReader, roleOf(ctx, t, st, untouched))
	})

	t.Run("stays quiet when the user is already Admin", func(t *testing.T) {
		t.Parallel()

		ctx, st, _ := setup(t)

		require.NoError(t, user.Update(ctx, st, locked, roleAdmin))

		identity, err := safe.StateGetByID[*auth.Identity](ctx, st, locked)
		require.NoError(t, err)

		before, err := safe.StateGetByID[*auth.User](ctx, st, identity.TypedSpec().Value.UserId)
		require.NoError(t, err)

		core, logs := observer.New(zapcore.InfoLevel)

		require.NoError(t, user.ElevateRecoveryAdmin(ctx, st, zap.New(core), locked))

		after, err := safe.StateGetByID[*auth.User](ctx, st, identity.TypedSpec().Value.UserId)
		require.NoError(t, err)

		assert.Equal(t, roleAdmin, after.TypedSpec().Value.Role)

		// an unchanged version means COSI suppressed the write, and with it the audit entry
		assert.True(t, before.Metadata().Version().Equal(after.Metadata().Version()),
			"expected no write, version went from %s to %s", before.Metadata().Version(), after.Metadata().Version())

		assert.Empty(t, logs.FilterMessageSnippet("elevated recovery admin").All(),
			"a restart that changed nothing should not announce an elevation")
	})

	t.Run("announces a real elevation", func(t *testing.T) {
		t.Parallel()

		ctx, st, _ := setup(t)

		core, logs := observer.New(zapcore.InfoLevel)

		require.NoError(t, user.ElevateRecoveryAdmin(ctx, st, zap.New(core), locked))

		entries := logs.FilterMessageSnippet("elevated recovery admin").All()
		require.Len(t, entries, 1)
		assert.Equal(t, roleNone, entries[0].ContextMap()["previous_role"])
	})

	t.Run("matches the email case-insensitively", func(t *testing.T) {
		t.Parallel()

		ctx, st, logger := setup(t)

		require.NoError(t, user.ElevateRecoveryAdmin(ctx, st, logger, "Locked-Out@Example.com"))

		assert.Equal(t, roleAdmin, roleOf(ctx, t, st, locked))
	})

	t.Run("is a no-op when no email is configured", func(t *testing.T) {
		t.Parallel()

		ctx, st, logger := setup(t)

		require.NoError(t, user.ElevateRecoveryAdmin(ctx, st, logger, ""))

		assert.Equal(t, roleNone, roleOf(ctx, t, st, locked))
	})

	t.Run("does not create a user that does not exist", func(t *testing.T) {
		t.Parallel()

		ctx, st, logger := setup(t)

		require.NoError(t, user.ElevateRecoveryAdmin(ctx, st, logger, absent))

		_, err := safe.StateGetByID[*auth.Identity](ctx, st, absent)
		assert.True(t, state.IsNotFoundError(err), "identity should not have been created, got %v", err)
	})

	t.Run("skips service accounts", func(t *testing.T) {
		t.Parallel()

		ctx, st, logger := setup(t)

		require.NoError(t, user.Ensure(ctx, st, automation, role.Reader, false))

		identity, err := safe.StateGetByID[*auth.Identity](ctx, st, automation)
		require.NoError(t, err)

		identity.Metadata().Labels().Set(auth.LabelIdentityTypeServiceAccount, "")
		require.NoError(t, st.Update(ctx, identity))

		require.NoError(t, user.ElevateRecoveryAdmin(ctx, st, logger, automation))

		assert.Equal(t, roleReader, roleOf(ctx, t, st, automation))
	})

	t.Run("overwrites an unparseable role", func(t *testing.T) {
		t.Parallel()

		ctx, st, logger := setup(t)

		identity, err := safe.StateGetByID[*auth.Identity](ctx, st, locked)
		require.NoError(t, err)

		_, err = safe.StateUpdateWithConflicts(ctx, st, auth.NewUser(identity.TypedSpec().Value.UserId).Metadata(),
			func(usr *auth.User) error {
				usr.TypedSpec().Value.Role = roleUnknown

				return nil
			})
		require.NoError(t, err)

		require.NoError(t, user.ElevateRecoveryAdmin(ctx, st, logger, locked))

		assert.Equal(t, roleAdmin, roleOf(ctx, t, st, locked))
	})
}
