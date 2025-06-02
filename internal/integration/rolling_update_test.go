// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/pair"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// AssertWorkerNodesRollingConfigUpdate tests that config patch rollout parallelism is respected.
// Example: if there are five workers and the rollout parallelism is 2, then at most two workers should be rebooting at the same time,
// and it should be rolled out to all the workers.
func AssertWorkerNodesRollingConfigUpdate(testCtx context.Context, cli *client.Client, clusterName string, maxParallelism int) TestFunc {
	return func(t *testing.T) {
		require.GreaterOrEqual(t, maxParallelism, 2, "maxParallelism should be greater or equal to 2 for the test to be meaningful")

		st := cli.Omni().State()
		workersResourceID := omni.WorkersResourceID(clusterName)

		clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](testCtx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, workersResourceID)))
		require.NoError(t, err)

		require.Greater(t, clusterMachineList.Len(), maxParallelism, "number of machine in the worker machine set must to be greater than maxParallelism")

		timeoutDuration := time.Duration(clusterMachineList.Len()) * time.Minute

		ctx, cancel := context.WithTimeout(testCtx, timeoutDuration+1*time.Minute)
		t.Cleanup(cancel)

		// update worker machine set to have rolling update with specified max parallelism
		_, err = safe.StateUpdateWithConflicts[*omni.MachineSet](ctx, st, omni.NewMachineSet(resources.DefaultNamespace, workersResourceID).Metadata(), func(ms *omni.MachineSet) error {
			ms.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling
			ms.TypedSpec().Value.UpdateStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
				Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
					MaxParallelism: uint32(maxParallelism),
				},
			}

			return nil
		})
		require.NoError(t, err)

		// create reboot-requiring config patch for the machine set
		epochSeconds := time.Now().Unix()
		machineSetPatch := omni.NewConfigPatch(resources.DefaultNamespace,
			fmt.Sprintf("000-test-update-parallelism-%d", epochSeconds),
			pair.MakePair(omni.LabelCluster, clusterName),
			pair.MakePair(omni.LabelMachineSet, workersResourceID))

		err = machineSetPatch.TypedSpec().Value.SetUncompressedData([]byte(fmt.Sprintf(`{"machine":{"env":{"%d":"test-val"}}}`, epochSeconds)))
		require.NoError(t, err)

		require.NoError(t, st.Create(ctx, machineSetPatch))

		// expect the machine set to go into the reconfiguring phase
		rtestutils.AssertResource(ctx, t, st, omni.WorkersResourceID(clusterName), func(ms *omni.MachineSetStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.MachineSetPhase_Reconfiguring, ms.TypedSpec().Value.GetPhase())
		})

		maxRebootingAtOnce := 0
		rebootedIDs := make(map[string]struct{})

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			machineStatusList, err := safe.StateListAll[*omni.ClusterMachineStatus](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, workersResourceID)))
			require.NoError(t, err)

			numRebooting := 0

			machineStatusList.ForEach(func(machineStatus *omni.ClusterMachineStatus) {
				stage := machineStatus.TypedSpec().Value.GetStage()

				assert.Equal(collect, specs.ClusterMachineStatusSpec_RUNNING, stage)
				assert.Equal(collect, specs.ConfigApplyStatus_APPLIED, machineStatus.TypedSpec().Value.GetConfigApplyStatus())

				if stage == specs.ClusterMachineStatusSpec_REBOOTING {
					rebootedIDs[machineStatus.Metadata().ID()] = struct{}{}
					numRebooting++
				}
			})

			if numRebooting > maxRebootingAtOnce {
				maxRebootingAtOnce = numRebooting
			}
		}, timeoutDuration, 1*time.Second)

		assert.Len(t, rebootedIDs, clusterMachineList.Len(), "expected all the machines to be rebooted")
		assert.Equal(t, maxParallelism, maxRebootingAtOnce, "expected a maximum of %d machines to be rebooting at the same time", maxParallelism)

		// wait for the machine set to go back into the running phase
		rtestutils.AssertResource(ctx, t, st, omni.WorkersResourceID(clusterName), func(ms *omni.MachineSetStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.MachineSetPhase_Running, ms.TypedSpec().Value.GetPhase())
		})
	}
}

// AssertWorkerNodesRollingScaleDown tests that config patch rollout parallelism is respected.
// Example: if there are five workers and the rollout parallelism is 2, then at most two workers should be rebooting at the same time,
// and it should be rolled out to all the workers.
func AssertWorkerNodesRollingScaleDown(testCtx context.Context, cli *client.Client, clusterName string, maxParallelism int) TestFunc {
	return func(t *testing.T) {
		st := cli.Omni().State()
		workersResourceID := omni.WorkersResourceID(clusterName)

		machineSetNodeList, err := safe.StateListAll[*omni.MachineSetNode](testCtx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, workersResourceID)))
		require.NoError(t, err)

		require.Greater(t, machineSetNodeList.Len(), maxParallelism, "number of machines in the worker machine set must to be greater than maxParallelism")

		timeoutDuration := time.Duration(machineSetNodeList.Len()) * time.Minute

		ctx, cancel := context.WithTimeout(testCtx, timeoutDuration+1*time.Minute)
		t.Cleanup(cancel)

		// update worker machine set to have rolling update max parallelism of 2
		_, err = safe.StateUpdateWithConflicts[*omni.MachineSet](ctx, st, omni.NewMachineSet(resources.DefaultNamespace, workersResourceID).Metadata(), func(ms *omni.MachineSet) error {
			ms.TypedSpec().Value.DeleteStrategy = specs.MachineSetSpec_Rolling
			ms.TypedSpec().Value.DeleteStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
				Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
					MaxParallelism: uint32(maxParallelism),
				},
			}

			return nil
		})
		require.NoError(t, err)

		// remove all workers without blocking
		var wg sync.WaitGroup

		t.Cleanup(wg.Wait)

		machineSetNodeList.ForEach(func(node *omni.MachineSetNode) {
			wg.Add(1)

			go func() {
				defer wg.Done()

				rtestutils.Destroy[*omni.MachineSetNode](ctx, t, st, []string{node.Metadata().ID()})
			}()
		})

		// expect the machine set to go into the ScalingDown phase
		rtestutils.AssertResource(ctx, t, st, omni.WorkersResourceID(clusterName), func(ms *omni.MachineSetStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.MachineSetPhase_ScalingDown, ms.TypedSpec().Value.GetPhase())
		})

		maxDestroyingAtOnce := 0
		destroyedIDs := make(map[string]struct{})

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			machineStatusList, err := safe.StateListAll[*omni.ClusterMachineStatus](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, workersResourceID)))
			require.NoError(t, err)

			numDeleting := 0

			machineStatusList.ForEach(func(machineStatus *omni.ClusterMachineStatus) {
				stage := machineStatus.TypedSpec().Value.GetStage()

				assert.Equal(collect, specs.ClusterMachineStatusSpec_DESTROYING, stage)

				if stage == specs.ClusterMachineStatusSpec_DESTROYING {
					destroyedIDs[machineStatus.Metadata().ID()] = struct{}{}
					numDeleting++
				}
			})

			if numDeleting > maxDestroyingAtOnce {
				maxDestroyingAtOnce = numDeleting
			}
		}, timeoutDuration, 1*time.Second)

		assert.Len(t, destroyedIDs, machineSetNodeList.Len(), "expected all the machines to be destroyed")
		assert.Equal(t, maxParallelism, maxDestroyingAtOnce, "expected a maximum of %d machines to be destroyed at the same time", maxParallelism)
	}
}
