// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/machineoperation"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func TestMachineOperationStatusController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	operations := machineoperation.NewReporter()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineOperationStatusController(operations)))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			const (
				inCluster   = "in-cluster"
				unallocated = "unallocated"
			)

			setBootID := func(machineID, bootID string) {
				snapshot := omni.NewMachineStatusSnapshot(machineID)

				require.NoError(t, safe.StateModify(ctx, st, snapshot, func(res *omni.MachineStatusSnapshot) error {
					res.TypedSpec().Value.BootId = bootID

					return nil
				}))
			}

			clusterMachine := omni.NewClusterMachine(inCluster)
			clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "cluster-1")
			clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "cluster-1-workers")

			require.NoError(t, st.Create(ctx, clusterMachine))

			setBootID(inCluster, "boot-1")
			setBootID(unallocated, "boot-1")

			// a status is published with the first report, and follows the operation
			operation := operations.Start(inCluster, specs.MachineOperationStatusSpec_PULLING_IMAGE)

			rtestutils.AssertResource(ctx, t, st, inCluster, func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationStatusSpec_PULLING_IMAGE, status.TypedSpec().Value.Stage)
				assertion.NotNil(status.TypedSpec().Value.StageStartedAt)

				cluster, _ := status.Metadata().Labels().Get(omni.LabelCluster)
				assertion.Equal("cluster-1", cluster)

				machineSet, _ := status.Metadata().Labels().Get(omni.LabelMachineSet)
				assertion.Equal("cluster-1-workers", machineSet)
			})

			operation.SetStage(specs.MachineOperationStatusSpec_DRAINING)
			operation.SetMessage("evicting pod")

			rtestutils.AssertResource(ctx, t, st, inCluster, func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationStatusSpec_DRAINING, status.TypedSpec().Value.Stage)
				assertion.Equal("evicting pod", status.TypedSpec().Value.Message)
			})

			// the labels follow the cluster machine
			rtestutils.Destroy[*omni.ClusterMachine](ctx, t, st, []string{inCluster})

			rtestutils.AssertResource(ctx, t, st, inCluster, func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				_, hasCluster := status.Metadata().Labels().Get(omni.LabelCluster)
				assertion.False(hasCluster)

				_, hasMachineSet := status.Metadata().Labels().Get(omni.LabelMachineSet)
				assertion.False(hasMachineSet)
			})

			// the reboot keeps the status until the machine reports another boot ID
			operation.SetStage(specs.MachineOperationStatusSpec_REBOOTING)
			operation.End("boot-1")

			rtestutils.AssertResource(ctx, t, st, inCluster, func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationStatusSpec_REBOOTING, status.TypedSpec().Value.Stage)
			})

			setBootID(inCluster, "boot-2")

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, inCluster)

			// an operation which ends with its call destroys the status
			other := operations.Start(unallocated, specs.MachineOperationStatusSpec_INSTALLING)

			rtestutils.AssertResource(ctx, t, st, unallocated, func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationStatusSpec_INSTALLING, status.TypedSpec().Value.Stage)
			})

			other.End("")

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, unallocated)

			// a reboot reported for a machine which already has another boot ID is not kept
			operations.Start(inCluster, specs.MachineOperationStatusSpec_REBOOTING).End("boot-1")

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, inCluster)

			// a machine removed while it reboots loses its status
			operations.Start(unallocated, specs.MachineOperationStatusSpec_REBOOTING).End("boot-1")

			rtestutils.AssertResource(ctx, t, st, unallocated, func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationStatusSpec_REBOOTING, status.TypedSpec().Value.Stage)
			})

			rtestutils.Destroy[*omni.MachineStatusSnapshot](ctx, t, st, []string{unallocated})

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, unallocated)

			// a machine removed while its operation still runs loses its status, and a later report does not bring it back
			running := operations.Start(inCluster, specs.MachineOperationStatusSpec_PULLING_IMAGE)

			rtestutils.AssertResource(ctx, t, st, inCluster, func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationStatusSpec_PULLING_IMAGE, status.TypedSpec().Value.Stage)
			})

			rtestutils.Destroy[*omni.MachineStatusSnapshot](ctx, t, st, []string{inCluster})

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, inCluster)

			running.SetMessage("late progress")

			setBootID(unallocated, "boot-3")

			last := operations.Start(unallocated, specs.MachineOperationStatusSpec_UPGRADING)

			rtestutils.AssertResource(ctx, t, st, unallocated, func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationStatusSpec_UPGRADING, status.TypedSpec().Value.Stage)
			})

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, inCluster)

			running.End("")
			last.End("")

			// an operation which starts before the snapshot of its machine exists is published once the snapshot is there
			early := operations.Start("early", specs.MachineOperationStatusSpec_INSTALLING)

			setBootID("early", "boot-1")

			rtestutils.AssertResource(ctx, t, st, "early", func(status *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationStatusSpec_INSTALLING, status.TypedSpec().Value.Stage)
			})

			early.End("")

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, "early")
		},
	)
}
