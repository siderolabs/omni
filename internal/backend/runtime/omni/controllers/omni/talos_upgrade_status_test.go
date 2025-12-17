// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

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
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	testoptions "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

//nolint:gocognit,maintidx
func TestTalosUpgradeStatus(t *testing.T) {
	t.Parallel()

	createCluster := func(
		ctx context.Context,
		t *testing.T,
		st state.State,
		machineServices *testutils.MachineServices,
		clusterName string,
		controlPlanes, workers int,
		opts ...testoptions.MockOption,
	) (*omni.Cluster, []*omni.ClusterMachine) {
		clusterOptions := append([]testoptions.MockOption{
			testoptions.WithID(clusterName),
		}, opts...)

		cluster := rmock.Mock[*omni.Cluster](ctx, t, st,
			clusterOptions...,
		)

		rmock.Mock[*omni.ClusterConfigVersion](ctx, t, st, testoptions.SameID(cluster),
			testoptions.Modify(func(res *omni.ClusterConfigVersion) error {
				res.TypedSpec().Value.Version = cluster.TypedSpec().Value.TalosVersion

				return nil
			}))
		rmock.Mock[*omni.ClusterStatus](ctx, t, st, testoptions.SameID(cluster),
			testoptions.Modify(
				func(res *omni.ClusterStatus) error {
					res.TypedSpec().Value.Available = true
					res.TypedSpec().Value.Ready = true
					res.TypedSpec().Value.Phase = specs.ClusterStatusSpec_RUNNING

					return nil
				},
			),
		)

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
				res = append(res, fmt.Sprintf("node-%s-%d", machineType, i))
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
				testoptions.Modify(
					func(res *omni.ClusterMachine) error {
						res.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

						return nil
					},
				),
			),
		)

		workerMachines := rmock.MockList[*omni.ClusterMachine](ctx, t, st,
			testoptions.QueryIDs[*omni.MachineSetNode](resource.LabelEqual(omni.LabelMachineSet, workersMachineSet.Metadata().ID())),
			testoptions.ItemOptions(
				testoptions.LabelCluster(cluster),
				testoptions.LabelMachineSet(workersMachineSet),
				testoptions.Modify(
					func(res *omni.ClusterMachine) error {
						res.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

						return nil
					},
				),
			),
		)

		machines := slices.Concat(cpMachines, workerMachines)

		for _, machine := range machines {
			rmock.Mock[*omni.MachineStatus](ctx, t, st, testoptions.SameID(machine),
				testoptions.Modify(func(res *omni.MachineStatus) error {
					res.TypedSpec().Value.Maintenance = false

					return nil
				}),
				testoptions.WithMachineServices(ctx, machineServices),
			)

			rmock.Mock[*omni.ClusterMachineIdentity](ctx, t, st, testoptions.SameID(machine))
			rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, testoptions.SameID(machine))
			rmock.Mock[*omni.SchematicConfiguration](ctx, t, st, testoptions.SameID(machine),
				testoptions.Modify(func(res *omni.SchematicConfiguration) error {
					res.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
					res.TypedSpec().Value.SchematicId = defaultSchematic

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

		for cs := range list.All() {
			rmock.Destroy[*omni.ClusterMachineIdentity](ctx, t, st, []string{cs.Metadata().ID()})
			rmock.Destroy[*omni.MachineSetNode](ctx, t, st, []string{cs.Metadata().ID()})
			rmock.Destroy[*omni.ClusterMachine](ctx, t, st, []string{cs.Metadata().ID()})
			rmock.Destroy[*omni.MachineStatus](ctx, t, st, []string{cs.Metadata().ID()})
			rmock.Destroy[*omni.SchematicConfiguration](ctx, t, st, []string{cs.Metadata().ID()})
			rmock.Destroy[*omni.ClusterMachineConfigStatus](ctx, t, st, []string{cs.Metadata().ID()})
		}

		rmock.Destroy[*omni.Cluster](ctx, t, st, []string{clusterID})
		rmock.Destroy[*omni.ClusterStatus](ctx, t, st, []string{clusterID})
		rmock.Destroy[*omni.ClusterConfigVersion](ctx, t, st, []string{clusterID})
	}

	t.Run("reconcile", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				machineServices := testutils.NewMachineServices(t, st)
				clusterName := "talos-upgrade-cluster"
				talosVersion := "1.11.3"
				anotherTalosVersion := "1.11.5"

				cluster, machines := createCluster(ctx, t, st, machineServices, clusterName, 3, 1, testoptions.WithTalosVersion(talosVersion))

				talosVersions := map[string][]string{"1.10.0": {"1.29.1", "1.32.0", "1.33.0"}, talosVersion: {"1.32.0", "1.33.0"}, anotherTalosVersion: {"1.32.0", "1.33.0", "1.34.2"}}

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

				for _, machine := range machines {
					rtestutils.AssertResource(ctx, t, st, machine.Metadata().ID(),
						func(res *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
							versionSpec := res.TypedSpec().Value

							assertions.Equal(cluster.TypedSpec().Value.TalosVersion, versionSpec.TalosVersion)
						})

					rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, testoptions.SameID(machine),
						testoptions.Modify(func(res *omni.ClusterMachineConfigStatus) error {
							helpers.CopyAllLabels(machine, res)
							res.TypedSpec().Value.ClusterMachineConfigSha256 = "aaaa"
							res.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
							res.TypedSpec().Value.SchematicId = defaultSchematic

							return nil
						}))
				}

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal(talosVersion, res.TypedSpec().Value.LastUpgradeVersion)
					},
				)

				rmock.Mock[*omni.ClusterStatus](ctx, t, st, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.ClusterStatus) error {
					res.TypedSpec().Value.Ready = false

					return nil
				}))

				rmock.Mock[*omni.Cluster](ctx, t, st, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.Cluster) error {
					res.TypedSpec().Value.TalosVersion = anotherTalosVersion

					return nil
				}))

				rmock.MockList[*omni.SchematicConfiguration](ctx, t, st, testoptions.IDs(xslices.Map(machines, func(machine *omni.ClusterMachine) resource.ID { return machine.Metadata().ID() })),
					testoptions.ItemOptions(
						testoptions.Modify(func(res *omni.SchematicConfiguration) error {
							res.TypedSpec().Value.TalosVersion = anotherTalosVersion

							return nil
						}),
					))

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal("waiting for the cluster to be ready", res.TypedSpec().Value.Status)
						assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.CurrentUpgradeVersion)
						assertions.Equal(talosVersion, res.TypedSpec().Value.LastUpgradeVersion)
					})

				rmock.Mock[*omni.ClusterStatus](ctx, t, st, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.ClusterStatus) error {
					res.TypedSpec().Value.Ready = true

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						assertions.Equal("updating machines 1/4", res.TypedSpec().Value.Status)
						assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.CurrentUpgradeVersion)
						assertions.Equal(talosVersion, res.TypedSpec().Value.LastUpgradeVersion)
					})

				rmock.Mock[*omni.SchematicConfiguration](ctx, t, st, testoptions.SameID(machines[1]), testoptions.Modify(func(res *omni.SchematicConfiguration) error {
					res.TypedSpec().Value.SchematicId = "c6ee5f479027e5ca84e5518c3a56d62e2283b6d30a5846e6295aa7113735df40"

					return nil
				}))

				for i := range machines {
					expectedSchematic := defaultSchematic
					if i == 1 {
						expectedSchematic = "c6ee5f479027e5ca84e5518c3a56d62e2283b6d30a5846e6295aa7113735df40"
					}

					rtestutils.AssertResource(ctx, t, st, machines[i].Metadata().ID(),
						func(r *omni.ClusterMachineTalosVersion, assertion *assert.Assertions) {
							assertion.Equal(expectedSchematic, r.TypedSpec().Value.SchematicId)
						},
					)

					rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, testoptions.SameID(machines[i]),
						testoptions.Modify(func(res *omni.ClusterMachineConfigStatus) error {
							res.TypedSpec().Value.ClusterMachineConfigSha256 = "aaaa"
							res.TypedSpec().Value.TalosVersion = anotherTalosVersion
							res.TypedSpec().Value.SchematicId = expectedSchematic

							return nil
						}))

					rtestutils.AssertResource(ctx, t, st, clusterName, func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
						if i < len(machines)-1 {
							assertions.Equal(fmt.Sprintf("updating machines %d/4", i+2), res.TypedSpec().Value.Status)
							assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.CurrentUpgradeVersion)
							assertions.Equal(talosVersion, res.TypedSpec().Value.LastUpgradeVersion)
							assertions.Equal(specs.TalosUpgradeStatusSpec_Upgrading, res.TypedSpec().Value.Phase)
						} else {
							assertions.Empty(res.TypedSpec().Value.Step)
							assertions.Empty(res.TypedSpec().Value.Error)
							assertions.Empty(res.TypedSpec().Value.Status)
							assertions.Empty(res.TypedSpec().Value.CurrentUpgradeVersion)
							assertions.Equal(anotherTalosVersion, res.TypedSpec().Value.LastUpgradeVersion)
							assertions.Equal(specs.TalosUpgradeStatusSpec_Done, res.TypedSpec().Value.Phase)
							assertions.True(slices.Contains(res.TypedSpec().Value.UpgradeVersions, talosVersion))
							assertions.False(slices.Contains(res.TypedSpec().Value.UpgradeVersions, "1.10.0"))
						}
					})
				}

				rmock.Destroy[*omni.ClusterMachine](ctx, t, st, []resource.ID{machines[0].Metadata().ID()})
				rmock.Destroy[*omni.TalosVersion](ctx, t, st, xmaps.Keys(talosVersions))
				rtestutils.AssertNoResource[*omni.ClusterMachineTalosVersion](ctx, t, st, machines[0].Metadata().ID())

				destroyCluster(ctx, t, st, clusterName)

				for _, res := range machines {
					rtestutils.AssertNoResource[*omni.ClusterMachineTalosVersion](ctx, t, st, res.Metadata().ID())
				}
			},
		)
	})

	// This test checks that machines' Talos version can be updated immediately if a machine is still running in the maintenance mode.
	t.Run("update versions maintenance", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				machineServices := testutils.NewMachineServices(t, st)
				clusterName := "talos-upgrade-cluster-2"

				cluster, machines := createCluster(ctx, t, st, machineServices, clusterName, 3, 1, testoptions.WithTalosVersion("1.5.5"))

				rmock.Mock[*omni.ClusterStatus](ctx, t, st, testoptions.SameID(cluster),
					testoptions.Modify(
						func(res *omni.ClusterStatus) error {
							res.TypedSpec().Value.Available = true
							res.TypedSpec().Value.Ready = false
							res.TypedSpec().Value.Phase = specs.ClusterStatusSpec_SCALING_UP

							return nil
						},
					),
				)

				for i, res := range machines {
					rtestutils.AssertResource(ctx, t, st, res.Metadata().ID(),
						func(version *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
							versionSpec := version.TypedSpec().Value

							assertions.Equal(cluster.TypedSpec().Value.TalosVersion, versionSpec.TalosVersion)
						})

					rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, testoptions.SameID(res),
						testoptions.Modify(
							func(res *omni.ClusterMachineConfigStatus) error {
								if i != 0 {
									res.TypedSpec().Value.ClusterMachineConfigSha256 = "bbbb"
								}

								res.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion

								return nil
							},
						),
					)
				}

				rmock.Mock[*omni.Cluster](ctx, t, st, testoptions.SameID(cluster),
					testoptions.Modify(
						func(res *omni.Cluster) error {
							res.TypedSpec().Value.TalosVersion = "1.5.5"

							return nil
						},
					),
				)

				for i, res := range machines {
					rtestutils.AssertResource(ctx, t, st, res.Metadata().ID(),
						func(version *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
							versionSpec := version.TypedSpec().Value

							expectedVersion := cluster.TypedSpec().Value.TalosVersion
							if i == 0 {
								expectedVersion = "1.5.5"
							}

							assertions.Equal(expectedVersion, versionSpec.TalosVersion)
						})
				}
			},
		)
	})

	t.Run("locked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				machineServices := testutils.NewMachineServices(t, st)
				clusterName := "talos-upgrade-locked"
				schematicID := "7d79f1ce28d7e6c099bc89ccf02238fb574165eb4834c2abf2a61eab998d4dc6"

				cluster, machines := createCluster(ctx, t, st, machineServices, clusterName, 1, 3, testoptions.WithTalosVersion(constants.DefaultTalosVersion))

				for _, machine := range machines {
					rtestutils.AssertResource[*omni.ClusterMachineTalosVersion](ctx, t, st, machine.Metadata().ID(),
						func(res *omni.ClusterMachineTalosVersion, assertions *assert.Assertions) {
							assertions.Equal(cluster.TypedSpec().Value.TalosVersion, res.TypedSpec().Value.TalosVersion)
						})

					if _, ok := machine.Metadata().Labels().Get(omni.LabelWorkerRole); ok {
						rmock.Mock[*omni.MachineSetNode](ctx, t, st, testoptions.SameID(machine), testoptions.Modify(func(res *omni.MachineSetNode) error {
							res.Metadata().Annotations().Set(omni.MachineLocked, "")

							return nil
						}))

						rmock.Mock[*omni.SchematicConfiguration](ctx, t, st, testoptions.SameID(machine), testoptions.Modify(func(res *omni.SchematicConfiguration) error {
							res.TypedSpec().Value.SchematicId = schematicID

							return nil
						}))
					}

					rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, testoptions.SameID(machine),
						testoptions.Modify(func(res *omni.ClusterMachineConfigStatus) error {
							helpers.CopyAllLabels(machine, res)

							res.TypedSpec().Value.ClusterMachineConfigSha256 = "dddd"
							res.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
							res.TypedSpec().Value.SchematicId = defaultSchematic

							return nil
						}))
				}

				rtestutils.AssertResource(ctx, t, st, clusterName, func(res *omni.TalosUpgradeStatus, assertions *assert.Assertions) {
					assertions.Contains(res.TypedSpec().Value.Status, "upgrade paused")
					assertions.Contains(res.TypedSpec().Value.Step, "waiting for the machine")
				})
			},
		)
	})
}
