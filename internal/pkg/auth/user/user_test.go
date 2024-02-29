// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package user_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

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
	ctx := context.Background()

	const (
		john    = "john@example.com"
		richard = "richard@example.com"
		timothy = "timothy@example.com"
	)

	require.NoError(t, user.EnsureInitialResources(ctx, st, logger,
		[]string{
			john,
			richard,
		},
	))

	assertUsers(ctx, t, st, []string{john, richard})

	require.NoError(t, user.EnsureInitialResources(ctx, st, logger,
		[]string{
			richard,
			john,
		},
	))

	assertUsers(ctx, t, st, []string{john, richard})

	require.NoError(t, user.EnsureInitialResources(ctx, st, logger,
		[]string{
			richard,
			timothy,
		},
	))

	// new user doesn't get created, as the state is already initialized
	assertUsers(ctx, t, st, []string{john, richard})
}
