// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
)

// AssertWorkerNodesConfigUpdateRolloutCompletes tests that a capped config rollout reaches every worker.
//
// Talos 1.14 and later apply configuration without rebooting, so a config rollout finishes too fast for
// the cap itself to be observable from outside. What stays worth testing is that a capped rollout still
// reaches every machine, because a leaked config update lock would wedge it at the cap forever.
func AssertWorkerNodesConfigUpdateRolloutCompletes(testCtx context.Context, cli *client.Client, clusterName string, maxParallelism int) TestFunc {
	return func(t *testing.T) {
		require.GreaterOrEqual(t, maxParallelism, 2, "maxParallelism should be greater or equal to 2 for the test to be meaningful")

		st := cli.Omni().State()
		workersResourceID := omni.WorkersResourceID(clusterName)

		ctx, cancel := context.WithTimeout(testCtx, 5*time.Minute)
		t.Cleanup(cancel)

		workerIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, workersResourceID)))
		require.Greater(t, len(workerIDs), maxParallelism, "the rollout must be capped below the number of machines for a wedged rollout to be observable")

		// cap how many machines may be updated at once
		_, err := safe.StateUpdateWithConflicts[*omni.MachineSet](ctx, st, omni.NewMachineSet(workersResourceID).Metadata(), func(ms *omni.MachineSet) error {
			ms.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling
			ms.TypedSpec().Value.UpdateStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
				Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
					MaxParallelism: uint32(maxParallelism),
				},
			}

			return nil
		})
		require.NoError(t, err)

		epochSeconds := time.Now().Unix()
		iface := fmt.Sprintf("dummy%d", epochSeconds)

		machineSetPatch := omni.NewConfigPatch(
			fmt.Sprintf("000-test-update-rollout-%d", epochSeconds),
			pair.MakePair(omni.LabelCluster, clusterName),
			pair.MakePair(omni.LabelMachineSet, workersResourceID),
		)

		require.NoError(t, machineSetPatch.TypedSpec().Value.SetUncompressedData(fmt.Appendf(nil, dummyIfacePatchTemplate, iface)))
		require.NoError(t, st.Create(ctx, machineSetPatch))

		// Every worker must end up carrying the new config in its recorded status. The controller writes
		// it right after a successful apply call, so a rollout wedged on the update lock, or one whose
		// applies keep failing, never gets every machine there.
		rtestutils.AssertResources(ctx, t, st, workerIDs, func(cmcs *omni.ClusterMachineConfigStatus, assertion *assert.Assertions) {
			buffer, dataErr := cmcs.TypedSpec().Value.GetUncompressedData()
			if !assertion.NoError(dataErr) {
				return
			}

			defer buffer.Free()

			assertion.Contains(string(buffer.Data()), iface, "machine %q has not received the new config", cmcs.Metadata().ID())
		})

		// and the machine set must settle back into the running phase
		rtestutils.AssertResource(ctx, t, st, workersResourceID, func(ms *omni.MachineSetStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.MachineSetPhase_Running, ms.TypedSpec().Value.GetPhase())
		})

		// Every config update lock must be released. The two assertions above do not cover this: the config
		// is recorded before the lock is dropped, and the machine set phase is derived from the pending
		// updates rather than from the locks. A leak on the last machine would leave both of them happy
		// while permanently costing a slot on every later capped rollout.
		rtestutils.AssertResources(ctx, t, st, workerIDs, func(cm *omni.ClusterMachine, assertion *assert.Assertions) {
			assertion.False(cm.Metadata().Finalizers().Has(machineconfig.ConfigUpdateFinalizer),
				"machine %q still holds the config update lock", cm.Metadata().ID())
		})

		rtestutils.Destroy[*omni.ConfigPatch](ctx, t, st, []string{machineSetPatch.Metadata().ID()})
	}
}

// AssertWorkerNodesUpgradeParallelism tests that an upgrade rollout respects the machine set upgrade parallelism.
//
// Upgrades still reboot the machine, so unlike a config rollout the cap is observable. The observation
// uses the upgrade lock rather than the machine stage, because the lock is held across the whole
// operation including the reboot, while a stage poll can miss a machine entirely.
func AssertWorkerNodesUpgradeParallelism(testCtx context.Context, cli *client.Client, clusterName string, maxParallelism int, extensions []string) TestFunc {
	return func(t *testing.T) {
		require.GreaterOrEqual(t, maxParallelism, 2, "maxParallelism should be greater or equal to 2 for the test to be meaningful")

		st := cli.Omni().State()
		workersResourceID := omni.WorkersResourceID(clusterName)

		workerIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](testCtx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, workersResourceID)))
		require.Greater(t, len(workerIDs), maxParallelism, "number of machines in the worker machine set must to be greater than maxParallelism")

		ctx, cancel := context.WithTimeout(testCtx, 20*time.Minute)
		t.Cleanup(cancel)

		// cap how many machines may be upgraded at once
		_, err := safe.StateUpdateWithConflicts[*omni.MachineSet](ctx, st, omni.NewMachineSet(workersResourceID).Metadata(), func(ms *omni.MachineSet) error {
			ms.TypedSpec().Value.UpgradeStrategy = specs.MachineSetSpec_Rolling
			ms.TypedSpec().Value.UpgradeStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
				Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
					MaxParallelism: uint32(maxParallelism),
				},
			}

			return nil
		})
		require.NoError(t, err)

		// the rollout controller must pick the cap up before anything is triggered, otherwise the
		// assertion below could pass simply because the upgrade ran unconstrained
		rtestutils.AssertResource(ctx, t, st, clusterName, func(rollout *omni.UpgradeRollout, assertion *assert.Assertions) {
			assertion.Equal(int32(maxParallelism), rollout.TypedSpec().Value.GetMachineSetsUpgradeQuota()[workersResourceID])
		})

		// Remember what schematic each worker runs, to prove below that the change actually changed it.
		schematicIDsBefore := make(map[resource.ID]string, len(workerIDs))

		for _, id := range workerIDs {
			schematicConfiguration, getErr := safe.StateGetByID[*omni.SchematicConfiguration](ctx, st, id)
			require.NoError(t, getErr)

			schematicIDsBefore[id] = schematicConfiguration.TypedSpec().Value.SchematicId
		}

		// Changing the schematic upgrades the machines, which is the same rollout path as a Talos version
		// change. Scope it to the worker machine set: a cluster wide change upgrades the control plane
		// first, and the rollout controller withholds worker quota while any control plane is outdated, so
		// an unrelated control plane reboot would eat the timeout of a test that only cares about workers.
		extensionsConfiguration := omni.NewExtensionsConfiguration(workersResourceID)

		createOrUpdate(ctx, t, st, extensionsConfiguration, func(res *omni.ExtensionsConfiguration) error {
			res.Metadata().Labels().Set(omni.LabelCluster, clusterName)
			res.Metadata().Labels().Set(omni.LabelMachineSet, workersResourceID)
			res.TypedSpec().Value.Extensions = extensions

			return nil
		})

		t.Logf("changing the schematic of the workers of cluster %q to have extensions %#v", clusterName, extensions)

		// The schematic of every worker must actually change, and quickly, since only the image factory
		// call stands between the configuration and the recomputed ID. Waiting out the full rollout
		// timeout to learn that the requested extensions were already installed helps nobody, and a no-op
		// change is a real hazard: boot media which already carry the extensions turn this test into an
		// eighteen minute wait for an upgrade that never comes.
		schematicChangeCtx, schematicChangeCancel := context.WithTimeout(ctx, 3*time.Minute)
		defer schematicChangeCancel()

		rtestutils.AssertResources(schematicChangeCtx, t, st, workerIDs, func(res *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.NotEqual(schematicIDsBefore[res.Metadata().ID()], res.TypedSpec().Value.SchematicId,
				"the schematic of machine %q did not change: the requested extensions must differ from the ones the machines already run", res.Metadata().ID())
		})

		var (
			maxUpgradingAtOnce int
			sawWorkerUpgrade   bool
		)

		// Watch the upgrade lock for the whole rollout. The count may legitimately be lower than the cap,
		// because the rollout controller subtracts machines that are not ready, so only the ceiling is
		// asserted. The status is already Done before the controllers notice the new schematic, so the
		// rollout is only considered finished once a worker has actually been seen holding the lock.
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			// Count from a single list, not a read per machine: the lock moves between machines, so
			// separate reads can see it on the machine that just released it and on the one that just took
			// it, and report more holders than ever existed at once.
			clusterMachines, listErr := safe.StateListAll[*omni.ClusterMachine](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, workersResourceID)))
			if !assert.NoError(collect, listErr) {
				return
			}

			upgrading := 0

			for clusterMachine := range clusterMachines.All() {
				if clusterMachine.Metadata().Finalizers().Has(machineconfig.UpgradeFinalizer) {
					upgrading++
				}
			}

			if upgrading > 0 {
				sawWorkerUpgrade = true
			}

			if upgrading > maxUpgradingAtOnce {
				maxUpgradingAtOnce = upgrading
			}

			// A cap violation is asserted after the loop, on the running maximum: a failed assertion
			// inside this callback would only make the poll retry, not fail the test.

			status, getErr := safe.StateGetByID[*omni.TalosUpgradeStatus](ctx, st, clusterName)
			if !assert.NoError(collect, getErr) {
				return
			}

			assert.True(collect, sawWorkerUpgrade, "no worker has started upgrading yet")
			assert.Equal(collect, specs.TalosUpgradeStatusSpec_Done, status.TypedSpec().Value.Phase)
		}, 18*time.Minute, 500*time.Millisecond)

		assert.LessOrEqual(t, maxUpgradingAtOnce, maxParallelism, "expected a maximum of %d machines to be upgrading at the same time", maxParallelism)

		t.Logf("upgrade rollout finished, at most %d machines were upgrading at once", maxUpgradingAtOnce)

		rtestutils.Destroy[*omni.ExtensionsConfiguration](ctx, t, st, []string{extensionsConfiguration.Metadata().ID()})
	}
}

// AssertWorkerNodesRollingScaleDown tests that the machine set delete strategy parallelism is respected
// when the workers are scaled down.
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
		_, err = safe.StateUpdateWithConflicts[*omni.MachineSet](ctx, st, omni.NewMachineSet(workersResourceID).Metadata(), func(ms *omni.MachineSet) error {
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
			wg.Go(func() {
				rtestutils.Destroy[*omni.MachineSetNode](ctx, t, st, []string{node.Metadata().ID()})
			})
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
