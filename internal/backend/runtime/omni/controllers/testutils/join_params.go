// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testutils

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// CreateJoinParams populates the default join token.
func CreateJoinParams(ctx context.Context, state state.State, t *testing.T) {
	params := siderolink.NewDefaultJoinToken()
	params.TypedSpec().Value.TokenId = "testtoken"

	require.NoError(t, state.Create(ctx, params))
	require.NoError(t, state.Create(ctx, siderolink.NewConfig()))
}
