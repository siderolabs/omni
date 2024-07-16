// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
)

type MachineSetStatusSuite struct {
	OmniSuite
}

func (suite *MachineSetStatusSuite) createMachineSet(clusterName string, machineSetName string, machines []string, patches ...string) *omni.MachineSet {
	opts := xslices.Map(patches, withPatch)

	return suite.createMachineSetWithOpts(clusterName, machineSetName, machines, opts...)
}

func (suite *MachineSetStatusSuite) createMachineSetWithOpts(clusterName string, machineSetName string, machines []string, opts ...option) *omni.MachineSet {
	options := initOptions(&options{healthy: true}, opts...)
	cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)

	cluster.TypedSpec().Value.TalosVersion = "1.2.1"
	cluster.TypedSpec().Value.KubernetesVersion = "1.25.0"

	err := suite.state.Create(suite.ctx, cluster)
	if !state.IsConflictError(err) {
		suite.Require().NoError(err)
	}

	clusterStatus := omni.NewClusterStatus(resources.DefaultNamespace, clusterName)

	clusterStatus.TypedSpec().Value.Available = true

	err = suite.state.Create(suite.ctx, clusterStatus)
	if !state.IsConflictError(err) {
		suite.Require().NoError(err)
	}

	secrets := omni.NewClusterSecrets(resources.DefaultNamespace, clusterName)

	bundle, err := talossecrets.NewBundle(talossecrets.NewClock(), config.TalosVersionCurrent)
	suite.Require().NoError(err)

	data, err := json.Marshal(bundle)
	suite.Require().NoError(err)

	secrets.TypedSpec().Value.Data = data

	err = suite.state.Create(suite.ctx, secrets)
	if !state.IsConflictError(err) {
		suite.Require().NoError(err)
	}

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, machineSetName)
	machineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	spec := machineSet.TypedSpec().Value

	loadbalancer := omni.NewLoadBalancerStatus(resources.DefaultNamespace, clusterName)
	loadbalancer.TypedSpec().Value.Healthy = options.healthy

	err = suite.state.Create(suite.ctx, loadbalancer)
	if !state.IsConflictError(err) {
		suite.Require().NoError(err)
	}

	patchName := fmt.Sprintf("%s-patch-default", machineSetName)

	for i, patch := range options.patches {
		extraPatchName := fmt.Sprintf("%s-patch-extra%d", machineSetName, i)

		extraPatch := omni.NewConfigPatch(
			resources.DefaultNamespace, extraPatchName,
			pair.MakePair(omni.LabelCluster, clusterName),
			pair.MakePair(omni.LabelMachineSet, machineSetName),
		)

		extraPatch.TypedSpec().Value.Data = patch

		suite.Require().NoError(suite.state.Create(suite.ctx, extraPatch))
	}

	spec.UpdateStrategy = specs.MachineSetSpec_Rolling
	if options.maxUpdateParallelism > 0 {
		spec.UpdateStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
			Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
				MaxParallelism: options.maxUpdateParallelism,
			},
		}
	}

	if options.maxDeleteParallelism > 0 {
		spec.DeleteStrategy = specs.MachineSetSpec_Rolling
		spec.DeleteStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
			Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
				MaxParallelism: options.maxDeleteParallelism,
			},
		}
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, machineSet))

	patch1 := omni.NewConfigPatch(resources.DefaultNamespace, patchName, pair.MakePair(omni.LabelCluster, clusterName))

	patch1.TypedSpec().Value.Data = `machine:
  network:
    kubespan:
      enabled: true`

	err = suite.state.Create(suite.ctx, patch1)
	if !state.IsConflictError(err) {
		suite.Require().NoError(err)
	}

	for _, machine := range machines {
		mchn := omni.NewMachine(resources.DefaultNamespace, machine)
		mchn.TypedSpec().Value.ManagementAddress = suite.socketConnectionString

		err = suite.state.Create(suite.ctx, mchn)
		if !state.IsConflictError(err) {
			suite.Require().NoError(err)
		}

		machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, machine)

		machineStatus.TypedSpec().Value.ManagementAddress = suite.socketConnectionString
		machineStatus.TypedSpec().Value.Connected = true

		machineStatus.TypedSpec().Value.Cluster = clusterName
		machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_CONTROL_PLANE

		err = suite.state.Create(suite.ctx, machineStatus)
		if !state.IsConflictError(err) {
			suite.Require().NoError(err)
		}

		msn := omni.NewMachineSetNode(resources.DefaultNamespace, machine, machineSet)

		suite.Require().NoError(suite.state.Create(suite.ctx, msn))

		clusterMachineIdentity := omni.NewClusterMachineIdentity(resources.DefaultNamespace, machine)

		err = suite.state.Create(suite.ctx, clusterMachineIdentity)
		if !state.IsConflictError(err) {
			suite.Require().NoError(err)
		}
	}

	return machineSet
}

//nolint:unparam
func (suite *MachineSetStatusSuite) updateStage(nodes []string, stage specs.ClusterMachineStatusSpec_Stage, ready bool) {
	for _, node := range nodes {
		cms := omni.NewClusterMachineStatus(resources.DefaultNamespace, node)
		cms.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
		spec := cms.TypedSpec().Value

		spec.Ready = ready
		spec.Stage = stage

		machine, err := suite.state.Get(suite.ctx, resource.NewMetadata(
			resources.DefaultNamespace,
			omni.ClusterMachineType,
			cms.Metadata().ID(),
			resource.VersionUndefined,
		))
		suite.Require().NoError(err)

		helpers.CopyLabels(machine, cms, omni.LabelControlPlaneRole, omni.LabelCluster, omni.LabelMachineSet, omni.LabelWorkerRole)

		err = suite.state.Create(suite.ctx, cms)
		if state.IsConflictError(err) {
			_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, cms.Metadata(), func(res *omni.ClusterMachineStatus) error {
				res.TypedSpec().Value = cms.TypedSpec().Value

				helpers.CopyLabels(machine, res, omni.LabelControlPlaneRole, omni.LabelCluster, omni.LabelMachineSet, omni.LabelWorkerRole)

				return nil
			})

			suite.Assert().NoError(err)
		}
	}
}

func (suite *MachineSetStatusSuite) syncConfig(nodes []string) {
	for _, node := range nodes {
		machine, err := suite.state.Get(suite.ctx, resource.NewMetadata(
			resources.DefaultNamespace,
			omni.ClusterMachineType,
			node,
			resource.VersionUndefined,
		))
		suite.Assert().NoError(err)

		cmcs := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, node)
		spec := cmcs.TypedSpec().Value
		spec.ClusterMachineVersion = machine.Metadata().Version().String()

		helpers.CopyLabels(machine, cmcs, omni.LabelControlPlaneRole, omni.LabelCluster, omni.LabelMachineSet, omni.LabelWorkerRole)

		err = suite.state.Create(suite.ctx, cmcs)
		if state.IsConflictError(err) {
			_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, cmcs.Metadata(), func(res *omni.ClusterMachineConfigStatus) error {
				res.TypedSpec().Value = cmcs.TypedSpec().Value
				res.TypedSpec().Value.ClusterMachineVersion = machine.Metadata().Version().String()

				helpers.CopyLabels(machine, res, omni.LabelControlPlaneRole, omni.LabelCluster, omni.LabelMachineSet, omni.LabelWorkerRole)

				return nil
			})

			suite.Assert().NoError(err)
		}
	}
}

func (suite *MachineSetStatusSuite) SetupTest() {
	suite.OmniSuite.SetupTest()

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineSetStatusController()))

	// create siderolink config as it's endpoint is used while generating kubernetes endpoint
	siderolink := siderolink.NewConfig(resources.DefaultNamespace)
	siderolink.TypedSpec().Value.WireguardEndpoint = "0.0.0.0"

	if err := suite.state.Create(suite.ctx, siderolink); err != nil && !state.IsConflictError(err) {
		suite.Assert().NoError(err)
	}
}

// TestScaleUp verifies that scaling the machine set up is never blocked.
// Checks the state of created ClusterMachines.
func (suite *MachineSetStatusSuite) TestScaleUp() {
	clusterName := "scale-up"

	machines := []string{
		"node1",
		"node2",
		"node3",
	}

	machineSet := suite.createMachineSet(clusterName, "machine-set-scale-up", machines)

	suite.assertMachinesState(machines, clusterName, machineSet.Metadata().ID())

	suite.updateStage(machines, specs.ClusterMachineStatusSpec_RUNNING, true)

	suite.assertMachineSetPhase(machineSet, *specs.MachineSetPhase_Running.Enum())
}

// TestEmptyTeardown should destroy the machine sets without cluster machines.
func (suite *MachineSetStatusSuite) TestEmptyTeardown() {
	cluster := omni.NewCluster(resources.DefaultNamespace, "test")

	ms := omni.NewMachineSet(resources.DefaultNamespace, "tearingdown")
	ms.Metadata().SetPhase(resource.PhaseTearingDown)
	ms.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

	msn := omni.NewMachineSetNode(resources.DefaultNamespace, "1", ms)
	msn.Metadata().Finalizers().Add(machineset.ControllerName)

	suite.Require().NoError(suite.state.Create(suite.ctx, msn))
	suite.Require().NoError(suite.state.Create(suite.ctx, ms))

	rtestutils.DestroyAll[*omni.MachineSetNode](suite.ctx, suite.T(), suite.state)
	rtestutils.DestroyAll[*omni.MachineSet](suite.ctx, suite.T(), suite.state)
}

// TestScaleDown create a machine set with 3 machines
// which are not running. Immediately try to scale down.
// Scale down should remove the first machine.
// Make all machines ready, check that machines count went to 2 and the machine set is ready.
func (suite *MachineSetStatusSuite) TestScaleDown() {
	clusterName := "scale-down"

	machines := []string{
		"scaledown-1",
		"scaledown-2",
		"scaledown-3",
	}

	// We create machine with "Healthy=false" status, so that rtestutils.Destroy will not work.
	machineSet := suite.createMachineSetWithOpts(clusterName, "machine-set-scale-down", machines, withHealthy(false))

	suite.assertMachinesState(machines, clusterName, machineSet.Metadata().ID())

	rtestutils.Destroy[*omni.MachineSetNode](suite.ctx, suite.T(), suite.state, []string{machines[0]})

	suite.assertMachineSetPhase(machineSet, specs.MachineSetPhase_ScalingUp)

	expectedMachines := []string{"scaledown-2", "scaledown-3"}

	suite.assertMachinesState(expectedMachines, clusterName, machineSet.Metadata().ID())

	suite.updateStage(expectedMachines, specs.ClusterMachineStatusSpec_RUNNING, true)

	loadbalancer := omni.NewLoadBalancerStatus(resources.DefaultNamespace, clusterName)
	_, err := safe.StateUpdateWithConflicts(
		suite.ctx,
		suite.state,
		loadbalancer.Metadata(),
		func(r *omni.LoadBalancerStatus) error {
			r.TypedSpec().Value.Healthy = true

			return nil
		},
	)
	suite.Require().NoError(err)

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertNoResource(*omni.NewClusterMachine(resources.DefaultNamespace, machines[0]).Metadata()),
	))

	suite.assertMachineSetPhase(machineSet, specs.MachineSetPhase_Running)
}

// TestScaleDownWithMaxParallelism tests scale down behaves correctly when max parallelism is set in a worker machine set.
func (suite *MachineSetStatusSuite) TestScaleDownWithMaxParallelism() {
	clusterName := "cluster-test-scale-down-max-parallelism"
	numMachines := 8
	machines := make([]string, 0, numMachines)

	for i := range numMachines {
		machines = append(machines, fmt.Sprintf("node-test-scale-down-max-parallelism-%02d", i))
	}

	machineSet := suite.createMachineSetWithOpts(clusterName, "machine-set-test-scale-down-max-parallelism", machines, withMaxDeleteParallelism(3))

	// initially, each machine must have a single config patch
	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachine(resources.DefaultNamespace, m).Metadata(),
			func(machine *omni.ClusterMachine, assertions *assert.Assertions) {
				assertions.NoError(suite.assertLabels(machine, omni.LabelCluster, clusterName, omni.LabelMachineSet, machineSet.Metadata().ID()))
			},
		)
	}

	clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](suite.ctx, suite.state, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())))
	suite.Require().NoError(err)

	clusterMachineList.ForEach(func(clusterMachine *omni.ClusterMachine) {
		_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, clusterMachine.Metadata(), func(r *omni.ClusterMachine) error {
			r.Metadata().Finalizers().Add("test-finalizer")

			return nil
		}, state.WithUpdateOwner(clusterMachine.Metadata().Owner()))
		suite.Require().NoError(err)
	})

	watchCh := make(chan safe.WrappedStateEvent[*omni.ClusterMachine])

	err = safe.StateWatchKind(suite.ctx, suite.state, omni.NewClusterMachine(resources.DefaultNamespace, "").Metadata(), watchCh)
	suite.Require().NoError(err)

	for _, machine := range machines[1:] {
		rtestutils.Destroy[*omni.MachineSetNode](suite.ctx, suite.T(), suite.state, []string{machine})
	}

	getTearingDownClusterMachines := func() []*omni.ClusterMachine {
		list, err := safe.StateListAll[*omni.ClusterMachine](suite.ctx, suite.state, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())))
		suite.Require().NoError(err)

		tearingDown := make([]*omni.ClusterMachine, 0, list.Len())

		list.ForEach(func(clusterMachine *omni.ClusterMachine) {
			if clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
				tearingDown = append(tearingDown, clusterMachine)
			}
		})

		return tearingDown
	}

	expectTearingDownMachines := func(num int) {
		suite.EventuallyWithT(func(collect *assert.CollectT) {
			assert.Equal(collect, num, len(getTearingDownClusterMachines()))
		}, 2*time.Second, 100*time.Millisecond)

		// wait for a second, then ensure that the number has not changed, i.e., we didn't go over the max parallelism and deleted too many at once
		timer := time.NewTimer(1 * time.Second)
		defer timer.Stop()

		select {
		case <-suite.ctx.Done():
			suite.Require().NoError(suite.ctx.Err())
		case <-timer.C:
		}

		suite.Require().Equal(num, len(getTearingDownClusterMachines()))
	}

	unblockAndDestroyTearingDownMachines := func() {
		for _, clusterMachine := range getTearingDownClusterMachines() {
			_, err := safe.StateUpdateWithConflicts[*omni.ClusterMachine](suite.ctx, suite.state, clusterMachine.Metadata(), func(res *omni.ClusterMachine) error {
				res.Metadata().Finalizers().Remove("test-finalizer")

				return nil
			}, state.WithUpdateOwner(clusterMachine.Metadata().Owner()), state.WithExpectedPhaseAny())
			suite.Require().NoError(err)

			rtestutils.AssertNoResource[*omni.ClusterMachine](suite.ctx, suite.T(), suite.state, clusterMachine.Metadata().ID())
		}
	}

	// 1st batch: three machines should be tearing down at once, as max delete parallelism is set to 3
	expectTearingDownMachines(3)

	// unblock the machines by clearing their test finalizer and wait for them to be destroyed
	unblockAndDestroyTearingDownMachines()

	// 2nd batch: three machines should be tearing down at once, as max delete parallelism is set to 3
	expectTearingDownMachines(3)

	// unblock the machines by clearing their test finalizer and wait for them to be destroyed
	unblockAndDestroyTearingDownMachines()

	// 3rd batch: only a single machine should be tearing down, as max delete parallelism is set to 3 and there is only one machine left
	expectTearingDownMachines(1)

	// unblock the machines by clearing their test finalizer and wait for them to be destroyed
	unblockAndDestroyTearingDownMachines()
}

func (suite *MachineSetStatusSuite) TestConfigUpdate() {
	clusterName := "patch-party"

	machines := []string{
		"patched1",
		"patched2",
		"patched3",
	}

	machineSet := suite.createMachineSet(clusterName, "machine-set-configs-update", machines, `machine:
  install:
    disk: /dev/vdb`)

	// initially, each machine should have 2 config patches
	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachine(resources.DefaultNamespace, m).Metadata(),
			func(machine *omni.ClusterMachine, assertions *assert.Assertions) {
				assertions.NoError(suite.assertLabels(machine, omni.LabelCluster, clusterName, omni.LabelMachineSet, machineSet.Metadata().ID()))
			},
		)

		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, m).Metadata(),
			func(res *omni.ClusterMachineConfigPatches, assertions *assert.Assertions) {
				assertions.Len(res.TypedSpec().Value.Patches, 2, "expected to have 2 machine patches in the ClusterMachine")
			},
		)
	}

	suite.assertMachinesState(machines, clusterName, machineSet.Metadata().ID())

	// mark machines except machine[2] as running
	suite.updateStage(machines[:2], specs.ClusterMachineStatusSpec_RUNNING, true)
	suite.syncConfig(machines[:2])

	// create a ClusterMachine-level patch for the running machine[0]
	machinePatch := omni.NewConfigPatch(
		resources.DefaultNamespace,
		machines[0]+"-cluster-machine",
		pair.MakePair(omni.LabelCluster, clusterName),
		pair.MakePair(omni.LabelClusterMachine, machines[0]),
	)

	machinePatch.TypedSpec().Value.Data = `machine:
  network:
   hostname: the-running-node-cluster-machine-patch`

	rtestutils.AssertResource[*omni.MachineSetStatus](suite.ctx, suite.T(), suite.state, machineSet.Metadata().ID(), func(r *omni.MachineSetStatus, assertion *assert.Assertions) {
		assertion.True(
			r.TypedSpec().Value.Machines.EqualVT(
				&specs.Machines{
					Total:     3,
					Healthy:   2,
					Connected: 2,
					Requested: 3,
				},
			),
			"status %#v", r.TypedSpec().Value.Machines,
		)
	})

	suite.Assert().NoError(suite.state.Create(suite.ctx, machinePatch))

	// create a Machine-level patch for the running machine[0]
	machinePatch = omni.NewConfigPatch(
		resources.DefaultNamespace,
		machines[0]+"-machine",
		pair.MakePair(omni.LabelMachine, machines[0]),
	)

	machinePatch.TypedSpec().Value.Data = `machine:
  network:
   hostname: the-running-node-machine-patch`

	suite.Assert().NoError(suite.state.Create(suite.ctx, machinePatch))

	time.Sleep(time.Millisecond * 200)

	// the running machine[0] should still have the old list of patches, because there is a non-running machine[2]
	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, machines[0]).Metadata(),
		func(res *omni.ClusterMachineConfigPatches, assertions *assert.Assertions) {
			assertions.Len(res.TypedSpec().Value.Patches, 2, "the machine was updated while it shouldn't be")
		},
	)

	// create a ClusterMachine-level patch for the machine[2] which is still not ready
	machinePatch = omni.NewConfigPatch(resources.DefaultNamespace, machines[2],
		pair.MakePair(omni.LabelCluster, clusterName),
		pair.MakePair(omni.LabelClusterMachine, machines[2]),
	)

	machinePatch.TypedSpec().Value.Data = `machine:
  network:
    hostname: the-pending-node`

	suite.Assert().NoError(suite.state.Create(suite.ctx, machinePatch))

	// the non-running machine[2] should be updated to have the new patch
	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, machines[2]).Metadata(),
		func(res *omni.ClusterMachineConfigPatches, assertions *assert.Assertions) {
			assertions.Len(res.TypedSpec().Value.Patches, 3, "expected to have 3 machine patches in the ClusterMachine but found %d", len(res.TypedSpec().Value.Patches))
		},
	)

	// mark the non-running machine[2] as running & ready, which should trigger the running machine[0] to be updated
	suite.updateStage(machines[2:], specs.ClusterMachineStatusSpec_RUNNING, true)
	suite.syncConfig(machines[2:])

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, machines[0]).Metadata(),
		func(res *omni.ClusterMachineConfigPatches, assertions *assert.Assertions) {
			patches := res.TypedSpec().Value.Patches
			assertions.Len(patches, 4, "expected to have 4 machine patches in the ClusterMachine but found %d", len(patches))
		},
	)
}

// TestConfigUpdateWithMaxParallelism tests if config updates are batched correctly when max parallelism is set in a worker machine set.
func (suite *MachineSetStatusSuite) TestConfigUpdateWithMaxParallelism() {
	clusterName := "cluster-test-update-max-parallelism"
	numMachines := 8
	machines := make([]string, 0, numMachines)

	for i := range numMachines {
		machines = append(machines, fmt.Sprintf("node-test-update-max-parallelism-%02d", i))
	}

	machineSet := suite.createMachineSetWithOpts(clusterName, "machine-set-test-update-max-parallelism", machines, withMaxUpdateParallelism(3))

	// initially, each machine must have a single config patch
	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachine(resources.DefaultNamespace, m).Metadata(),
			func(machine *omni.ClusterMachine, assertions *assert.Assertions) {
				assertions.NoError(suite.assertLabels(machine, omni.LabelCluster, clusterName, omni.LabelMachineSet, machineSet.Metadata().ID()))
			},
		)

		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, m).Metadata(),
			func(res *omni.ClusterMachineConfigPatches, assertions *assert.Assertions) {
				assertions.Len(res.TypedSpec().Value.Patches, 1, "unexpected number of patches in the ClusterMachine")
			},
		)
	}

	watchCh := make(chan safe.WrappedStateEvent[*omni.ClusterMachineConfigPatches])

	err := safe.StateWatchKind(suite.ctx, suite.state, omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "").Metadata(), watchCh)
	suite.Require().NoError(err)

	suite.assertMachinesState(machines, clusterName, machineSet.Metadata().ID())

	// mark all machines as running
	suite.updateStage(machines, specs.ClusterMachineStatusSpec_RUNNING, true)
	suite.syncConfig(machines)

	machineSetPatch := omni.NewConfigPatch(resources.DefaultNamespace, machineSet.Metadata().ID()+"-patch",
		pair.MakePair(omni.LabelCluster, clusterName), pair.MakePair(omni.LabelMachineSet, machineSet.Metadata().ID()))
	machineSetPatch.TypedSpec().Value.Data = `{"machine":{"env":{"AAA":"BBB"}}}`

	suite.Require().NoError(suite.state.Create(suite.ctx, machineSetPatch))

	rtestutils.AssertResources[*omni.ClusterMachine](suite.ctx, suite.T(), suite.state, machines, func(r *omni.ClusterMachine, assert *assert.Assertions) {
		_, ok := r.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)

		assert.True(ok)
	})

	expectEvents := func(num int) []resource.ID {
		ids := make([]resource.ID, 0, num)

		for range num {
			var event safe.WrappedStateEvent[*omni.ClusterMachineConfigPatches]

			select {
			case <-suite.ctx.Done():
				suite.Require().NoError(suite.ctx.Err())
			case event = <-watchCh:
			}

			var res *omni.ClusterMachineConfigPatches

			res, err = event.Resource()
			suite.Require().NoError(err)

			suite.Len(res.TypedSpec().Value.Patches, 2, "unexpected number of patches in the ClusterMachine")

			ids = append(ids, res.Metadata().ID())
		}

		return ids
	}

	expectNoEvent := func(d time.Duration) {
		timer := time.NewTimer(d)
		defer timer.Stop()

		select {
		case <-suite.ctx.Done():
			suite.Require().NoError(suite.ctx.Err())
		case event := <-watchCh:
			suite.Failf("unexpected event", "got event %s", formatEvent[*omni.ClusterMachineConfigPatches](suite.T(), &event))
		case <-timer.C:
		}
	}

	// expect 3 events, as maxParallelism is 3
	updatedIDs := expectEvents(3)

	// batch complete, do not expect any more events - assert by waiting for 2 seconds
	expectNoEvent(2 * time.Second)

	// sync configs of the machines updated on the 1st batch
	// this will also trigger the next batch, as it updates the ClusterMachineConfigStatuses
	// and they are also inputs for the controller
	suite.syncConfig(updatedIDs)

	// expect 3 more events
	updatedIDs = expectEvents(3)

	// batch complete, do not expect any more events - assert by waiting for 2 seconds
	expectNoEvent(2 * time.Second)

	// trigger the next batch
	suite.syncConfig(updatedIDs)

	// expect only 2 more events, as there are only 2 machines left to update
	expectEvents(2)
}

type event[T resource.Resource] interface {
	Resource() (T, error)
	Old() (T, error)
	Type() state.EventType
	Error() error
}

func formatEvent[T resource.Resource](t *testing.T, event event[T]) string {
	newRes := must.Value(event.Resource())(t)
	oldRes := must.Value(event.Old())(t)
	newResYaml := must.Value(resource.MarshalYAML(newRes))(t)
	oldResYaml := must.Value(resource.MarshalYAML(oldRes))(t)

	return fmt.Sprintf(
		"\ntype:\n%s\n------\nerror:\n%s\n------\nnew:\n%s\n------\nold:\n%s",
		event.Type(),
		event.Error(),
		string(must.Value(yaml.Marshal(newResYaml))(t)),
		string(must.Value(yaml.Marshal(oldResYaml))(t)),
	)
}

func (suite *MachineSetStatusSuite) TestTeardown() {
	machines := []string{
		"n1",
		"n2",
		"n3",
	}

	for _, tt := range []struct {
		setup func(*omni.MachineSet)
		name  string
	}{
		{
			name: "scalingUp",
			setup: func(machineSet *omni.MachineSet) {
				suite.assertMachineSetPhase(machineSet, specs.MachineSetPhase_ScalingUp)
			},
		},
		{
			name: "scalingDown",
			setup: func(machineSet *omni.MachineSet) {
				suite.updateStage(machines, specs.ClusterMachineStatusSpec_RUNNING, true)
				suite.assertMachineSetPhase(machineSet, specs.MachineSetPhase_Running)

				// add a finalizer to ClusterMachine to simulate that machines is being deleted
				suite.Require().NoError(suite.state.AddFinalizer(
					suite.ctx,
					resource.NewMetadata(resources.DefaultNamespace, omni.ClusterMachineType, machines[0], resource.VersionUndefined),
					"test",
				))

				rtestutils.Destroy[*omni.MachineSetNode](suite.ctx, suite.T(), suite.state, []string{machines[0]})

				suite.assertMachineSetPhase(machineSet, specs.MachineSetPhase_ScalingDown)

				suite.Require().NoError(suite.state.RemoveFinalizer(
					suite.ctx,
					resource.NewMetadata(resources.DefaultNamespace, omni.ClusterMachineType, machines[0], resource.VersionUndefined),
					"test",
				))
			},
		},
		{
			name: "ready",
			setup: func(machineSet *omni.MachineSet) {
				suite.updateStage(machines, specs.ClusterMachineStatusSpec_RUNNING, true)
				suite.assertMachineSetPhase(machineSet, specs.MachineSetPhase_Running)
			},
		},
	} {
		if !suite.Run(tt.name, func() {
			clusterName := "teardown"

			machineSet := suite.createMachineSet(clusterName, "machine-set-teardown", machines)
			suite.assertMachinesState(machines, clusterName, machineSet.Metadata().ID())

			tt.setup(machineSet)

			suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				func() error {
					ready, err := suite.state.Teardown(suite.ctx, machineSet.Metadata())
					if err != nil {
						return err
					}

					if !ready {
						return retry.ExpectedErrorf("the machine set is not ready to be destroyed yet")
					}

					return nil
				},
			))

			suite.Assert().NoError(suite.state.Destroy(suite.ctx, machineSet.Metadata()))
			rtestutils.Destroy[*omni.MachineSetNode](suite.ctx, suite.T(), suite.state, machines)
		}) {
			break
		}
	}
}

// TestMachineLocks verifies machine locks block patch updates, deletions, do not block cluster deletion.
func (suite *MachineSetStatusSuite) TestMachineLocks() {
	clusterName := "cluster-locks"

	machines := []string{
		"node01",
		"node02",
		"node03",
	}

	machineSet := suite.createMachineSet(clusterName, "machine-set-locks", machines)

	suite.assertMachinesState(machines, clusterName, machineSet.Metadata().ID())

	suite.updateStage(machines, specs.ClusterMachineStatusSpec_RUNNING, true)

	machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, machines[1], machineSet)

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, machineSetNode.Metadata(), func(ms *omni.MachineSetNode) error {
		ms.Metadata().Annotations().Set(omni.MachineLocked, "")

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResource[*omni.MachineSetStatus](suite.ctx, suite.T(), suite.state, machineSet.Metadata().ID(), func(r *omni.MachineSetStatus, assertion *assert.Assertions) {
		assertion.True(
			r.TypedSpec().Value.Machines.EqualVT(
				&specs.Machines{
					Total:     3,
					Healthy:   3,
					Connected: 3,
					Requested: 3,
				},
			),
			"status %#v", r.TypedSpec().Value.Machines,
		)
	})

	patch := omni.NewConfigPatch(
		resources.DefaultNamespace,
		machineSet.Metadata().ID()+"-patch",
		pair.MakePair(
			omni.LabelCluster, clusterName,
		),
		pair.MakePair(
			omni.LabelMachineSet, machineSet.Metadata().ID(),
		),
	)

	patch.TypedSpec().Value.Data = `cluster:
  allowSchedulingOnControlPlanes: true`

	suite.Assert().NoError(suite.state.Create(suite.ctx, patch))

	var eg errgroup.Group

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer func() {
		cancel()

		suite.Require().NoError(eg.Wait())
	}()

	eg.Go(func() error {
		events := make(chan safe.WrappedStateEvent[*omni.ClusterMachine])

		if e := safe.StateWatchKind(ctx, suite.state, omni.NewClusterMachine(resources.DefaultNamespace, "").Metadata(), events); e != nil {
			return e
		}

		for {
			select {
			case <-ctx.Done():
				return suite.ctx.Err()
			case <-events:
			}

			suite.syncConfig(machines)
		}
	})

	suite.assertMachinePatches(machines, func(machine *omni.ClusterMachineConfigPatches, assertions *assert.Assertions) {
		if machine.Metadata().ID() == machines[1] {
			assertions.Lenf(machine.TypedSpec().Value.Patches, 1, "machine %s is locked but was updated", machine.Metadata().ID())

			return
		}

		assertions.Lenf(machine.TypedSpec().Value.Patches, 2, "machine %s is not locked but was not updated", machine.Metadata().ID())
	})

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSetNode.Metadata(), func(res *omni.MachineSetNode) error {
		res.Metadata().Annotations().Delete(omni.MachineLocked)

		return nil
	})

	suite.Require().NoError(err)

	suite.assertMachinePatches(machines, func(machine *omni.ClusterMachineConfigPatches, assertions *assert.Assertions) {
		assertions.Lenf(machine.TypedSpec().Value.Patches, 2, "machine %s is not locked but was not updated", machine.Metadata().ID())
	})
}

func (suite *MachineSetStatusSuite) assertLabels(res resource.Resource, labels ...string) error {
	if len(labels)%2 != 0 {
		return errors.New("the number of label params must be even")
	}

	for i := 0; i < len(labels); i += 2 {
		value, ok := res.Metadata().Labels().Get(labels[i])
		if !ok {
			return fmt.Errorf("the label %s doesn't exist in the resource %s", labels[i], res.Metadata())
		}

		if labels[i+1] != value {
			return fmt.Errorf("the label %s expected to be %s, got %s", labels[i], labels[i+1], value)
		}
	}

	return nil
}

func (suite *MachineSetStatusSuite) assertMachinesState(machines []string, clusterName, machineSetName string) {
	suite.assertMachines(machines, func(machine *omni.ClusterMachine, assertions *assert.Assertions) {
		assertions.NoError(suite.assertLabels(machine, omni.LabelCluster, clusterName, omni.LabelMachineSet, machineSetName))
	})

	suite.assertMachinePatches(machines, func(res *omni.ClusterMachineConfigPatches, assertions *assert.Assertions) {
		assertions.NotEmpty(res.TypedSpec().Value.Patches, "expected to have machine patches in the ClusterMachine")
	})
}

func (suite *MachineSetStatusSuite) assertMachines(machines []string, check func(machine *omni.ClusterMachine, assertions *assert.Assertions)) {
	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachine(resources.DefaultNamespace, m).Metadata(),
			check,
		)
	}
}

func (suite *MachineSetStatusSuite) assertMachinePatches(machines []string, check func(machine *omni.ClusterMachineConfigPatches, assertions *assert.Assertions)) {
	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, m).Metadata(),
			check,
		)
	}
}

func (suite *MachineSetStatusSuite) assertMachineSetPhase(machineSet *omni.MachineSet, expectedPhase specs.MachineSetPhase) {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []resource.ID{machineSet.Metadata().ID()}, func(machineSet *omni.MachineSetStatus, assert *assert.Assertions) {
		assert.Equal(expectedPhase, machineSet.TypedSpec().Value.Phase)
	})
}

func TestMachineSetStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineSetStatusSuite))
}

type options struct {
	patches              []string
	healthy              bool
	maxUpdateParallelism uint32
	maxDeleteParallelism uint32
}

type option func(*options)

func withPatch(patch string) option {
	return func(o *options) {
		o.patches = append(o.patches, patch)
	}
}

func withHealthy(healthy bool) option {
	return func(o *options) {
		o.healthy = healthy
	}
}

func withMaxUpdateParallelism(maxUpdateParallelism uint32) option {
	return func(o *options) {
		o.maxUpdateParallelism = maxUpdateParallelism
	}
}

func withMaxDeleteParallelism(maxDeleteParallelism uint32) option {
	return func(o *options) {
		o.maxDeleteParallelism = maxDeleteParallelism
	}
}

func initOptions[T any, Fn ~func(*T)](t *T, fns ...Fn) *T {
	for _, fn := range fns {
		fn(t)
	}

	return t
}
