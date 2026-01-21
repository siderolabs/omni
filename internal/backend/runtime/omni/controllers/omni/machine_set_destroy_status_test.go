// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func TestMachineSetDestroyStatusController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx,
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineSetDestroyStatusController()))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			machineSet := omni.NewMachineSet("ms1")
			machineSet.Metadata().Finalizers().Add("foo") // put a finalizer to mimic the machine set teardown
			require.NoError(t, testContext.State.Create(ctx, machineSet))

			for i := range 2 {
				cms := omni.NewClusterMachineStatus("cm" + strconv.Itoa(i))
				cms.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())
				require.NoError(t, testContext.State.Create(ctx, cms))
			}

			rtestutils.AssertResource(ctx, t, testContext.State, machineSet.Metadata().ID(), func(res *omni.MachineSet, asrt *assert.Assertions) {
				asrt.True(res.Metadata().Finalizers().Has(omnictrl.NewMachineSetDestroyStatusController().Name()))
			})

			rtestutils.AssertNoResource[*omni.MachineSetDestroyStatus](ctx, t, testContext.State, machineSet.Metadata().ID())

			_, err := testContext.State.Teardown(ctx, machineSet.Metadata())
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, testContext.State, machineSet.Metadata().ID(), func(res *omni.MachineSetDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 2 machines", res.TypedSpec().Value.Phase)
			})

			rtestutils.Destroy[*omni.ClusterMachineStatus](ctx, t, testContext.State, []string{"cm0"})

			rtestutils.AssertResource(ctx, t, testContext.State, machineSet.Metadata().ID(), func(res *omni.MachineSetDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 1 machine", res.TypedSpec().Value.Phase)
			})

			_, err = testContext.State.UpdateWithConflicts(ctx, machineSet.Metadata(), func(ms resource.Resource) error {
				ms.Metadata().Labels().Set("foo", "bar")

				return nil
			}, state.WithExpectedPhaseAny())
			require.NoError(t, err)

			rtestutils.Destroy[*omni.ClusterMachineStatus](ctx, t, testContext.State, []string{"cm1"})

			rtestutils.AssertResource(ctx, t, testContext.State, machineSet.Metadata().ID(), func(res *omni.MachineSetDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 0 machines", res.TypedSpec().Value.Phase)
			})

			require.NoError(t, testContext.State.RemoveFinalizer(ctx, machineSet.Metadata(), "foo"))

			rtestutils.AssertNoResource[*omni.MachineSetDestroyStatus](ctx, t, testContext.State, machineSet.Metadata().ID())
		},
	)
}
