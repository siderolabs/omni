// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

type fakeMachineOperations struct {
	forgotten []string
	removed   []string
	mu        sync.Mutex
}

func (f *fakeMachineOperations) Forget(machineID string, machineRemoved bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if machineRemoved {
		f.removed = append(f.removed, machineID)
	} else {
		f.forgotten = append(f.forgotten, machineID)
	}
}

func (f *fakeMachineOperations) forgottenMachines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.forgotten)
}

func (f *fakeMachineOperations) removedMachines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.removed)
}

func TestMachineOperationStatusController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	operations := &fakeMachineOperations{}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineOperationStatusController(operations)))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			startedAt := time.Unix(1000, 0).UTC()

			require.NoError(t, st.Create(ctx, omni.NewMachine("m1")))

			rtestutils.AssertResource(ctx, t, st, "m1", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationSpec_UNKNOWN, res.TypedSpec().Value.Stage)
				assertion.Empty(res.TypedSpec().Value.LastError)
			})

			machineSetNode := omni.NewMachineSetNode("m1", omni.NewMachineSet("cluster-a-workers"))
			machineSetNode.Metadata().Labels().Set(omni.LabelCluster, "cluster-a")
			machineSetNode.Metadata().Labels().Set(omni.LabelMachineSet, "cluster-a-workers")

			require.NoError(t, st.Create(ctx, machineSetNode))

			rtestutils.AssertResource(ctx, t, st, "m1", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				cluster, _ := res.Metadata().Labels().Get(omni.LabelCluster)
				machineSet, _ := res.Metadata().Labels().Get(omni.LabelMachineSet)

				assertion.Equal("cluster-a", cluster)
				assertion.Equal("cluster-a-workers", machineSet)
			})

			rtestutils.Destroy[*omni.MachineSetNode](ctx, t, st, []string{"m1"})

			rtestutils.AssertResource(ctx, t, st, "m1", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				_, hasCluster := res.Metadata().Labels().Get(omni.LabelCluster)
				_, hasMachineSet := res.Metadata().Labels().Get(omni.LabelMachineSet)

				assertion.False(hasCluster, "the status of a machine which left its cluster must not keep the cluster label")
				assertion.False(hasMachineSet)
			})

			operation := omni.NewMachineOperation("m1")
			operation.TypedSpec().Value.Stage = specs.MachineOperationSpec_PULLING_IMAGE
			operation.TypedSpec().Value.Message = "pulling"
			operation.TypedSpec().Value.StageStartedAt = timestamppb.New(startedAt)

			require.NoError(t, st.Create(ctx, operation))

			rtestutils.AssertResource(ctx, t, st, "m1", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationSpec_PULLING_IMAGE, res.TypedSpec().Value.Stage)
				assertion.Equal("pulling", res.TypedSpec().Value.Message)
				assertion.Equal(startedAt, res.TypedSpec().Value.StageStartedAt.AsTime())
			})

			_, err := safe.StateUpdateWithConflicts(ctx, st, operation.Metadata(), func(res *omni.MachineOperation) error {
				res.TypedSpec().Value = &specs.MachineOperationSpec{
					Message:     "uncordoning",
					LastError:   "failed",
					FailedStage: specs.MachineOperationSpec_UNCORDONING,
				}

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, st, "m1", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationSpec_UNKNOWN, res.TypedSpec().Value.Stage)
				assertion.Equal("uncordoning", res.TypedSpec().Value.Message)
				assertion.Equal("failed", res.TypedSpec().Value.LastError)
				assertion.Equal(specs.MachineOperationSpec_UNCORDONING, res.TypedSpec().Value.FailedStage)
				assertion.Nil(res.TypedSpec().Value.StageStartedAt)
			})

			rtestutils.Destroy[*omni.MachineOperation](ctx, t, st, []string{"m1"})

			rtestutils.AssertResource(ctx, t, st, "m1", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Empty(res.TypedSpec().Value.Message)
				assertion.Empty(res.TypedSpec().Value.LastError)
				assertion.Equal(specs.MachineOperationSpec_UNKNOWN, res.TypedSpec().Value.FailedStage)
			})

			early := omni.NewMachineOperation("m2")
			early.TypedSpec().Value.Stage = specs.MachineOperationSpec_INSTALLING

			require.NoError(t, st.Create(ctx, early))
			require.NoError(t, st.Create(ctx, omni.NewMachine("m2")))

			rtestutils.AssertResource(ctx, t, st, "m2", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationSpec_INSTALLING, res.TypedSpec().Value.Stage)
			})

			snapshot := omni.NewMachineStatusSnapshot("m3")
			snapshot.TypedSpec().Value.BootId = "boot-1"

			rebooting := omni.NewMachineOperation("m3")
			rebooting.TypedSpec().Value.Stage = specs.MachineOperationSpec_REBOOTING
			rebooting.TypedSpec().Value.PreRebootBootId = "boot-1"

			require.NoError(t, st.Create(ctx, snapshot))
			require.NoError(t, st.Create(ctx, rebooting))
			require.NoError(t, st.Create(ctx, omni.NewMachine("m3")))

			rtestutils.AssertResource(ctx, t, st, "m3", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationSpec_REBOOTING, res.TypedSpec().Value.Stage)
			})

			_, err = safe.StateUpdateWithConflicts(ctx, st, snapshot.Metadata(), func(res *omni.MachineStatusSnapshot) error {
				res.TypedSpec().Value.BootId = "boot-2"

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, st, "m3", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationSpec_UNKNOWN, res.TypedSpec().Value.Stage, "a machine which booted again must not show the REBOOTING stage")
			})

			assert.Contains(t, operations.forgottenMachines(), "m3", "the operation of a machine which booted again must be forgotten")

			rtestutils.Destroy[*omni.Machine](ctx, t, st, []string{"m1"})

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, "m1")
			assert.Equal(t, []string{"m1"}, slices.Compact(operations.removedMachines()), "the operation of a removed machine must be forgotten")
		},
	)
}
