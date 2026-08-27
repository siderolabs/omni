// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

// TestUpgradeBlockedByOwnHealth pins the recovery-bypass predicate corner by corner, in particular
// that maintenance machines never take the bypass: their not-readiness is inherent, so fresh installs
// and machines sent back to maintenance keep respecting the cluster-health quota.
func TestUpgradeBlockedByOwnHealth(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		status  *machine.MachineStatusEvent_MachineStatus
		name    string
		stage   machine.MachineStatusEvent_MachineStage
		blocked bool
	}{
		{name: "runningReady", stage: machine.MachineStatusEvent_RUNNING, status: &machine.MachineStatusEvent_MachineStatus{Ready: true}, blocked: false},
		{name: "runningNotReady", stage: machine.MachineStatusEvent_RUNNING, status: &machine.MachineStatusEvent_MachineStatus{Ready: false}, blocked: true},
		{name: "booting", stage: machine.MachineStatusEvent_BOOTING, status: &machine.MachineStatusEvent_MachineStatus{Ready: false}, blocked: true},
		{name: "maintenance", stage: machine.MachineStatusEvent_MAINTENANCE, blocked: false},
		{name: "unknown", stage: machine.MachineStatusEvent_UNKNOWN, blocked: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			snapshot := omni.NewMachineStatusSnapshot("test-machine")
			snapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{Stage: tt.stage, Status: tt.status}

			assert.Equal(t, tt.blocked, machineconfig.UpgradeBlockedByOwnHealthForTest(snapshot))
		})
	}
}

// TestClusterLifecycleRecoversUnhealthyMachine verifies that a pending update reaches a machine that is
// itself unhealthy, which is its only way to become healthy again (siderolabs/omni#3262).
//
// Both checks that used to block this are exercised: the stage check (the machine is stuck in the booting
// stage after rebooting into a bad schematic) and the rollout quota (the published quota is zero because
// the machine's own not-readiness is subtracted from it). The recovery must go through anyway, bounded
// only by the upgrade finalizer count.
func TestClusterLifecycleRecoversUnhealthyMachine(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		// schematicID / talosVersion model the revert the user makes to fix the machine: a schematic
		// change (kernel args / extensions) or a Talos version change.
		schematicID  string
		talosVersion string
		stage        machine.MachineStatusEvent_MachineStage
	}{
		{name: "bootingSchematicRevert", stage: machine.MachineStatusEvent_BOOTING, schematicID: "bbbb"},
		// A real cluster-wide version revert enters the Reverting phase, which publishes the quota
		// without the not-ready subtraction; the zero quota here is synthetic. The case still proves
		// that a booting machine no longer blocks a version operation.
		{name: "bootingVersionChange", stage: machine.MachineStatusEvent_BOOTING, talosVersion: "1.13.6"},
		{name: "runningNotReadySchematicRevert", stage: machine.MachineStatusEvent_RUNNING, schematicID: "bbbb"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
			t.Cleanup(cancel)

			provider := &testutils.FakeKubernetesProvider{}

			testutils.WithRuntime(
				ctx, t, testutils.TestOptions{},
				func(_ context.Context, tc testutils.TestContext) {
					require.NoError(t, tc.Runtime.RegisterQController(
						machineconfig.NewStatusController(testutils.NewLifecycleManager(t, tc.State, provider)),
					))
				},
				func(ctx context.Context, tc testutils.TestContext) {
					st := tc.State

					clusterName := "recovery-" + tt.name

					machineServices := testutils.NewMachineServices(t, st)

					_, machines := createCluster(ctx, t, st, machineServices, clusterName, 1, 0)
					id := machines[0].Metadata().ID()

					// Wait for the initial config to be applied so the upgrade lock logic engages.
					rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
						a.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256)
					})

					nodeName := "node-" + id

					provider.SetClientset(testutils.NewFakeKubernetesClientset(nodeName))

					rmock.Mock[*omni.ClusterMachineIdentity](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.ClusterMachineIdentity) error {
						res.TypedSpec().Value.Nodename = nodeName

						return nil
					}))

					const bootID = "boot-recovery"

					// The machine is unhealthy: stuck in the booting stage, or running but not ready.
					// Either way it stays connected to Omni.
					rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.MachineStatusSnapshot) error {
						res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
							Stage:  tt.stage,
							Status: &machine.MachineStatusEvent_MachineStatus{Ready: false},
						}
						res.TypedSpec().Value.BootId = bootID

						return nil
					}))

					// The rollout quota reflects the machine's own not-readiness: zero for its machine set,
					// exactly what the upgrade rollout controller would publish for this cluster.
					machineSetName, ok := machines[0].Metadata().Labels().Get(omni.LabelMachineSet)
					require.True(t, ok)

					rmock.Mock[*omni.UpgradeRollout](ctx, t, st, options.WithID(clusterName), options.Modify(func(res *omni.UpgradeRollout) error {
						res.TypedSpec().Value.MachineSetsUpgradeQuota = map[string]int32{machineSetName: 0}

						return nil
					}))

					// The fixing change arrives: the desired install image differs from what the machine runs.
					rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.MachineConfigGenOptions) error {
						if tt.schematicID != "" {
							res.TypedSpec().Value.InstallImage.SchematicId = tt.schematicID
						}

						if tt.talosVersion != "" {
							res.TypedSpec().Value.InstallImage.TalosVersion = tt.talosVersion
						}

						return nil
					}))

					// The machine receives the lifecycle upgrade despite being unhealthy and despite the
					// zero quota, and the reboot is armed (the boot ID is recorded).
					rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
						a.Equal(bootID, res.TypedSpec().Value.PreRebootBootId)
					})

					require.NotEmpty(t, machineServices.Get(id).GetLifecycleUpgradeRequests())
					assert.Empty(t, machineServices.Get(id).GetUpgradeRequests(), "the machine must not receive a legacy MachineService.Upgrade")
				},
			)
		})
	}
}

// TestUpgradeQuotaStillBlocksHealthyMachine verifies that the recovery bypass does not weaken the rollout
// safety for healthy machines: a ready machine with a pending update still waits when the published quota
// is zero (some other machine in the cluster is broken).
func TestUpgradeQuotaStillBlocksHealthyMachine(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
	t.Cleanup(cancel)

	provider := &testutils.FakeKubernetesProvider{}

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(
				machineconfig.NewStatusController(testutils.NewLifecycleManager(t, tc.State, provider)),
			))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			clusterName := "quota-blocks-healthy"

			machineServices := testutils.NewMachineServices(t, st)

			_, machines := createCluster(ctx, t, st, machineServices, clusterName, 1, 0)
			id := machines[0].Metadata().ID()

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256)
			})

			nodeName := "node-" + id

			provider.SetClientset(testutils.NewFakeKubernetesClientset(nodeName))

			rmock.Mock[*omni.ClusterMachineIdentity](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.ClusterMachineIdentity) error {
				res.TypedSpec().Value.Nodename = nodeName

				return nil
			}))

			// The machine is healthy: running, ready and connected.
			rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.MachineStatusSnapshot) error {
				res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
					Stage:  machine.MachineStatusEvent_RUNNING,
					Status: &machine.MachineStatusEvent_MachineStatus{Ready: true},
				}
				res.TypedSpec().Value.BootId = "boot-healthy"

				return nil
			}))

			machineSetName, ok := machines[0].Metadata().Labels().Get(omni.LabelMachineSet)
			require.True(t, ok)

			// No free quota: some other machine in the cluster is not ready.
			rmock.Mock[*omni.UpgradeRollout](ctx, t, st, options.WithID(clusterName), options.Modify(func(res *omni.UpgradeRollout) error {
				res.TypedSpec().Value.MachineSetsUpgradeQuota = map[string]int32{machineSetName: 0}

				return nil
			}))

			rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.MachineConfigGenOptions) error {
				res.TypedSpec().Value.InstallImage.SchematicId = "bbbb"

				return nil
			}))

			// From here on every pass of this machine has an upgrade due, so the barrier below pins a
			// pass that was actually blocked, not an early one with nothing to do. Do not remove: the
			// whole causality argument below depends on this checkpoint.
			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.MachinePendingUpdates, a *assert.Assertions) {
				a.NotNil(res.TypedSpec().Value.Upgrade)
			})

			// Force one more full pass and prove the previous one completed its slot decision: the
			// config change below flows into the pending update's config diff, which is written at
			// the start of every pass, before the slot decision, and the passes of one machine are
			// serialized. Once the diff is visible, any upgrade wrongly issued under the exhausted
			// quota has already reached the machine mock. The chain holds because nothing in this
			// fixture can make a pass return between the diff write and the slot decision (no machine
			// lock, no config generation error, no reboot marker, node name and install image already
			// resolved), and the positive at the end proves the pipeline was otherwise ready.
			rmock.Mock[*omni.ClusterMachineConfig](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.ClusterMachineConfig) error {
				return res.TypedSpec().Value.SetUncompressedData([]byte("machine:\n  network:\n    kubespan:\n      enabled: true"))
			}))

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.MachinePendingUpdates, a *assert.Assertions) {
				data, err := res.TypedSpec().Value.GetUncompressedData()
				if !a.NoError(err) {
					return
				}

				defer data.Free()

				a.Contains(string(data.Data()), "kubespan")
			})

			assert.Empty(t, machineServices.Get(id).GetLifecycleUpgradeRequests(), "a healthy machine must wait for free quota")

			// Freeing the quota lets the machine proceed, proving the wait above was the quota and
			// nothing else. A recorded pre-reboot boot ID means the lifecycle upgrade ran (the legacy
			// path clears it).
			rmock.Mock[*omni.UpgradeRollout](ctx, t, st, options.WithID(clusterName), options.Modify(func(res *omni.UpgradeRollout) error {
				res.TypedSpec().Value.MachineSetsUpgradeQuota = map[string]int32{machineSetName: 1}

				return nil
			}))

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.Equal("boot-healthy", res.TypedSpec().Value.PreRebootBootId)
			})
		},
	)
}

// TestUnhealthyMachinesRecoverOneByOne verifies that the recovery bypass keeps the upgrade concurrency
// bounded: with two unhealthy machines and parallelism 1, only one of them starts its update, the other
// waits for the upgrade lock to be released. The absence of a second upgrade is proven with the same
// causal barrier as in TestUpgradeQuotaStillBlocksHealthyMachine, applied to both machines: once both
// config diffs are visible, both machines have completed a slot decision with an upgrade due, so a
// wrongly issued second upgrade would already be in its machine mock.
func TestUnhealthyMachinesRecoverOneByOne(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
	t.Cleanup(cancel)

	provider := &testutils.FakeKubernetesProvider{}

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(
				machineconfig.NewStatusController(testutils.NewLifecycleManager(t, tc.State, provider)),
			))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			clusterName := "recovery-one-by-one"

			machineServices := testutils.NewMachineServices(t, st)

			_, machines := createCluster(ctx, t, st, machineServices, clusterName, 2, 0)

			ids := make([]string, 0, len(machines))
			nodeNames := make([]string, 0, len(machines))

			for _, m := range machines {
				ids = append(ids, m.Metadata().ID())
				nodeNames = append(nodeNames, "node-"+m.Metadata().ID())
			}

			rtestutils.AssertResources(ctx, t, st, ids, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256)
			})

			provider.SetClientset(testutils.NewFakeKubernetesClientset(nodeNames...))

			machineSetName, ok := machines[0].Metadata().Labels().Get(omni.LabelMachineSet)
			require.True(t, ok)

			// Both machines are broken, and the quota reflects that: zero for the machine set.
			rmock.Mock[*omni.UpgradeRollout](ctx, t, st, options.WithID(clusterName), options.Modify(func(res *omni.UpgradeRollout) error {
				res.TypedSpec().Value.MachineSetsUpgradeQuota = map[string]int32{machineSetName: 0}

				return nil
			}))

			for i, id := range ids {
				rmock.Mock[*omni.ClusterMachineIdentity](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.ClusterMachineIdentity) error {
					res.TypedSpec().Value.Nodename = nodeNames[i]

					return nil
				}))

				rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.MachineStatusSnapshot) error {
					res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
						Stage:  machine.MachineStatusEvent_BOOTING,
						Status: &machine.MachineStatusEvent_MachineStatus{Ready: false},
					}
					res.TypedSpec().Value.BootId = "boot-" + id

					return nil
				}))

				rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.MachineConfigGenOptions) error {
					res.TypedSpec().Value.InstallImage.SchematicId = "bbbb"

					return nil
				}))
			}

			countUpgrading := func() int {
				count := 0

				for _, id := range ids {
					if len(machineServices.Get(id).GetLifecycleUpgradeRequests()) > 0 {
						count++
					}
				}

				return count
			}

			// From here on every pass of either machine has an upgrade due (see the same checkpoint in
			// TestUpgradeQuotaStillBlocksHealthyMachine).
			rtestutils.AssertResources(ctx, t, st, ids, func(res *omni.MachinePendingUpdates, a *assert.Assertions) {
				a.NotNil(res.TypedSpec().Value.Upgrade)
			})

			// The causal barrier, for both machines: once both config diffs are visible, both machines
			// have completed a pass that decided on an upgrade slot, so whichever upgrades were going
			// to be issued are already in the machine mocks. The winner never finishes its reboot in
			// this test, so its upgrade lock stays held and there is nothing left to wait for.
			for _, id := range ids {
				rmock.Mock[*omni.ClusterMachineConfig](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.ClusterMachineConfig) error {
					return res.TypedSpec().Value.SetUncompressedData([]byte("machine:\n  network:\n    kubespan:\n      enabled: true"))
				}))
			}

			rtestutils.AssertResources(ctx, t, st, ids, func(res *omni.MachinePendingUpdates, a *assert.Assertions) {
				data, err := res.TypedSpec().Value.GetUncompressedData()
				if !a.NoError(err) {
					return
				}

				defer data.Free()

				a.Contains(string(data.Data()), "kubespan")
			})

			assert.Equal(t, 1, countUpgrading(), "exactly one unhealthy machine may upgrade at a time")
		},
	)
}

// TestLegacyRecoveryStagesUpgradeAndReboots verifies the recovery path for machines older than the
// LifecycleService (Talos < 1.13). The regular upgrade sequence on such a machine drains the node
// through the Kubernetes API, which an unhealthy machine may not have at all, so the recovery uses a
// staged upgrade followed by an explicit reboot instead. A healthy machine must keep the regular
// upgrade with the node-side drain: no staging, no reboot request.
func TestLegacyRecoveryStagesUpgradeAndReboots(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		unhealthy bool
	}{
		{name: "unhealthyMachineGetsStagedUpgradeAndReboot", unhealthy: true},
		{name: "healthyMachineKeepsRegularUpgrade", unhealthy: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
			t.Cleanup(cancel)

			testutils.WithRuntime(
				ctx, t, testutils.TestOptions{},
				func(_ context.Context, tc testutils.TestContext) {
					require.NoError(t, tc.Runtime.RegisterQController(
						machineconfig.NewStatusController(testutils.NewLifecycleManager(t, tc.State, nil)),
					))
				},
				func(ctx context.Context, tc testutils.TestContext) {
					st := tc.State

					const targetVersion = "1.11.9"

					clusterName := "legacy-recovery-" + tt.name

					machineServices := testutils.NewMachineServices(t, st)

					_, machines := createCluster(ctx, t, st, machineServices, clusterName, 1, 0,
						withClusterMockOption(options.WithTalosVersion("1.11.6")))
					id := machines[0].Metadata().ID()

					// The mock machine reports the target version once upgraded, as a real one would after the reboot.
					machineServices.Get(id).OnUpdate = func(ctx context.Context, _ *machine.UpgradeRequest, st state.State, id string) (*machine.UpgradeResponse, error) {
						if err := safe.StateModify(ctx, st, omni.NewMachineStatus(id), func(res *omni.MachineStatus) error {
							res.TypedSpec().Value.TalosVersion = targetVersion

							return nil
						}); err != nil {
							return nil, err
						}

						return &machine.UpgradeResponse{}, nil
					}

					// Wait for the initial config to be applied before changing the target.
					rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
						a.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256)
					})

					if tt.unhealthy {
						// The machine is stuck in the booting stage. Talos versions this old report no boot ID.
						rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.MachineStatusSnapshot) error {
							res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
								Stage:  machine.MachineStatusEvent_BOOTING,
								Status: &machine.MachineStatusEvent_MachineStatus{Ready: false},
							}

							return nil
						}))
					}

					// The fixing change arrives: a new target Talos version.
					rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.MachineConfigGenOptions) error {
						res.TypedSpec().Value.InstallImage.TalosVersion = targetVersion

						return nil
					}))

					// Once the applied version is recorded back, the pass that issued the upgrade has fully
					// completed: reconciles of one machine are serialized, so by now the reboot request of
					// that pass has been sent, or provably never will be.
					rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
						a.Equal(targetVersion, res.TypedSpec().Value.TalosVersion)
					})

					requests := machineServices.Get(id).GetUpgradeRequests()
					require.NotEmpty(t, requests)

					for _, request := range requests {
						assert.Equal(t, tt.unhealthy, request.Stage)
					}

					assert.Empty(t, machineServices.Get(id).GetLifecycleUpgradeRequests(), "a pre-1.13 machine must not receive a lifecycle upgrade")

					if tt.unhealthy {
						assert.Positive(t, machineServices.Get(id).GetRebootCount(), "the staged upgrade needs an explicit reboot to apply")
					} else {
						assert.Zero(t, machineServices.Get(id).GetRebootCount(), "a healthy machine reboots through its own upgrade sequence")
					}
				},
			)
		})
	}
}
