// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package user_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/user"
)

func TestSync(t *testing.T) {
	t.Parallel()

	const (
		existing = "existing@example.com"
		newcomer = "newcomer@example.com"
		devs     = auth.OIDCLabelPrefix + "groups/devs"
	)

	ctx := t.Context()
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, user.Ensure(ctx, st, existing, role.Reader, false))

	require.NoError(t, user.NewSyncer(st, zaptest.NewLogger(t), "", false).Sync(ctx, newcomer, map[string]string{devs: ""}))

	_, err := safe.StateGetByID[*auth.Identity](ctx, st, newcomer)
	require.True(t, state.IsNotFoundError(err), "expected no identity, got %v", err)

	syncer := user.NewSyncer(st, zaptest.NewLogger(t), "", true)

	require.NoError(t, syncer.Sync(ctx, newcomer, map[string]string{devs: ""}))
	require.EqualValues(t, role.None, roleOf(ctx, t, st, newcomer))

	// moved from SAML to OIDC, and left the admins group, so the labels of both providers are replaced
	require.NoError(t, syncer.Sync(ctx, existing, map[string]string{auth.SAMLLabelPrefix + "groups/admins": ""}))
	require.NoError(t, syncer.Sync(ctx, existing, map[string]string{devs: ""}))

	identity, err := safe.StateGetByID[*auth.Identity](ctx, st, existing)
	require.NoError(t, err)

	require.Equal(t, map[string]string{
		auth.LabelIdentityUserID: identity.TypedSpec().Value.UserId,
		devs:                     "",
	}, identity.Metadata().Labels().Raw())
}
