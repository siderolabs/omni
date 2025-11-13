// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func TestClusterDestroyStatusController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx,
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterDestroyStatusController()))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			c := omni.NewCluster(resources.DefaultNamespace, "c1")
			c.Metadata().Finalizers().Add("foo") // put a finalizer to mimic the cluster teardown
			c.Metadata().Finalizers().Add(omnictrl.ClusterStatusControllerName)
			require.NoError(t, st.Create(ctx, c))

			machineSet := omni.NewMachineSetStatus(resources.DefaultNamespace, "ms1")
			machineSet.Metadata().Labels().Set(omni.LabelCluster, c.Metadata().ID())
			require.NoError(t, st.Create(ctx, machineSet))

			for i := range 2 {
				cms := omni.NewClusterMachineStatus(resources.DefaultNamespace, "cm"+strconv.Itoa(i))
				cms.Metadata().Labels().Set(omni.LabelCluster, c.Metadata().ID())
				cms.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())
				require.NoError(t, st.Create(ctx, cms))
			}

			rtestutils.AssertResource(ctx, t, st, c.Metadata().ID(), func(res *omni.Cluster, asrt *assert.Assertions) {
				asrt.True(res.Metadata().Finalizers().Has(omnictrl.NewClusterDestroyStatusController().Name()))
			})

			rtestutils.AssertNoResource[*omni.ClusterDestroyStatus](ctx, t, st, c.Metadata().ID())

			_, err := st.Teardown(ctx, c.Metadata())
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, st, c.Metadata().ID(), func(res *omni.ClusterDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 1 machine set, 2 machines", res.TypedSpec().Value.Phase)
			})

			rtestutils.Destroy[*omni.ClusterMachineStatus](ctx, t, st, []string{"cm0", "cm1"})

			rtestutils.AssertResource(ctx, t, st, c.Metadata().ID(), func(res *omni.ClusterDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 1 machine set, 0 machines", res.TypedSpec().Value.Phase)
			})

			rtestutils.Destroy[*omni.MachineSetStatus](ctx, t, st, []string{machineSet.Metadata().ID()})

			rtestutils.AssertResource(ctx, t, st, c.Metadata().ID(), func(res *omni.ClusterDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 0 machine sets, 0 machines", res.TypedSpec().Value.Phase)
			})

			require.NoError(t, st.RemoveFinalizer(ctx, c.Metadata(), "foo"))

			rtestutils.AssertNoResource[*omni.ClusterDestroyStatus](ctx, t, st, c.Metadata().ID())
		},
	)
}
