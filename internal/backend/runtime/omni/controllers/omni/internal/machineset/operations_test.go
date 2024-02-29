// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
)

var logger *zap.Logger

type runtime struct {
	state.State
}

func (rt *runtime) Create(ctx context.Context, r resource.Resource) error {
	return rt.State.Create(ctx, r)
}

func (rt *runtime) Destroy(ctx context.Context, r resource.Pointer, _ ...controller.Option) error {
	return rt.State.Destroy(ctx, r)
}

func (rt *runtime) Modify(ctx context.Context, r resource.Resource, f func(resource.Resource) error) error {
	_, err := rt.ModifyWithResult(ctx, r, f)

	return err
}

func (rt *runtime) Update(ctx context.Context, r resource.Resource) error {
	return rt.State.Update(ctx, r)
}

func (rt *runtime) ModifyWithResult(ctx context.Context, r resource.Resource, f func(resource.Resource) error) (resource.Resource, error) {
	res, err := rt.State.UpdateWithConflicts(ctx, r.Metadata(), f)
	if state.IsNotFoundError(err) {
		res = r.DeepCopy()

		if err = f(res); err != nil {
			return nil, err
		}

		return res, rt.State.Create(ctx, res)
	}

	return res, err
}

func (rt *runtime) Teardown(ctx context.Context, r resource.Pointer, _ ...controller.Option) (bool, error) {
	return rt.State.Teardown(ctx, r)
}

func createRuntime() *runtime {
	state := state.WrapCore(namespaced.NewState(inmem.Build))

	return &runtime{state}
}

// TestCreate runs create once, checks that both cluster machine and cluster machine config status resources were created.
// Validate that both resources have the right data in the spec.
func TestCreate(t *testing.T) {
	rt := createRuntime()

	cluster := omni.NewCluster(resources.DefaultNamespace, "test")
	cluster.TypedSpec().Value.KubernetesVersion = "v1.6.2"

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "machineset")

	create := machineset.Create{ID: "aa"}

	require := require.New(t)

	ctx := context.Background()

	patch := omni.NewConfigPatch(resources.DefaultNamespace, "some")
	patch.TypedSpec().Value.Data = `machine:
  network:
    kubespan:
      enabled: true`

	rc, err := machineset.NewReconciliationContext(
		cluster,
		machineSet,
		newHealthyLB(cluster.Metadata().ID()),
		&fakePatchHelper{
			patches: map[string][]*omni.ConfigPatch{
				"aa": {
					patch,
				},
			},
		},
		[]*omni.MachineSetNode{
			omni.NewMachineSetNode(resources.DefaultNamespace, "aa", machineSet),
		},
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(err)

	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "aa")

	helpers.UpdateInputsVersions(clusterMachine, patch)

	inputsSHA, ok := clusterMachine.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
	require.True(ok)

	require.NoError(create.Apply(ctx, rt, logger, rc))

	clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, rt, "aa")

	actualInputsSHA, ok := clusterMachine.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
	require.True(ok)
	require.Equal(inputsSHA, actualInputsSHA)

	require.NoError(err)

	require.NotEmpty(clusterMachine.TypedSpec().Value.KubernetesVersion)

	clusterMachineConfigPatches, err := safe.ReaderGetByID[*omni.ClusterMachineConfigPatches](ctx, rt, "aa")

	require.NoError(err)

	require.NotEmpty(clusterMachineConfigPatches.TypedSpec().Value.Patches)
}

// TestUpdate run update 4 times:
// - quota is 2, the machine has 1 patch.
// - quota is 1, the machine should have 2 patches, verify that config patches resource was synced.
// - quota is 0, the machine is still updating, so the update should work.
// - quota is 0, the machine config status is synced, no update should happen.
func TestUpdate(t *testing.T) {
	rt := createRuntime()

	cluster := omni.NewCluster(resources.DefaultNamespace, "test")
	cluster.TypedSpec().Value.KubernetesVersion = "v1.6.4"

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "machineset")

	quota := &machineset.ChangeQuota{
		Update: 2,
	}

	update := machineset.Update{ID: "aa", Quota: quota}

	require := require.New(t)

	ctx := context.Background()

	patch1 := omni.NewConfigPatch(resources.DefaultNamespace, "some")
	patch1.TypedSpec().Value.Data = `machine:
  network:
    kubespan:
      enabled: true`

	patch2 := omni.NewConfigPatch(resources.DefaultNamespace, "some")
	patch2.TypedSpec().Value.Data = `machine:
  network:
    hostname: some`

	patchHelper := &fakePatchHelper{
		patches: map[string][]*omni.ConfigPatch{
			"aa": {
				patch1,
			},
		},
	}

	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "aa")
	clusterMachine.Metadata().SetVersion(resource.VersionUndefined.Next())

	configStatus := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "aa")
	configStatus.TypedSpec().Value.ClusterMachineVersion = clusterMachine.Metadata().Version().String()

	var (
		rc  *machineset.ReconciliationContext
		err error
	)

	updateReconciliationContext := func() {
		rc, err = machineset.NewReconciliationContext(
			cluster,
			machineSet,
			newHealthyLB(cluster.Metadata().ID()),
			patchHelper,
			[]*omni.MachineSetNode{
				omni.NewMachineSetNode(resources.DefaultNamespace, "aa", machineSet),
			},
			[]*omni.ClusterMachine{
				clusterMachine,
			},
			[]*omni.ClusterMachineConfigStatus{
				configStatus,
			},
			[]*omni.ClusterMachineConfigPatches{
				omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "aa"),
			},
			nil,
		)
		require.NoError(err)
	}

	updateReconciliationContext()

	require.NoError(update.Apply(ctx, rt, logger, rc))

	require.Equal(1, quota.Update)

	patchHelper.patches["aa"] = append(patchHelper.patches["aa"], patch2)

	updateReconciliationContext()

	clusterMachine = omni.NewClusterMachine(resources.DefaultNamespace, "aa")
	helpers.UpdateInputsVersions(clusterMachine, patch1, patch2)

	inputsSHA, ok := clusterMachine.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
	require.True(ok)

	require.NoError(update.Apply(ctx, rt, logger, rc))

	require.Equal(0, quota.Update)

	clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, rt, "aa")

	actualInputsSHA, ok := clusterMachine.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
	require.True(ok)
	require.Equal(inputsSHA, actualInputsSHA)

	require.NoError(err)

	require.NotEmpty(clusterMachine.TypedSpec().Value.KubernetesVersion)

	clusterMachineConfigPatches, err := safe.ReaderGetByID[*omni.ClusterMachineConfigPatches](ctx, rt, "aa")

	require.NoError(err)

	require.NotEmpty(clusterMachineConfigPatches.TypedSpec().Value.Patches)

	patchHelper.patches["aa"] = append(patchHelper.patches["aa"], patch1)

	updateReconciliationContext()

	clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, rt, "aa")
	version := clusterMachine.Metadata().Version()

	// update should happen as the machine update is still pending
	require.NoError(update.Apply(ctx, rt, logger, rc))

	clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, rt, "aa")
	require.False(clusterMachine.Metadata().Version().Equal(version))

	version = clusterMachine.Metadata().Version()

	// simulate config synced
	configStatus.TypedSpec().Value.ClusterMachineVersion = clusterMachine.Metadata().Version().String()

	updateReconciliationContext()

	// update shouldn't happen as the quota reached
	require.NoError(update.Apply(ctx, rt, logger, rc))

	clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, rt, "aa")
	require.True(clusterMachine.Metadata().Version().Equal(version))
}

// TestTeardown create 2 cluster machine, destroy with quota 1, first should proceed, second should skip.
func TestTeardown(t *testing.T) {
	rt := createRuntime()

	cluster := omni.NewCluster(resources.DefaultNamespace, "test")
	cluster.TypedSpec().Value.KubernetesVersion = "v1.6.3"

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "machineset")

	quota := machineset.ChangeQuota{
		Teardown: 1,
	}

	require := require.New(t)

	ctx := context.Background()

	clusterMachines := []*omni.ClusterMachine{
		omni.NewClusterMachine(resources.DefaultNamespace, "aa"),
		omni.NewClusterMachine(resources.DefaultNamespace, "bb"),
	}

	for _, cm := range clusterMachines {
		cm.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
		cm.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())

		require.NoError(rt.Create(ctx, cm))
	}

	rc, err := machineset.NewReconciliationContext(
		cluster,
		machineSet,
		newHealthyLB(cluster.Metadata().ID()),
		&fakePatchHelper{},
		[]*omni.MachineSetNode{
			omni.NewMachineSetNode(resources.DefaultNamespace, "aa", machineSet),
			omni.NewMachineSetNode(resources.DefaultNamespace, "bb", machineSet),
		},
		clusterMachines,
		nil,
		nil,
		nil,
	)
	require.NoError(err)

	teardown := machineset.Teardown{ID: "aa", Quota: &quota}
	require.NoError(teardown.Apply(ctx, rt, logger, rc))

	require.Equal(0, quota.Teardown)

	teardown = machineset.Teardown{ID: "bb", Quota: &quota}
	require.NoError(teardown.Apply(ctx, rt, logger, rc))

	require.Equal(0, quota.Teardown)

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, rt, "aa")

	require.NoError(err)
	require.Equal(resource.PhaseTearingDown, clusterMachine.Metadata().Phase())

	clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, rt, "bb")

	require.NoError(err)
	require.Equal(resource.PhaseRunning, clusterMachine.Metadata().Phase())
}

// TestDestroy create tearing down machines, destroy them, should have no resources after the operation is complete.
func TestDestroy(t *testing.T) {
	rt := createRuntime()

	cluster := omni.NewCluster(resources.DefaultNamespace, "test")
	cluster.TypedSpec().Value.KubernetesVersion = "v1.6.3"

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "machineset")

	require := require.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	clusterMachines := []*omni.ClusterMachine{
		omni.NewClusterMachine(resources.DefaultNamespace, "aa"),
		omni.NewClusterMachine(resources.DefaultNamespace, "bb"),
	}

	for _, cm := range clusterMachines {
		cm.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
		cm.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())

		cmcp := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, cm.Metadata().ID())
		helpers.CopyAllLabels(cm, cmcp)

		machine := omni.NewMachine(resources.DefaultNamespace, cm.Metadata().ID())
		machine.Metadata().Finalizers().Add(machineset.ControllerName)

		require.NoError(rt.Create(ctx, cm))
		require.NoError(rt.Create(ctx, cmcp))
		require.NoError(rt.Create(ctx, machine))

		_, err := rt.Teardown(ctx, cm.Metadata())
		require.NoError(err)

		cm.Metadata().SetPhase(resource.PhaseTearingDown)
	}

	rc, err := machineset.NewReconciliationContext(
		cluster,
		machineSet,
		newHealthyLB(cluster.Metadata().ID()),
		&fakePatchHelper{},
		nil,
		clusterMachines,
		nil,
		nil,
		nil,
	)
	require.NoError(err)

	destroy := machineset.Destroy{ID: "aa"}
	require.NoError(destroy.Apply(ctx, rt, logger, rc))

	destroy = machineset.Destroy{ID: "bb"}
	require.NoError(destroy.Apply(ctx, rt, logger, rc))

	rtestutils.AssertNoResource[*omni.ClusterMachine](ctx, t, rt.State, "aa")
	rtestutils.AssertNoResource[*omni.ClusterMachine](ctx, t, rt.State, "bb")
	rtestutils.AssertNoResource[*omni.ClusterMachineConfigPatches](ctx, t, rt.State, "aa")
	rtestutils.AssertNoResource[*omni.ClusterMachineConfigPatches](ctx, t, rt.State, "bb")
	rtestutils.AssertResources(ctx, t, rt.State, []string{"aa", "bb"}, func(r *omni.Machine, assertion *assert.Assertions) {
		assertion.True(r.Metadata().Finalizers().Empty())
	})
}

func init() {
	var err error

	logger, err = zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
}

func init() {
	var err error

	logger, err = zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
}
