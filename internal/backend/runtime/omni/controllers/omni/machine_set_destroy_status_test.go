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

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

func TestMachineSetDestroyStatusController(t *testing.T) {
	t.Parallel()

	sb := dynamicStateBuilder{m: map[resource.Namespace]state.CoreState{}}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	withRuntime(
		ctx,
		t,
		sb.Builder,
		func(_ context.Context, _ state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
			require.NoError(t, rt.RegisterQController(omnictrl.NewMachineSetDestroyStatusController()))
		},
		func(ctx context.Context, st state.State, _ *runtime.Runtime, _ *zap.Logger) {
			machineSet := omni.NewMachineSet(resources.DefaultNamespace, "ms1")
			machineSet.Metadata().Finalizers().Add("foo") // put a finalizer to mimic the machine set teardown
			require.NoError(t, st.Create(ctx, machineSet))

			for i := range 2 {
				cms := omni.NewClusterMachineStatus(resources.DefaultNamespace, "cm"+strconv.Itoa(i))
				cms.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())
				require.NoError(t, st.Create(ctx, cms))
			}

			rtestutils.AssertResource(ctx, t, st, machineSet.Metadata().ID(), func(res *omni.MachineSet, asrt *assert.Assertions) {
				asrt.True(res.Metadata().Finalizers().Has(omnictrl.NewMachineSetDestroyStatusController().Name()))
			})

			rtestutils.AssertNoResource[*omni.MachineSetDestroyStatus](ctx, t, st, machineSet.Metadata().ID())

			_, err := st.Teardown(ctx, machineSet.Metadata())
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, st, machineSet.Metadata().ID(), func(res *omni.MachineSetDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 2 machines", res.TypedSpec().Value.Phase)
			})

			rtestutils.AssertResource(ctx, t, st, "cm0", func(res *omni.ClusterMachineStatus, asrt *assert.Assertions) {
				asrt.True(res.Metadata().Finalizers().Has(omnictrl.NewMachineSetDestroyStatusController().Name()))
			})

			rtestutils.Destroy[*omni.ClusterMachineStatus](ctx, t, st, []string{"cm0"})

			rtestutils.AssertResource(ctx, t, st, machineSet.Metadata().ID(), func(res *omni.MachineSetDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 1 machine", res.TypedSpec().Value.Phase)
			})

			_, err = st.UpdateWithConflicts(ctx, machineSet.Metadata(), func(ms resource.Resource) error {
				ms.Metadata().Labels().Set("foo", "bar")

				return nil
			}, state.WithExpectedPhaseAny())
			require.NoError(t, err)

			rtestutils.Destroy[*omni.ClusterMachineStatus](ctx, t, st, []string{"cm1"})

			rtestutils.AssertResource(ctx, t, st, machineSet.Metadata().ID(), func(res *omni.MachineSetDestroyStatus, asrt *assert.Assertions) {
				asrt.Equal("Destroying: 0 machines", res.TypedSpec().Value.Phase)
			})

			require.NoError(t, st.RemoveFinalizer(ctx, machineSet.Metadata(), "foo"))

			rtestutils.AssertNoResource[*omni.MachineSetDestroyStatus](ctx, t, st, machineSet.Metadata().ID())
		},
	)
}
