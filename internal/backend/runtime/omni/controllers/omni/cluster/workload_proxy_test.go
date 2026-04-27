// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cluster_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/cluster"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
)

func TestClusterWorkloadProxySuite(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		ctrl, err := cluster.NewClusterWorkloadProxyController(true)

		require.NoError(t, err)

		require.NoError(t, testContext.Runtime.RegisterQController(ctrl))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		cluster := rmock.Mock[*omni.Cluster](ctx, t, testContext.State)

		manifestID := fmt.Sprintf("cluster-%s-workload-proxy", cluster.Metadata().ID())

		_, err := safe.StateUpdateWithConflicts(ctx, testContext.State, cluster.Metadata(), func(r *omni.Cluster) error {
			r.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
				EnableWorkloadProxy: true,
			}

			return nil
		})

		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, testContext.State, []string{manifestID}, func(res *omni.KubernetesManifestGroup, assertion *assert.Assertions) {
			var buffer specs.Buffer

			buffer, err = res.TypedSpec().Value.GetUncompressedData()
			assertion.NoError(err)

			defer buffer.Free()

			data := string(buffer.Data())

			assertion.Contains(data, "omni-kube-service-exposer")
		})

		_, err = safe.StateUpdateWithConflicts(ctx, testContext.State, cluster.Metadata(), func(r *omni.Cluster) error {
			r.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
				EnableWorkloadProxy: false,
			}

			return nil
		})

		require.NoError(t, err)

		rtestutils.AssertNoResource[*omni.KubernetesManifestGroup](ctx, t, testContext.State, manifestID)

		rmock.Destroy[*omni.Cluster](ctx, t, testContext.State, []string{cluster.Metadata().ID()})
	})
}
