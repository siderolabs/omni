// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func TestImportedClusterSecretsCleanup(t *testing.T) {
	t.Parallel()

	t.Run("destroys ICS once ClusterSecrets is marked imported", func(t *testing.T) {
		t.Parallel()

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(&secrets.ImportedClusterSecretsCleanupController{}))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State

				const clusterID = "imported-cluster"

				ics := omni.NewImportedClusterSecrets(clusterID)
				ics.TypedSpec().Value.Data = "secret-bundle"
				require.NoError(t, st.Create(ctx, ics))

				cs := omni.NewClusterSecrets(clusterID)
				cs.TypedSpec().Value.Data = []byte("derived-bundle")
				cs.TypedSpec().Value.Imported = true
				require.NoError(t, st.Create(ctx, cs))

				rtestutils.AssertNoResource[*omni.ImportedClusterSecrets](ctx, t, st, clusterID)
			},
		)
	})
}
