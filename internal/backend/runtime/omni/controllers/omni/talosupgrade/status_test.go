// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talosupgrade_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	xmaps "github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/talosupgrade"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	testoptions "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

const defaultSchematic = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

//nolint:gocognit,maintidx
func TestStatusController(t *testing.T) {
	t.Parallel()

	addControllers := func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(talosupgrade.NewStatusController()))
	}

	createCluster := func(
		ctx context.Context,
		t *testing.T,
		st state.State,
		clusterName string,
		controlPlanes, workers int,
		opts ...testoptions.MockOption,
	) (*omni.Cluster, []*omni.ClusterMachine) {
		clusterOptions := append([]testoptions.MockOption{
			testoptions.WithID(clusterName),
		}, opts...)

		cluster := rmock.Mock[*omni.Cluster](ctx, t, st, clusterOptions...)

		rmock.Mock[*omni.ClusterConfigVersion](ctx, t, st, testoptions.SameID(cluster),
			testoptions.Modify(func(res *omni.ClusterConfigVersion) error {
				res.TypedSpec().Value.Version = cluster.TypedSpec().Value.TalosVersion

				return nil
			}))

		cpMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
			testoptions.WithID(omni.ControlPlanesResourceID(clusterName)),
			testoptions.LabelCluster(cluster),
			testoptions.EmptyLabel(omni.LabelControlPlaneRole),
		)

		workersMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
			testoptions.WithID(omni.WorkersResourceID(clusterName)),
			testoptions.LabelCluster(cluster),
			testoptions.EmptyLabel(omni.LabelWorkerRole),
		)

		getIDs := func(machineType string, count int) []string {
			res := make([]string, 0, count)

			for i := range count {
				res = append(res, fmt.Sprintf("node-%s-%s-%d", clusterName, machineType, i))
			}

			return res
		}

		// create control planes
		rmock.MockList[*omni.MachineSetNode](ctx, t, st,
			testoptions.IDs(getIDs("cp", controlPlanes)),
			testoptions.ItemOptions(
				testoptions.LabelCluster(cluster),
				testoptions.LabelMachineSet(cpMachineSet),
				testoptions.EmptyLabel(omni.LabelControlPlaneRole),
			),
		)

		if workers > 0 {
			// create workers
			rmock.MockList[*omni.MachineSetNode](ctx, t, st,
				testoptions.IDs(getIDs("w", workers)),
				testoptions.ItemOptions(
					testoptions.LabelCluster(cluster),
					testoptions.LabelMachineSet(workersMachineSet),
					testoptions.EmptyLabel(omni.LabelWorkerRole),
				),
			)
		}

		cpMachines := rmock.MockList[*omni.ClusterMachine](ctx, t, st,
			testoptions.QueryIDs[*omni.MachineSetNode](resource.LabelEqual(omni.LabelMachineSet, cpMachineSet.Metadata().ID())),
			testoptions.ItemOptions(
				testoptions.LabelCluster(cluster),
				testoptions.LabelMachineSet(cpMachineSet),
				testoptions.EmptyLabel(omni.LabelControlPlaneRole),
				testoptions.Modify(func(res *omni.ClusterMachine) error {
					res.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

					return nil
				}),
			),
		)

		workerMachines := rmock.MockList[*omni.ClusterMachine](ctx, t, st,
			testoptions.QueryIDs[*omni.MachineSetNode](resource.LabelEqual(omni.LabelMachineSet, workersMachineSet.Metadata().ID())),
			testoptions.ItemOptions(
				testoptions.LabelCluster(cluster),
				testoptions.LabelMachineSet(workersMachineSet),
				testoptions.EmptyLabel(omni.LabelWorkerRole),
				testoptions.Modify(func(res *omni.ClusterMachine) error {
					res.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

					return nil
				}),
			),
		)

		machines := slices.Concat(cpMachines, workerMachines)

		for _, machine := range machines {
			rmock.Mock[*omni.SchematicConfiguration](ctx, t, st, testoptions.SameID(machine),
				testoptions.Modify(func(res *omni.SchematicConfiguration) error {
					res.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

					res.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
					res.TypedSpec().Value.SchematicId = defaultSchematic

					return nil
				}))

			rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, testoptions.SameID(machine),
				testoptions.Modify(func(res *omni.ClusterMachineConfigStatus) error {
					helpers.CopyAllLabels(machine, res)

					res.TypedSpec().Value.ClusterMachineConfigSha256 = "aaaa"
					res.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
					res.TypedSpec().Value.SchematicId = defaultSchematic

					return nil
				}))

			rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st, testoptions.SameID(machine),
				testoptions.Modify(func(res *omni.ClusterMachineStatus) error {
					helpers.CopyAllLabels(machine, res)

					res.TypedSpec().Value.Stage = specs.ClusterMachineStatusSpec_RUNNING
					res.TypedSpec().Value.Ready = true

					return nil
				}))
		}

		return cluster, machines
	}

	destroyCluster := func(ctx context.Context, t *testing.T, st state.State, clusterID string) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		list, err := safe.StateListAll[*omni.ClusterMachine](ctx, st,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)),
		)

		require.NoError(t, err)

		for cm := range list.All() {
			rmock.Destroy[*omni.MachineSetNode](ctx, t, st, []string{cm.Metadata().ID()})
			rmock.Destroy[*omni.ClusterMachine](ctx, t, st, []string{cm.Metadata().ID()})
			rmock.Destroy[*omni.ClusterMachineStatus](ctx, t, st, []string{cm.Metadata().ID()})
			rmock.Destroy[*omni.SchematicConfiguration](ctx, t, st, []string{cm.Metadata().ID()})
			rmock.Destroy[*omni.ClusterMachineConfigStatus](ctx, t, st, []string{cm.Metadata().ID()})
		}

		rmock.Destroy[*omni.Cluster](ctx, t, st, []string{clusterID})
		rmock.Destroy[*omni.ClusterConfigVersion](ctx, t, st, []string{clusterID})
	}

	// Tests the full upgrade cycle:
	// 1. Starts at version V1 in sync → Done
	// 2. Upgrades to V2 → Upgrading with correct status messages
	// 3. Marks each machine as upgraded → Done with lastUpgradeVersion = V2
	// Also verifies ClusterMachineTalosVersion resources lifecycle.
	t.Run("reconcile", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "talos-upgrade-cluster"
				talosVersion := constants.DefaultTalosVersion
				anotherTalosVersion := constants.AnotherTalosVersion
				stableTalosVersion := constants.StableTalosVersion

				cluster, machines := createCluster(ctx, t, st, clusterName, 3, 1,
					testoptions.WithTalosVersion(talosVersion))

				talosVersions := map[string][]string{
					stableTalosVersion:  {"1.29.1", "1.32.0", "1.33.0"},
					talosVersion:        {"1.32.0", "1.33.0"},
					anotherTalosVersion: {"1.32.0", "1.33.0", "1.34.2"},
				}

				rmock.MockList[*omni.TalosVersion](ctx, t, st,
					testoptions.IDs(xmaps.Keys(talosVersions)),
					testoptions.ItemOptions(
						testoptions.Modify(func(res *omni.TalosVersion) error {
							res.TypedSpec().Value.Version = res.Metadata().ID()
							res.TypedSpec().Value.CompatibleKubernetesVersions = talosVersions[res.Metadata().ID()]

							return nil
						}),
					),
				)

				// Assert ClusterMachineTalosVersion is created for each machine with the cluster's Talos version.
				for _, machine := range machines {
					rtestutils.AssertResource(ctx, t, st, machine.Metadata().ID(),
						func(res *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
							assertions.Equal(talosVersion, res.TypedSpec().Value.TalosVersion)
							assertions.Equal(defaultSchematic, res.TypedSpec().Value.SchematicId)
						})
				}

				// All machines are in sync → status should be Done.
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal(specs.TalosUpgradeStatusSpec_Done, res.TypedSpec().Value.Phase)
						assertions.Equal(talosVersion, res.TypedSpec().Value.LastUpgradeVersion)
						assertions.Empty(res.TypedSpec().Value.CurrentUpgradeVersion)
						assertions.Empty(res.TypedSpec().Value.Status)
					},
				)

				// Upgrade to a new version.
				rmock.Mock[*omni.Cluster](ctx, t, st, testoptions.SameID(cluster),
					testoptions.Modify(func(res *omni.Cluster) error {
						res.TypedSpec().Value.TalosVersion = anotherTalosVersion

						return nil
					}))

				// Update schematics to reflect the new version.
				rmock.MockList[*omni.SchematicConfiguration](ctx, t, st,
					testoptions.IDs(xslices.Map(machines, func(m *omni.ClusterMachine) resource.ID { return m.Metadata().ID() })),
					testoptions.ItemOptions(
						testoptions.Modify(func(res *omni.SchematicConfiguration) error {
							res.TypedSpec().Value.TalosVersion = anotherTalosVersion

							return nil
						}),
					))

				// Create MachinePendingUpdates to simulate the machineconfig controller detecting pending upgrades.
				for _, machine := range machines {
					rmock.Mock[*omni.MachinePendingUpdates](ctx, t, st,
						testoptions.WithID(machine.Metadata().ID()),
						testoptions.Modify(func(res *omni.MachinePendingUpdates) error {
							res.Metadata().Labels().Set(omni.LabelCluster, clusterName)
							res.TypedSpec().Value.Upgrade = &specs.MachinePendingUpdatesSpec_Upgrade{
								FromVersion:   talosVersion,
								ToVersion:     anotherTalosVersion,
								FromSchematic: defaultSchematic,
								ToSchematic:   defaultSchematic,
							}

							return nil
						}),
					)
				}

				// The upgrade should now be in progress: Upgrading phase.
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal(specs.TalosUpgradeStatusSpec_Upgrading, res.TypedSpec().Value.Phase)
						assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.CurrentUpgradeVersion)
						assertions.Equal(talosVersion, res.TypedSpec().Value.LastUpgradeVersion)
						assertions.Contains(res.TypedSpec().Value.Status, "updating machines")
					})

				// Simulate each machine completing the upgrade:
				// remove MachinePendingUpdates and update ClusterMachineConfigStatus.
				for i, machine := range machines {
					// Remove pending update for this machine.
					require.NoError(t, st.Destroy(ctx, omni.NewMachinePendingUpdates(machine.Metadata().ID()).Metadata()))

					// Update config status to reflect the new version.
					rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, testoptions.SameID(machine),
						testoptions.Modify(func(res *omni.ClusterMachineConfigStatus) error {
							helpers.CopyAllLabels(machine, res)

							res.TypedSpec().Value.ClusterMachineConfigSha256 = "bbbb"
							res.TypedSpec().Value.TalosVersion = anotherTalosVersion
							res.TypedSpec().Value.SchematicId = defaultSchematic

							return nil
						}))

					if i < len(machines)-1 {
						// Not the last machine: still upgrading.
						rtestutils.AssertResource(ctx, t, st, clusterName,
							func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
								assertions.Equal(specs.TalosUpgradeStatusSpec_Upgrading, res.TypedSpec().Value.Phase)
								assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.CurrentUpgradeVersion)
							})
					}
				}

				// All machines upgraded: Done.
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal(specs.TalosUpgradeStatusSpec_Done, res.TypedSpec().Value.Phase)
						assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.LastUpgradeVersion)
						assertions.Empty(res.TypedSpec().Value.CurrentUpgradeVersion)
						assertions.Empty(res.TypedSpec().Value.Status)
						assertions.Empty(res.TypedSpec().Value.Step)
						// Upgrade versions should list the previous version as a downgrade option.
						assertions.True(slices.Contains(res.TypedSpec().Value.UpgradeVersions, talosVersion))
					})

				// ClusterMachineTalosVersion should be updated to the new version.
				for _, machine := range machines {
					rtestutils.AssertResource(ctx, t, st, machine.Metadata().ID(),
						func(res *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
							assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.TalosVersion)
						})
				}

				destroyCluster(ctx, t, st, clusterName)

				// All ClusterMachineTalosVersion resources should be cleaned up.
				for _, machine := range machines {
					rtestutils.AssertNoResource[*omni.ClusterMachineTalosVersion](ctx, t, st, machine.Metadata().ID())
				}
			},
		)
	})

	// Tests that when all machines with pending upgrades are locked,
	// the status reports "all machines are locked".
	t.Run("locked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "talos-upgrade-locked"
				talosVersion := constants.DefaultTalosVersion
				anotherTalosVersion := constants.AnotherTalosVersion

				cluster, machines := createCluster(ctx, t, st, clusterName, 1, 3,
					testoptions.WithTalosVersion(talosVersion))

				machineIDs := xslices.Map(machines, func(m *omni.ClusterMachine) string { return m.Metadata().ID() })

				// Wait for initial Done state.
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal(specs.TalosUpgradeStatusSpec_Done, res.TypedSpec().Value.Phase)
					})

				// Change cluster version to trigger upgrade.
				rmock.Mock[*omni.Cluster](ctx, t, st, testoptions.SameID(cluster),
					testoptions.Modify(func(res *omni.Cluster) error {
						res.TypedSpec().Value.TalosVersion = anotherTalosVersion

						return nil
					}))

				// Update schematics to new version.
				rmock.MockList[*omni.SchematicConfiguration](ctx, t, st,
					testoptions.IDs(machineIDs),
					testoptions.ItemOptions(
						testoptions.Modify(func(res *omni.SchematicConfiguration) error {
							res.TypedSpec().Value.TalosVersion = anotherTalosVersion

							return nil
						}),
					))

				// Lock all machines.
				rmock.MockList[*omni.MachineSetNode](ctx, t, st,
					testoptions.IDs(machineIDs),
					testoptions.ItemOptions(
						testoptions.Modify(func(r *omni.MachineSetNode) error {
							r.Metadata().Annotations().Set(omni.MachineLocked, "")

							return nil
						}),
					),
				)

				// Create MachinePendingUpdates for all machines.
				for _, machine := range machines {
					rmock.Mock[*omni.MachinePendingUpdates](ctx, t, st,
						testoptions.WithID(machine.Metadata().ID()),
						testoptions.Modify(func(res *omni.MachinePendingUpdates) error {
							res.Metadata().Labels().Set(omni.LabelCluster, clusterName)
							res.TypedSpec().Value.Upgrade = &specs.MachinePendingUpdatesSpec_Upgrade{
								FromVersion:   talosVersion,
								ToVersion:     anotherTalosVersion,
								FromSchematic: "doesn't matter",
								ToSchematic:   "doesn't matter",
							}

							return nil
						}),
					)
				}

				// All machines are locked, no machines upgrading → "all machines are locked".
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal("all machines are locked", res.TypedSpec().Value.Step)
						assertions.Equal("waiting for machines to be unlocked", res.TypedSpec().Value.Status)
					})
			},
		)
	})

	// Tests that the UpgradeRollout resource is created and quotas are computed correctly
	// based on machine set parallelism and cluster readiness.
	t.Run("upgradeRollout", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "talos-upgrade-rollout"
				talosVersion := constants.DefaultTalosVersion

				_, machines := createCluster(ctx, t, st, clusterName, 3, 2,
					testoptions.WithTalosVersion(talosVersion))

				cpMachineSetID := omni.ControlPlanesResourceID(clusterName)
				workerMachineSetID := omni.WorkersResourceID(clusterName)

				// UpgradeRollout should be created with quota=1 per machine set (default Rolling strategy).
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.UpgradeRollout, assertions *assert.Assertions) {
						assertions.Equal(int32(1), res.TypedSpec().Value.MachineSetsUpgradeQuota[cpMachineSetID])
						assertions.Equal(int32(1), res.TypedSpec().Value.MachineSetsUpgradeQuota[workerMachineSetID])
					})

				// Mark one machine as not ready — this should reduce quotas to 0.
				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st, testoptions.SameID(machines[0]),
					testoptions.Modify(func(res *omni.ClusterMachineStatus) error {
						helpers.CopyAllLabels(machines[0], res)

						res.TypedSpec().Value.Stage = specs.ClusterMachineStatusSpec_BOOTING
						res.TypedSpec().Value.Ready = false

						return nil
					}))

				// With 1 not-ready machine and default parallelism of 1, quota = max(1-1, 0) = 0.
				// All quotas become 0, so the map should be empty.
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.UpgradeRollout, assertions *assert.Assertions) {
						assertions.Empty(res.TypedSpec().Value.MachineSetsUpgradeQuota)
					})

				// Restore machine to ready state.
				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st, testoptions.SameID(machines[0]),
					testoptions.Modify(func(res *omni.ClusterMachineStatus) error {
						helpers.CopyAllLabels(machines[0], res)

						res.TypedSpec().Value.Stage = specs.ClusterMachineStatusSpec_RUNNING
						res.TypedSpec().Value.Ready = true

						return nil
					}))

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.UpgradeRollout, assertions *assert.Assertions) {
						assertions.Equal(int32(1), res.TypedSpec().Value.MachineSetsUpgradeQuota[cpMachineSetID])
						assertions.Equal(int32(1), res.TypedSpec().Value.MachineSetsUpgradeQuota[workerMachineSetID])
					})
			},
		)
	})

	// Tests that when control planes are upgrading (outdated), the worker machine set
	// gets zero quota in the UpgradeRollout.
	t.Run("upgradeRolloutControlPlaneFirst", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "talos-upgrade-rollout-cp-first"
				talosVersion := constants.DefaultTalosVersion
				anotherTalosVersion := constants.AnotherTalosVersion

				cluster, machines := createCluster(ctx, t, st, clusterName, 1, 2,
					testoptions.WithTalosVersion(talosVersion))

				workerMachineSetID := omni.WorkersResourceID(clusterName)
				cpMachineSetID := omni.ControlPlanesResourceID(clusterName)

				// Wait for initial Done state.
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal(specs.TalosUpgradeStatusSpec_Done, res.TypedSpec().Value.Phase)
					})

				// Change cluster version.
				rmock.Mock[*omni.Cluster](ctx, t, st, testoptions.SameID(cluster),
					testoptions.Modify(func(res *omni.Cluster) error {
						res.TypedSpec().Value.TalosVersion = anotherTalosVersion

						return nil
					}))

				// Update schematics to new version for all machines.
				rmock.MockList[*omni.SchematicConfiguration](ctx, t, st,
					testoptions.IDs(xslices.Map(machines, func(m *omni.ClusterMachine) resource.ID { return m.Metadata().ID() })),
					testoptions.ItemOptions(
						testoptions.Modify(func(res *omni.SchematicConfiguration) error {
							res.TypedSpec().Value.TalosVersion = anotherTalosVersion

							return nil
						}),
					))

				// Create MachinePendingUpdates for all machines.
				for _, machine := range machines {
					rmock.Mock[*omni.MachinePendingUpdates](ctx, t, st,
						testoptions.WithID(machine.Metadata().ID()),
						testoptions.Modify(func(res *omni.MachinePendingUpdates) error {
							res.Metadata().Labels().Set(omni.LabelCluster, clusterName)
							res.TypedSpec().Value.Upgrade = &specs.MachinePendingUpdatesSpec_Upgrade{
								FromVersion: talosVersion,
								ToVersion:   anotherTalosVersion,
							}

							return nil
						}),
					)
				}

				// With control planes outdated, workers should get zero quota.
				// CP quota remains non-zero (control planes upgrade first).
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.UpgradeRollout, assertions *assert.Assertions) {
						assertions.Equal(int32(0), res.TypedSpec().Value.MachineSetsUpgradeQuota[workerMachineSetID])
						assertions.Equal(int32(1), res.TypedSpec().Value.MachineSetsUpgradeQuota[cpMachineSetID])
					})
			},
		)
	})

	// Tests that ClusterMachineTalosVersion is reconciled from SchematicConfiguration:
	// - created when schematic exists with matching version
	// - updated when schematic changes (same version, different schematic ID)
	// - updated when SchematicConfiguration version is updated to match cluster version
	// - destroyed when SchematicConfiguration is removed (machine removed from cluster)
	t.Run("reconcileTalosVersions", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "talos-upgrade-versions"
				talosVersion := constants.DefaultTalosVersion
				anotherTalosVersion := constants.AnotherTalosVersion
				altSchematic := "c6ee5f479027e5ca84e5518c3a56d62e2283b6d30a5846e6295aa7113735df40"

				cluster, machines := createCluster(ctx, t, st, clusterName, 2, 1,
					testoptions.WithTalosVersion(talosVersion))

				// All machines should get ClusterMachineTalosVersion with the cluster's version and default schematic.
				for _, machine := range machines {
					rtestutils.AssertResource(ctx, t, st, machine.Metadata().ID(),
						func(res *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
							assertions.Equal(talosVersion, res.TypedSpec().Value.TalosVersion)
							assertions.Equal(defaultSchematic, res.TypedSpec().Value.SchematicId)
						})
				}

				// Change schematic for one machine (same version, different schematic).
				rmock.Mock[*omni.SchematicConfiguration](ctx, t, st, testoptions.SameID(machines[1]),
					testoptions.Modify(func(res *omni.SchematicConfiguration) error {
						res.TypedSpec().Value.TalosVersion = talosVersion
						res.TypedSpec().Value.SchematicId = altSchematic

						return nil
					}))

				// That machine's ClusterMachineTalosVersion should reflect the new schematic.
				rtestutils.AssertResource(ctx, t, st, machines[1].Metadata().ID(),
					func(res *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
						assertions.Equal(altSchematic, res.TypedSpec().Value.SchematicId)
					})

				// Change cluster version and update schematics accordingly.
				rmock.Mock[*omni.Cluster](ctx, t, st, testoptions.SameID(cluster),
					testoptions.Modify(func(res *omni.Cluster) error {
						res.TypedSpec().Value.TalosVersion = anotherTalosVersion

						return nil
					}))

				// Update schematics to new version.
				rmock.MockList[*omni.SchematicConfiguration](ctx, t, st,
					testoptions.IDs(xslices.Map(machines, func(m *omni.ClusterMachine) resource.ID { return m.Metadata().ID() })),
					testoptions.ItemOptions(
						testoptions.Modify(func(res *omni.SchematicConfiguration) error {
							res.TypedSpec().Value.TalosVersion = anotherTalosVersion

							return nil
						}),
					))

				// ClusterMachineTalosVersion should be updated with the new version.
				for _, machine := range machines {
					rtestutils.AssertResource(ctx, t, st, machine.Metadata().ID(),
						func(res *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
							assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.TalosVersion)
						})
				}

				// Remove one machine from the cluster — its SchematicConfiguration is destroyed.
				removedMachine := machines[0]

				rmock.Destroy[*omni.MachineSetNode](ctx, t, st, []string{removedMachine.Metadata().ID()})
				rmock.Destroy[*omni.ClusterMachine](ctx, t, st, []string{removedMachine.Metadata().ID()})
				rmock.Destroy[*omni.ClusterMachineStatus](ctx, t, st, []string{removedMachine.Metadata().ID()})
				rmock.Destroy[*omni.SchematicConfiguration](ctx, t, st, []string{removedMachine.Metadata().ID()})
				rmock.Destroy[*omni.ClusterMachineConfigStatus](ctx, t, st, []string{removedMachine.Metadata().ID()})

				// ClusterMachineTalosVersion for the removed machine should be cleaned up.
				rtestutils.AssertNoResource[*omni.ClusterMachineTalosVersion](ctx, t, st, removedMachine.Metadata().ID())

				// Other machines should still have their ClusterMachineTalosVersion.
				for _, machine := range machines[1:] {
					rtestutils.AssertResource(ctx, t, st, machine.Metadata().ID(),
						func(_ *omni.ClusterMachineTalosVersion, _ *assert.Assertions) {})
				}

				destroyCluster(ctx, t, st, clusterName)
			},
		)
	})

	// Tests that when a cluster is deleted, the controller cleans up
	// ClusterMachineTalosVersion and UpgradeRollout resources.
	t.Run("finalization", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "talos-upgrade-finalization"

				_, machines := createCluster(ctx, t, st, clusterName, 2, 1,
					testoptions.WithTalosVersion(constants.DefaultTalosVersion))

				// Wait for ClusterMachineTalosVersion and UpgradeRollout to be created.
				for _, machine := range machines {
					rtestutils.AssertResource[*omni.ClusterMachineTalosVersion](ctx, t, st, machine.Metadata().ID(),
						func(_ *omni.ClusterMachineTalosVersion, _ *assert.Assertions) {})
				}

				rtestutils.AssertResource[*omni.UpgradeRollout](ctx, t, st, clusterName,
					func(_ *omni.UpgradeRollout, _ *assert.Assertions) {})

				// Destroy the cluster — triggers finalizer removal flow.
				destroyCluster(ctx, t, st, clusterName)

				// ClusterMachineTalosVersion should all be cleaned up.
				for _, machine := range machines {
					rtestutils.AssertNoResource[*omni.ClusterMachineTalosVersion](ctx, t, st, machine.Metadata().ID())
				}

				// UpgradeRollout should be destroyed too.
				rtestutils.AssertNoResource[*omni.UpgradeRollout](ctx, t, st, clusterName)
			},
		)
	})

	// Tests the schematic-only update path (no version change):
	// Phase should be UpdatingMachineSchematics when schematics change without a version bump.
	t.Run("schematicUpdate", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "talos-upgrade-schematic"
				talosVersion := constants.DefaultTalosVersion
				altSchematic := "c6ee5f479027e5ca84e5518c3a56d62e2283b6d30a5846e6295aa7113735df40"

				_, machines := createCluster(ctx, t, st, clusterName, 1, 1,
					testoptions.WithTalosVersion(talosVersion))

				// Wait for initial Done state.
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal(specs.TalosUpgradeStatusSpec_Done, res.TypedSpec().Value.Phase)
					})

				// Create pending updates indicating a schematic change (same version, different schematic).
				for _, machine := range machines {
					rmock.Mock[*omni.MachinePendingUpdates](ctx, t, st,
						testoptions.WithID(machine.Metadata().ID()),
						testoptions.Modify(func(res *omni.MachinePendingUpdates) error {
							res.Metadata().Labels().Set(omni.LabelCluster, clusterName)
							res.TypedSpec().Value.Upgrade = &specs.MachinePendingUpdatesSpec_Upgrade{
								FromVersion:   talosVersion,
								ToVersion:     talosVersion,
								FromSchematic: defaultSchematic,
								ToSchematic:   altSchematic,
							}

							return nil
						}),
					)
				}

				// Schematic-only update should yield UpdatingMachineSchematics phase.
				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal(specs.TalosUpgradeStatusSpec_UpdatingMachineSchematics, res.TypedSpec().Value.Phase)
						assertions.Contains(res.TypedSpec().Value.Status, "updating machines")
					})
			},
		)
	})
}
