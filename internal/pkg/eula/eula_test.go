// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package eula_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/eula"
)

func TestEnsureAccepted(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)

	assertAccepted := func(name, email string) {
		res, err := safe.StateGetByID[*authres.EulaAcceptance](ctx, st, authres.EulaAcceptanceID)
		require.NoError(t, err)
		require.Equal(t, name, res.TypedSpec().Value.AcceptedByName)
		require.Equal(t, email, res.TypedSpec().Value.AcceptedByEmail)
	}

	require.ErrorContains(t, eula.EnsureAccepted(ctx, st, logger, "", ""), eula.URL)

	require.NoError(t, eula.EnsureAccepted(ctx, st, logger, "Test User", "test-user@example.com"))
	assertAccepted("Test User", "test-user@example.com")

	require.NoError(t, eula.EnsureAccepted(ctx, st, logger, "", ""))
	assertAccepted("Test User", "test-user@example.com")

	require.NoError(t, eula.EnsureAccepted(ctx, st, logger, "Another User", "another-user@example.com"))
	assertAccepted("Another User", "another-user@example.com")
}
