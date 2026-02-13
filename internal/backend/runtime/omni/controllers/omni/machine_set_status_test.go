// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

func registerMachineSetStatusController(t *testing.T) testutils.TestFunc {
	return func(_ context.Context, tc testutils.TestContext) {
		require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewMachineSetStatusController()))
	}
}

// setupMachineSetCluster creates a cluster with a machine set and the given machines using rmock.
// lbHealthy controls whether the LoadBalancerStatus is initially healthy.
// msModify can be used to apply extra modifications to the MachineSet (e.g., rolling strategy config).
func setupMachineSetCluster(
	ctx context.Context, t *testing.T, st state.State,
	clusterName, machineSetName string, machineIDs []string,
	lbHealthy bool, msModify ...options.MockOption,
) *omni.MachineSet {
	cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))

	lbOpts := []options.MockOption{options.SameID(cluster)}
	if !lbHealthy {
		lbOpts = append(lbOpts, options.Modify(func(r *omni.LoadBalancerStatus) error {
			r.TypedSpec().Value.Healthy = false

			return nil
		}))
	}

	rmock.Mock[*omni.LoadBalancerStatus](ctx, t, st, lbOpts...)

	msOpts := make([]options.MockOption, 0, 3+len(msModify))
	msOpts = append(msOpts,
		options.WithID(machineSetName),
		options.LabelCluster(cluster),
		options.Modify(func(r *omni.MachineSet) error {
			r.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling

			return nil
		}),
	)
	msOpts = append(msOpts, msModify...)

	machineSet := rmock.Mock[*omni.MachineSet](ctx, t, st, msOpts...)

	rmock.MockList[*omni.MachineSetNode](ctx, t, st,
		options.IDs(machineIDs),
		options.ItemOptions(
			options.LabelCluster(cluster),
			options.LabelMachineSet(machineSet),
		),
	)

	for _, id := range machineIDs {
		rmock.Mock[*omni.MachineStatus](ctx, t, st, options.WithID(id))
		rmock.Mock[*system.ResourceLabels[*omni.MachineStatus]](ctx, t, st, options.WithID(id))
	}

	return machineSet
}

// mssUpdateStage creates or updates ClusterMachineStatus for each machine as RUNNING and ready.
func mssUpdateStage(ctx context.Context, t *testing.T, st state.State, machineIDs []string) {
	for _, id := range machineIDs {
		rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st,
			options.WithID(id),
			options.Modify(func(res *omni.ClusterMachineStatus) error {
				res.TypedSpec().Value.Stage = specs.ClusterMachineStatusSpec_RUNNING
				res.TypedSpec().Value.Ready = true
				res.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")

				return nil
			}),
		)
	}
}

// mssAssertMachinesState waits until all given machines have ClusterMachine resources with the correct labels.
func mssAssertMachinesState(ctx context.Context, t *testing.T, st state.State, machineIDs []string, clusterName, machineSetName string) {
	rtestutils.AssertResources(ctx, t, st, machineIDs, func(cm *omni.ClusterMachine, a *assert.Assertions) {
		clusterVal, ok := cm.Metadata().Labels().Get(omni.LabelCluster)
		a.True(ok, "LabelCluster missing")
		a.Equal(clusterName, clusterVal)

		msVal, ok := cm.Metadata().Labels().Get(omni.LabelMachineSet)
		a.True(ok, "LabelMachineSet missing")
		a.Equal(machineSetName, msVal)
	})
}

// mssAssertMachineSetPhase waits until the MachineSetStatus has the expected phase.
func mssAssertMachineSetPhase(ctx context.Context, t *testing.T, st state.State, machineSetID string, expectedPhase specs.MachineSetPhase) {
	phaseCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rtestutils.AssertResources(phaseCtx, t, st, []resource.ID{machineSetID}, func(mss *omni.MachineSetStatus, a *assert.Assertions) {
		a.Equal(expectedPhase, mss.TypedSpec().Value.Phase)
	})
}

// TestMachineSetStatus_ScaleUp verifies that scaling the machine set up is never blocked.
func TestMachineSetStatus_ScaleUp(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	clusterName := "scale-up"
	machineSetName := "machine-set-scale-up"
	machines := []string{"node1", "node2", "node3"}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerMachineSetStatusController(t),
		func(ctx context.Context, tc testutils.TestContext) {
			machineSet := setupMachineSetCluster(ctx, t, tc.State, clusterName, machineSetName, machines, true)

			mssAssertMachinesState(ctx, t, tc.State, machines, clusterName, machineSet.Metadata().ID())

			mssUpdateStage(ctx, t, tc.State, machines)

			mssAssertMachineSetPhase(ctx, t, tc.State, machineSet.Metadata().ID(), specs.MachineSetPhase_Running)
		},
	)
}

// TestMachineSetStatus_EmptyTeardown should destroy the machine sets without cluster machines.
func TestMachineSetStatus_EmptyTeardown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerMachineSetStatusController(t),
		func(ctx context.Context, tc testutils.TestContext) {
			cluster := rmock.Mock[*omni.Cluster](ctx, t, tc.State, options.WithID("empty-teardown"))

			ms := omni.NewMachineSet("empty-teardown-ms")
			ms.Metadata().SetPhase(resource.PhaseTearingDown)
			ms.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

			msn := omni.NewMachineSetNode("1", ms)
			msn.Metadata().Finalizers().Add(machineset.ControllerName)

			require.NoError(t, tc.State.Create(ctx, msn))
			require.NoError(t, tc.State.Create(ctx, ms))

			rtestutils.DestroyAll[*omni.MachineSetNode](ctx, t, tc.State)
			rtestutils.DestroyAll[*omni.MachineSet](ctx, t, tc.State)
		},
	)
}

// TestMachineSetStatus_ScaleDown creates a machine set with 3 unhealthy machines, scales down one,
// then makes the rest healthy and verifies the deleted machine is gone.
func TestMachineSetStatus_ScaleDown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	clusterName := "scale-down"
	machineSetName := "machine-set-scale-down"
	machines := []string{"scaledown-1", "scaledown-2", "scaledown-3"}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerMachineSetStatusController(t),
		func(ctx context.Context, tc testutils.TestContext) {
			// Create cluster with LoadBalancer initially unhealthy so the deleted machine can linger
			machineSet := setupMachineSetCluster(ctx, t, tc.State, clusterName, machineSetName, machines, false)

			mssAssertMachinesState(ctx, t, tc.State, machines, clusterName, machineSet.Metadata().ID())

			rtestutils.Destroy[*omni.MachineSetNode](ctx, t, tc.State, []string{machines[0]})

			mssAssertMachineSetPhase(ctx, t, tc.State, machineSet.Metadata().ID(), specs.MachineSetPhase_ScalingUp)

			expectedMachines := []string{"scaledown-2", "scaledown-3"}

			mssAssertMachinesState(ctx, t, tc.State, expectedMachines, clusterName, machineSet.Metadata().ID())

			mssUpdateStage(ctx, t, tc.State, expectedMachines)

			_, err := safe.StateUpdateWithConflicts(ctx, tc.State, omni.NewLoadBalancerStatus(clusterName).Metadata(), func(r *omni.LoadBalancerStatus) error {
				r.TypedSpec().Value.Healthy = true

				return nil
			})
			require.NoError(t, err)

			require.NoError(t, retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
				_, err := tc.State.Get(ctx, omni.NewClusterMachine(machines[0]).Metadata())
				if err == nil {
					return retry.ExpectedErrorf("ClusterMachine %s still exists", machines[0])
				}

				if state.IsNotFoundError(err) {
					return nil
				}

				return err
			}))

			mssAssertMachineSetPhase(ctx, t, tc.State, machineSet.Metadata().ID(), specs.MachineSetPhase_Running)
		},
	)
}

// TestMachineSetStatus_ScaleDownWithMaxParallelism tests scale down respects max delete parallelism.
func TestMachineSetStatus_ScaleDownWithMaxParallelism(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	clusterName := "cluster-test-scale-down-max-parallelism"
	machineSetName := "machine-set-test-scale-down-max-parallelism"
	numMachines := 8
	machines := make([]string, numMachines)

	for i := range numMachines {
		machines[i] = fmt.Sprintf("node-test-scale-down-max-parallelism-%02d", i)
	}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerMachineSetStatusController(t),
		func(ctx context.Context, tc testutils.TestContext) {
			machineSet := setupMachineSetCluster(ctx, t, tc.State, clusterName, machineSetName, machines, true,
				options.Modify(func(r *omni.MachineSet) error {
					r.TypedSpec().Value.DeleteStrategy = specs.MachineSetSpec_Rolling
					r.TypedSpec().Value.DeleteStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
						Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
							MaxParallelism: 3,
						},
					}

					return nil
				}),
			)

			// wait for all cluster machines to be created
			mssAssertMachinesState(ctx, t, tc.State, machines, clusterName, machineSet.Metadata().ID())

			// add a test finalizer to each cluster machine to prevent actual deletion
			clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](ctx, tc.State,
				state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())),
			)
			require.NoError(t, err)

			clusterMachineList.ForEach(func(cm *omni.ClusterMachine) {
				_, err = safe.StateUpdateWithConflicts(ctx, tc.State, cm.Metadata(), func(r *omni.ClusterMachine) error {
					r.Metadata().Finalizers().Add("test-finalizer")

					return nil
				}, state.WithUpdateOwner(cm.Metadata().Owner()))
				require.NoError(t, err)
			})

			// destroy all MachineSetNodes except machines[0]
			for _, machine := range machines[1:] {
				rtestutils.Destroy[*omni.MachineSetNode](ctx, t, tc.State, []string{machine})
			}

			getTearingDownClusterMachines := func() []*omni.ClusterMachine {
				list, err := safe.StateListAll[*omni.ClusterMachine](ctx, tc.State,
					state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())),
				)
				require.NoError(t, err)

				var tearingDown []*omni.ClusterMachine

				list.ForEach(func(cm *omni.ClusterMachine) {
					if cm.Metadata().Phase() == resource.PhaseTearingDown {
						tearingDown = append(tearingDown, cm)
					}
				})

				return tearingDown
			}

			expectTearingDownMachines := func(num int) {
				assert.EventuallyWithT(t, func(collect *assert.CollectT) {
					assert.Equal(collect, num, len(getTearingDownClusterMachines()))
				}, 10*time.Second, 100*time.Millisecond)

				// wait a second then verify count hasn't grown beyond the limit
				timer := time.NewTimer(1 * time.Second)
				defer timer.Stop()

				select {
				case <-ctx.Done():
					require.NoError(t, ctx.Err())
				case <-timer.C:
				}

				require.Equal(t, num, len(getTearingDownClusterMachines()))
			}

			unblockAndDestroyTearingDownMachines := func() {
				for _, cm := range getTearingDownClusterMachines() {
					_, err := safe.StateUpdateWithConflicts[*omni.ClusterMachine](ctx, tc.State, cm.Metadata(), func(res *omni.ClusterMachine) error {
						res.Metadata().Finalizers().Remove("test-finalizer")

						return nil
					}, state.WithUpdateOwner(cm.Metadata().Owner()), state.WithExpectedPhaseAny())
					require.NoError(t, err)

					rtestutils.AssertNoResource[*omni.ClusterMachine](ctx, t, tc.State, cm.Metadata().ID())
				}
			}

			// 1st batch: 3 machines should be tearing down at once
			expectTearingDownMachines(3)
			unblockAndDestroyTearingDownMachines()

			// 2nd batch: 3 machines
			expectTearingDownMachines(3)
			unblockAndDestroyTearingDownMachines()

			// 3rd batch: 1 machine remaining
			expectTearingDownMachines(1)
			unblockAndDestroyTearingDownMachines()
		},
	)
}

// TestMachineSetStatus_Teardown verifies that a machine set can be torn down from various states.
func TestMachineSetStatus_Teardown(t *testing.T) {
	t.Parallel()

	machines := []string{"n1", "n2", "n3"}

	for _, tt := range []struct {
		setup func(ctx context.Context, t *testing.T, st state.State, machineSet *omni.MachineSet)
		name  string
	}{
		{
			name: "scalingUp",
			setup: func(ctx context.Context, t *testing.T, st state.State, machineSet *omni.MachineSet) {
				mssAssertMachineSetPhase(ctx, t, st, machineSet.Metadata().ID(), specs.MachineSetPhase_ScalingUp)
			},
		},
		{
			name: "scalingDown",
			setup: func(ctx context.Context, t *testing.T, st state.State, machineSet *omni.MachineSet) {
				mssUpdateStage(ctx, t, st, machines)
				mssAssertMachineSetPhase(ctx, t, st, machineSet.Metadata().ID(), specs.MachineSetPhase_Running)

				// add a finalizer to ClusterMachine to simulate a machine being deleted
				require.NoError(t, st.AddFinalizer(
					ctx,
					resource.NewMetadata(omni.NewClusterMachine("").ResourceDefinition().DefaultNamespace, omni.ClusterMachineType, machines[0], resource.VersionUndefined),
					"test",
				))

				rtestutils.Destroy[*omni.MachineSetNode](ctx, t, st, []string{machines[0]})

				mssAssertMachineSetPhase(ctx, t, st, machineSet.Metadata().ID(), specs.MachineSetPhase_ScalingDown)

				require.NoError(t, st.RemoveFinalizer(
					ctx,
					resource.NewMetadata(omni.NewClusterMachine("").ResourceDefinition().DefaultNamespace, omni.ClusterMachineType, machines[0], resource.VersionUndefined),
					"test",
				))
			},
		},
		{
			name: "ready",
			setup: func(ctx context.Context, t *testing.T, st state.State, machineSet *omni.MachineSet) {
				mssUpdateStage(ctx, t, st, machines)
				mssAssertMachineSetPhase(ctx, t, st, machineSet.Metadata().ID(), specs.MachineSetPhase_Running)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerMachineSetStatusController(t),
				func(ctx context.Context, tc testutils.TestContext) {
					machineSet := setupMachineSetCluster(ctx, t, tc.State, "teardown", "machine-set-teardown", machines, true)

					mssAssertMachinesState(ctx, t, tc.State, machines, "teardown", machineSet.Metadata().ID())

					tt.setup(ctx, t, tc.State, machineSet)

					require.NoError(t, retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
						ready, err := tc.State.Teardown(ctx, machineSet.Metadata())
						if err != nil {
							return err
						}

						if !ready {
							return retry.ExpectedErrorf("machine set is not ready to be destroyed yet")
						}

						return nil
					}))

					require.NoError(t, tc.State.Destroy(ctx, machineSet.Metadata()))
					rtestutils.Destroy[*omni.MachineSetNode](ctx, t, tc.State, machines)
				},
			)
		})
	}
}

// TestMachineSetStatus_ClusterLocks verifies that a locked cluster prevents ClusterMachine deletion.
func TestMachineSetStatus_ClusterLocks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	clusterName := "cluster-locks"
	machineSetName := "machine-set-locks"
	machines := []string{"node01", "node02", "node03"}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerMachineSetStatusController(t),
		func(ctx context.Context, tc testutils.TestContext) {
			machineSet := setupMachineSetCluster(ctx, t, tc.State, clusterName, machineSetName, machines, true)

			mssAssertMachinesState(ctx, t, tc.State, machines, clusterName, machineSet.Metadata().ID())

			mssUpdateStage(ctx, t, tc.State, machines)

			rtestutils.AssertResource(ctx, t, tc.State, machineSet.Metadata().ID(), func(r *omni.MachineSetStatus, a *assert.Assertions) {
				a.True(r.TypedSpec().Value.Machines.EqualVT(&specs.Machines{
					Total:     3,
					Healthy:   3,
					Connected: 3,
					Requested: 3,
				}), "status %#v", r.TypedSpec().Value.Machines)
			})

			_, err := safe.StateUpdateWithConflicts(ctx, tc.State, omni.NewCluster(clusterName).Metadata(), func(res *omni.Cluster) error {
				res.Metadata().Annotations().Set(omni.ClusterLocked, "")

				return nil
			})
			require.NoError(t, err)

			// The controller reads cluster state from its cache. After setting the lock annotation the
			// cache may not reflect the change yet, so the controller could reconcile a MachineSetNode
			// teardown with a stale (unlocked) view of the cluster and delete the ClusterMachine.
			// Since SkipReconcileTag produces no observable state change, a sleep is the only practical
			// way to ensure the controller has processed a full reconcile cycle with the locked cluster
			// before we issue the MachineSetNode deletion.
			time.Sleep(500 * time.Millisecond)

			rtestutils.Destroy[*omni.MachineSetNode](ctx, t, tc.State, []string{machines[0]})

			// Verify the lock prevented deletion. When locked, the controller returns SkipReconcileTag
			// producing no observable state change, so AssertResources is the best signal available.
			rtestutils.AssertResources(ctx, t, tc.State, []string{machines[0]}, func(*omni.ClusterMachine, *assert.Assertions) {})

			_, err = safe.StateUpdateWithConflicts(ctx, tc.State, omni.NewCluster(clusterName).Metadata(), func(res *omni.Cluster) error {
				res.Metadata().Annotations().Delete(omni.ClusterLocked)

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertNoResource[*omni.ClusterMachine](ctx, t, tc.State, machines[0])
		},
	)
}

// TestMachineSetStatus_MachineIsAddedToAnotherMachineSet verifies that a machine moved between
// machine sets correctly picks up the new machine set label.
func TestMachineSetStatus_MachineIsAddedToAnotherMachineSet(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	clusterName := "cluster-machine-move"
	machines := []string{"node01"}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerMachineSetStatusController(t),
		func(ctx context.Context, tc testutils.TestContext) {
			setupMachineSetCluster(ctx, t, tc.State, clusterName, "machine-set-1", machines, true)

			rtestutils.AssertResources(ctx, t, tc.State, machines, func(*omni.ClusterMachine, *assert.Assertions) {})

			// add a finalizer to ClusterMachine so the controller cannot immediately destroy it
			_, err := safe.StateUpdateWithConflicts(ctx, tc.State, omni.NewClusterMachine("node01").Metadata(), func(
				res *omni.ClusterMachine,
			) error {
				res.Metadata().Finalizers().Add("locked")

				return nil
			}, state.WithUpdateOwner(machineset.ControllerName))
			require.NoError(t, err)

			rtestutils.DestroyAll[*omni.MachineSetNode](ctx, t, tc.State)
			rtestutils.DestroyAll[*omni.MachineStatus](ctx, t, tc.State)
			rtestutils.DestroyAll[*system.ResourceLabels[*omni.MachineStatus]](ctx, t, tc.State)

			// create a second machine set with the same machine
			cluster2 := rmock.Mock[*omni.Cluster](ctx, t, tc.State, options.WithID(clusterName))

			rmock.Mock[*omni.LoadBalancerStatus](ctx, t, tc.State, options.WithID(clusterName))

			machineSet2 := rmock.Mock[*omni.MachineSet](ctx, t, tc.State,
				options.WithID("machine-set-2"),
				options.LabelCluster(cluster2),
				options.Modify(func(r *omni.MachineSet) error {
					r.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling

					return nil
				}),
			)

			rmock.Mock[*omni.MachineSetNode](ctx, t, tc.State,
				options.WithID("node01"),
				options.LabelCluster(cluster2),
				options.LabelMachineSet(machineSet2),
			)
			rmock.Mock[*omni.MachineStatus](ctx, t, tc.State, options.WithID("node01"))
			rmock.Mock[*system.ResourceLabels[*omni.MachineStatus]](ctx, t, tc.State, options.WithID("node01"))

			// remove the finalizer so the controller can reconcile
			_, err = safe.StateUpdateWithConflicts(ctx, tc.State, omni.NewClusterMachine("node01").Metadata(), func(
				res *omni.ClusterMachine,
			) error {
				res.Metadata().Finalizers().Remove("locked")

				return nil
			}, state.WithUpdateOwner(machineset.ControllerName), state.WithExpectedPhaseAny())
			require.NoError(t, err)

			rtestutils.AssertResources(ctx, t, tc.State, machines, func(cm *omni.ClusterMachine, a *assert.Assertions) {
				machineSetName, ok := cm.Metadata().Labels().Get(omni.LabelMachineSet)
				a.True(ok)
				a.Equal("machine-set-2", machineSetName)
			})
		},
	)
}
