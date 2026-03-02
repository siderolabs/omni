// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	authctrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

const (
	roleAdmin    = string(role.Admin)
	roleReader   = string(role.Reader)
	roleOperator = string(role.Operator)
)

func TestIdentityStatus_AggregatesAllFields(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(authctrl.NewIdentityStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			rmock.Mock[*authres.User](ctx, t, st,
				options.WithID("user-1"),
				options.Modify(func(u *authres.User) error {
					u.TypedSpec().Value.Role = roleAdmin

					return nil
				}),
			)

			now := time.Now()

			rmock.Mock[*authres.IdentityLastActive](ctx, t, st,
				options.WithID("user@example.com"),
				options.Modify(func(r *authres.IdentityLastActive) error {
					r.TypedSpec().Value.LastActive = timestamppb.New(now)

					return nil
				}),
			)

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("user@example.com"),
				options.Modify(func(i *authres.Identity) error {
					i.TypedSpec().Value.UserId = "user-1"
					i.Metadata().Labels().Set(authres.LabelIdentityUserID, "user-1")

					return nil
				}),
			)

			rtestutils.AssertResource(ctx, t, st, "user@example.com", func(status *authres.IdentityStatus, asrt *assert.Assertions) {
				asrt.Equal("user-1", status.TypedSpec().Value.UserId)
				asrt.Equal(roleAdmin, status.TypedSpec().Value.Role)
				asrt.Equal(now.UTC().Format(time.RFC3339), status.TypedSpec().Value.LastActive)
			})
		},
	)
}

func TestIdentityStatus_SkipsWhenUserNotFound(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(authctrl.NewIdentityStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("orphan@example.com"),
				options.Modify(func(i *authres.Identity) error {
					i.TypedSpec().Value.UserId = "nonexistent-user"
					i.Metadata().Labels().Set(authres.LabelIdentityUserID, "nonexistent-user")

					return nil
				}),
			)

			// No IdentityStatus should be created since the User doesn't exist.
			time.Sleep(500 * time.Millisecond)
			rtestutils.AssertNoResource[*authres.IdentityStatus](ctx, t, st, "orphan@example.com")
		},
	)
}

func TestIdentityStatus_EmptyLastActive(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(authctrl.NewIdentityStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			rmock.Mock[*authres.User](ctx, t, st,
				options.WithID("user-2"),
				options.Modify(func(u *authres.User) error {
					u.TypedSpec().Value.Role = roleReader

					return nil
				}),
			)

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("inactive@example.com"),
				options.Modify(func(i *authres.Identity) error {
					i.TypedSpec().Value.UserId = "user-2"
					i.Metadata().Labels().Set(authres.LabelIdentityUserID, "user-2")

					return nil
				}),
			)

			rtestutils.AssertResource(ctx, t, st, "inactive@example.com", func(status *authres.IdentityStatus, asrt *assert.Assertions) {
				asrt.Equal("user-2", status.TypedSpec().Value.UserId)
				asrt.Equal(roleReader, status.TypedSpec().Value.Role)
				asrt.Empty(status.TypedSpec().Value.LastActive)
			})
		},
	)
}

func TestIdentityStatus_PropagatesServiceAccountLabel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(authctrl.NewIdentityStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			rmock.Mock[*authres.User](ctx, t, st,
				options.WithID("sa-user-1"),
				options.Modify(func(u *authres.User) error {
					u.TypedSpec().Value.Role = roleAdmin

					return nil
				}),
			)

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("mysa@serviceaccount.omni.sidero.dev"),
				options.Modify(func(i *authres.Identity) error {
					i.TypedSpec().Value.UserId = "sa-user-1"
					i.Metadata().Labels().Set(authres.LabelIdentityUserID, "sa-user-1")
					i.Metadata().Labels().Set(authres.LabelIdentityTypeServiceAccount, "")

					return nil
				}),
			)

			rtestutils.AssertResource(ctx, t, st, "mysa@serviceaccount.omni.sidero.dev", func(status *authres.IdentityStatus, asrt *assert.Assertions) {
				_, hasSALabel := status.Metadata().Labels().Get(authres.LabelIdentityTypeServiceAccount)
				asrt.True(hasSALabel)
				asrt.Equal(roleAdmin, status.TypedSpec().Value.Role)
			})
		},
	)
}

func TestIdentityStatus_UpdatesOnRoleChange(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(authctrl.NewIdentityStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			rmock.Mock[*authres.User](ctx, t, st,
				options.WithID("user-3"),
				options.Modify(func(u *authres.User) error {
					u.TypedSpec().Value.Role = roleReader

					return nil
				}),
			)

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("changeme@example.com"),
				options.Modify(func(i *authres.Identity) error {
					i.TypedSpec().Value.UserId = "user-3"
					i.Metadata().Labels().Set(authres.LabelIdentityUserID, "user-3")

					return nil
				}),
			)

			rtestutils.AssertResource(ctx, t, st, "changeme@example.com", func(status *authres.IdentityStatus, asrt *assert.Assertions) {
				asrt.Equal("Reader", status.TypedSpec().Value.Role)
			})

			// Update the user role.
			rmock.Mock[*authres.User](ctx, t, st,
				options.WithID("user-3"),
				options.Modify(func(u *authres.User) error {
					u.TypedSpec().Value.Role = roleAdmin

					return nil
				}),
			)

			rtestutils.AssertResource(ctx, t, st, "changeme@example.com", func(status *authres.IdentityStatus, asrt *assert.Assertions) {
				asrt.Equal(roleAdmin, status.TypedSpec().Value.Role)
			})
		},
	)
}

func TestIdentityStatus_UpdatesOnLastActiveChange(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(authctrl.NewIdentityStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			rmock.Mock[*authres.User](ctx, t, st,
				options.WithID("user-4"),
				options.Modify(func(u *authres.User) error {
					u.TypedSpec().Value.Role = roleOperator

					return nil
				}),
			)

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("track@example.com"),
				options.Modify(func(i *authres.Identity) error {
					i.TypedSpec().Value.UserId = "user-4"
					i.Metadata().Labels().Set(authres.LabelIdentityUserID, "user-4")

					return nil
				}),
			)

			// Initially no last active.
			rtestutils.AssertResource(ctx, t, st, "track@example.com", func(status *authres.IdentityStatus, asrt *assert.Assertions) {
				asrt.Empty(status.TypedSpec().Value.LastActive)
			})

			// Record activity.
			now := time.Now()

			rmock.Mock[*authres.IdentityLastActive](ctx, t, st,
				options.WithID("track@example.com"),
				options.Modify(func(r *authres.IdentityLastActive) error {
					r.TypedSpec().Value.LastActive = timestamppb.New(now)

					return nil
				}),
			)

			rtestutils.AssertResource(ctx, t, st, "track@example.com", func(status *authres.IdentityStatus, asrt *assert.Assertions) {
				asrt.Equal(now.UTC().Format(time.RFC3339), status.TypedSpec().Value.LastActive)
			})
		},
	)
}
