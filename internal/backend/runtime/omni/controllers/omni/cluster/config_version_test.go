// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cluster_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/cluster"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

func TestConfigVersion(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		annotations map[string]string
		name        string
		expected    string
	}{
		{
			name:     "no annotations",
			expected: "v1.14.2",
		},
		{
			name: "importing",
			annotations: map[string]string{
				omni.ClusterImportIsInProgress:        "",
				omni.ClusterImportInitialTalosVersion: "1.6.4",
			},
			expected: "v1.6.4",
		},
		{
			name: "not importing",
			annotations: map[string]string{
				omni.ClusterImportInitialTalosVersion: "1.6.4",
			},
			expected: "v1.14.2",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(ctx, t, testutils.TestOptions{},
				func(_ context.Context, tc testutils.TestContext) {
					require.NoError(t, tc.Runtime.RegisterQController(cluster.NewConfigVersionController()))
				},
				func(ctx context.Context, tc testutils.TestContext) {
					rmock.Mock[*omni.Cluster](ctx, t, tc.State, options.WithID("test"), options.Modify(func(res *omni.Cluster) error {
						res.TypedSpec().Value.TalosVersion = "1.14.2"

						for key, value := range tt.annotations {
							res.Metadata().Annotations().Set(key, value)
						}

						return nil
					}))

					rtestutils.AssertResource(ctx, t, tc.State, "test", func(res *omni.ClusterConfigVersion, assertion *assert.Assertions) {
						assertion.Equal(tt.expected, res.TypedSpec().Value.Version)
					})
				},
			)
		})
	}
}
