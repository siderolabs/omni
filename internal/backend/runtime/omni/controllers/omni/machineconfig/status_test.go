// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

const (
	imageFactoryHost = "factory-test.talos.dev"
)

//nolint:gocognit,maintidx,gocyclo,cyclop
func TestMachineConfigStatusController(t *testing.T) {
	t.Parallel()

	addControllers := func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController(imageFactoryHost)))
	}

	awaitAllMachinesConfigured := func(ctx context.Context, t *testing.T, st state.State, clusterName string) {
		ids := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, st,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		rtestutils.AssertResources( // fails
			ctx,
			t,
			st,
			ids,
			func(res *omni.ClusterMachineConfigStatus, assertions *assert.Assertions) {
				assertions.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256, "the machine is not configured yet")
			},
		)
	}

	upgradeHandler := func(ctx context.Context, ur *machine.UpgradeRequest, st state.State, id string) (*machine.UpgradeResponse, error) {
		if err := safe.StateModify(ctx, st, omni.NewMachineStatus(id), func(res *omni.MachineStatus) error {
			urlString, tag, found := strings.Cut(ur.Image, ":")
			if !found {
				return fmt.Errorf("bad install image")
			}

			u, err := url.Parse(urlString)
			if err != nil {
				return err
			}

			parts := strings.Split(u.Path, "/")

			if !strings.HasPrefix(urlString, "ghcr.io") {
				res.TypedSpec().Value.Schematic.Id = parts[2]
			} else {
				res.TypedSpec().Value.Schematic.Id = ""
			}

			res.TypedSpec().Value.TalosVersion = strings.TrimLeft(tag, "v")

			return nil
		}); err != nil {
			return nil, err
		}

		return &machine.UpgradeResponse{}, nil
	}

	resetHandler := func(ctx context.Context, _ *machine.ResetRequest, st state.State, id string) (*machine.ResetResponse, error) {
		t.Logf("setting maintenance for %v", id)

		if err := safe.StateModify(ctx, st, omni.NewMachineStatusSnapshot(id),
			func(res *omni.MachineStatusSnapshot) error {
				res.TypedSpec().Value.MachineStatus.Stage = machine.MachineStatusEvent_MAINTENANCE

				return nil
			},
		); err != nil {
			return nil, err
		}

		return &machine.ResetResponse{}, nil
	}

	// Wait for the initial configs to be created.
	// Then update the status to simulate that machines are running.
	// Reset the machines.
	t.Run("applyReset", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "apply-reset", 3, 3)

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.OnReset = resetHandler
				})

				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				awaitAllMachinesConfigured(ctx, t, testContext.State, cluster.Metadata().ID())

				machineServices.ForEach(func(machineServer *testutils.MachineServiceMock) {
					assert.GreaterOrEqual(t, len(machineServer.GetApplyRequests()), 1)
				})

				rmock.Destroy[*omni.ClusterMachineConfig](ctx, t, testContext.State, ids)

				for _, m := range ids {
					rtestutils.AssertNoResource[*omni.ClusterMachineConfigStatus](ctx, t, testContext.State, m)
				}

				machineServices.ForEach(func(machineServer *testutils.MachineServiceMock) {
					assert.GreaterOrEqual(t, len(machineServer.GetResetRequests()), 1)
				})
			},
		)
	})

	// TearDown omni.Machine resource.
	// The controller shouldn't try calling reset and remove the machine forcefully.
	t.Run("resetMachineRemoved", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "forced-removal", 3, 3)

				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				awaitAllMachinesConfigured(ctx, t, testContext.State, cluster.Metadata().ID())

				rmock.Destroy[*omni.Machine](ctx, t, testContext.State, ids[:1])
				rmock.Destroy[*omni.ClusterMachineConfig](ctx, t, testContext.State, ids[:1])

				rtestutils.AssertNoResource[*omni.ClusterMachineConfigStatus](ctx, t, testContext.State, ids[0])

				require.Len(t, machineServices.Get(ids[0]).GetResetRequests(), 0)
			},
		)
	})

	// Create a cluster with one broken control plane node.
	// The cluster teardown should still work: the broken control plane should resort to ungraceful reset.
	t.Run("retryNonGraceful", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "ungraceful-cp-removal", 3, 0)

				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				machineServices.Get(ids[2]).SetEtcdLeaveHandler(
					func(context.Context, *machine.EtcdLeaveClusterRequest) (*machine.EtcdLeaveClusterResponse, error) {
						return nil, errors.New("sowwy I'm bwoken")
					},
				)

				awaitAllMachinesConfigured(ctx, t, testContext.State, cluster.Metadata().ID())

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.OnReset = resetHandler
				})

				rmock.Destroy[*omni.ClusterMachineConfig](ctx, t, testContext.State, ids)

				for _, m := range ids {
					rtestutils.AssertNoResource[*omni.ClusterMachineConfigStatus](ctx, t, testContext.State, m)
				}

				machineServices.ForEach(func(machineService *testutils.MachineServiceMock) {
					assert.GreaterOrEqual(t, len(machineService.GetResetRequests()), 1)
				})
			},
		)
	})

	// Test advanced control plane cleanup mode when NodeForceDestroyRequest is created.
	// Sometimes the machine might stuck in provisioning before it can join the etcd cluster
	// for these cases there's force destroy option that deletes the control plane even if it might hurt etcd quorum.
	// User can create this NodeForceDestroyRequest to force the control plane reset.
	t.Run("resetForceDestroy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers, func(ctx context.Context, testContext testutils.TestContext) {
			clusterName := "force-destroy"

			machineServices := testutils.NewMachineServices(t, testContext.State)

			_, machines := createCluster(ctx, t, testContext.State, machineServices, clusterName, 3, 0)

			awaitAllMachinesConfigured(ctx, t, testContext.State, clusterName)

			brokenMachineID := machines[2].Metadata().ID()

			rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, testContext.State,
				options.WithID(brokenMachineID),
				options.Modify(func(res *omni.MachineStatusSnapshot) error {
					res.TypedSpec().Value.MachineStatus.Stage = machine.MachineStatusEvent_BOOTING

					return nil
				}),
			)

			machineServices.ForEach(func(m *testutils.MachineServiceMock) {
				m.OnReset = resetHandler
			})

			require.NoError(t, testContext.State.Create(ctx, omni.NewNodeForceDestroyRequest(brokenMachineID)))       // create a force-destroy request
			rmock.Destroy[*omni.ClusterMachineConfig](ctx, t, testContext.State, []string{brokenMachineID})           // destroy the broken machine
			rtestutils.AssertNoResource[*omni.ClusterMachineConfigStatus](ctx, t, testContext.State, brokenMachineID) // assert the broken machine is reset
			rtestutils.AssertNoResource[*omni.NodeForceDestroyRequest](ctx, t, testContext.State, brokenMachineID)    // assert the force-destroy request is cleaned up after force-destroy was done
		})
	})

	// Create a cluster with a single node and check that upgrades API is called with the correct parameters.
	t.Run("upgrades", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers, func(ctx context.Context, testContext testutils.TestContext) {
			clusterName := "test-upgrades"

			machineServices := testutils.NewMachineServices(t, testContext.State)

			cluster, machines := createCluster(ctx, t, testContext.State, machineServices, clusterName, 1, 0)

			awaitAllMachinesConfigured(ctx, t, testContext.State, clusterName)

			machineServices.ForEach(func(m *testutils.MachineServiceMock) {
				m.OnUpdate = upgradeHandler
			})

			const (
				expectedTalosVersion = "1.10.1"
				expectedSchematicID  = "cccc"
			)

			rmock.MockList[*omni.MachineConfigGenOptions](ctx, t, testContext.State,
				options.QueryIDs[*omni.MachineSetNode](resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
				options.ItemOptions(
					options.Modify(func(res *omni.MachineConfigGenOptions) error {
						res.TypedSpec().Value.InstallImage.SchematicId = expectedSchematicID
						res.TypedSpec().Value.InstallImage.TalosVersion = expectedTalosVersion

						return nil
					}),
				),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, rtestutils.ResourceIDs[*omni.MachineStatus](ctx, t, testContext.State),
				func(res *omni.MachineStatus, assert *assert.Assertions) {
					assert.Equal(expectedTalosVersion, res.TypedSpec().Value.TalosVersion)
					assert.Equal(expectedSchematicID, res.TypedSpec().Value.Schematic.Id)
				},
			)

			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				metaDeletes := machineServices.Get(machines[0].Metadata().ID()).GetMetaDeleteKeyToCount()

				if !assert.Len(collect, metaDeletes, 1) {
					return
				}

				count, ok := metaDeletes[meta.Upgrade]

				if !assert.True(collect, ok) {
					return
				}

				if assert.Greater(collect, count, 0) {
					t.Logf("upgrade meta key deletes: %v", count)
				}
			}, time.Second*5, 50*time.Millisecond)
		})
	})

	// Verifies staged upgrade mode (workaround for Talos 1.9.1).
	t.Run("stagedUpgrade", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers, func(ctx context.Context, testContext testutils.TestContext) {
			clusterName := "test-staged-upgrade"

			actualTalosVersion := "1.9.1"

			machineServices := testutils.NewMachineServices(t, testContext.State)

			_, machines := createCluster(ctx, t, testContext.State, machineServices, clusterName, 1, 0, options.WithTalosVersion(actualTalosVersion))

			machineServices.ForEach(func(m *testutils.MachineServiceMock) {
				m.OnUpdate = upgradeHandler
			})

			ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
				return m.Metadata().ID()
			})

			rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, testContext.State,
				options.WithID(ids[0]),
				options.Modify(func(res *omni.MachineConfigGenOptions) error {
					res.TypedSpec().Value.InstallImage.TalosVersion = "1.9.3"

					return nil
				}),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, ids,
				func(res *omni.MachineStatus, assert *assert.Assertions) {
					assert.Equal("1.9.3", res.TypedSpec().Value.TalosVersion)
				},
			)

			requests := machineServices.Get(ids[0]).GetUpgradeRequests()

			if assert.NotEmpty(t, requests, "no upgrade requests received") {
				for _, request := range requests {
					assert.True(t, request.Stage, "expected staged upgrade")
				}
			}
		})
	})

	// Creates a cluster with a single node, changes the schematic, checks that schematic is updated.
	// Updates the machine status to have invalid schematic, checks that the install image is using ghcr.io registry image.
	t.Run("schematicChange", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers, func(ctx context.Context, testContext testutils.TestContext) {
			clusterName := "test-upgrades"

			machineServices := testutils.NewMachineServices(t, testContext.State)

			cluster, machines := createCluster(ctx, t, testContext.State, machineServices, clusterName, 1, 0, options.WithTalosVersion("1.10.0"))

			ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
				return m.Metadata().ID()
			})

			machineServices.ForEach(func(m *testutils.MachineServiceMock) {
				m.OnUpdate = upgradeHandler
			})

			awaitAllMachinesConfigured(ctx, t, testContext.State, clusterName)

			rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, testContext.State,
				options.WithID(ids[0]),
				options.Modify(func(res *omni.MachineConfigGenOptions) error {
					res.TypedSpec().Value.InstallImage.SchematicId = "bbbb"

					return nil
				}),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.MachineStatus, assert *assert.Assertions) {
				assert.Equal(res.TypedSpec().Value.Schematic.Id, "bbbb")
			})

			rmock.Mock[*omni.MachineStatus](ctx, t, testContext.State,
				options.WithID(ids[0]),
				options.Modify(func(res *omni.MachineStatus) error {
					res.TypedSpec().Value.Schematic.Invalid = true

					return nil
				}),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.MachineStatus, assert *assert.Assertions) {
				assert.Empty(res.TypedSpec().Value.Schematic.Id)
				assert.Equal(cluster.TypedSpec().Value.TalosVersion, res.TypedSpec().Value.TalosVersion)
			})
		})
	})

	// Creates a cluster with the machine with secure boot mode enabled.
	// Install image should have `installer-secureboot` path.
	t.Run("secureBootInstallImage", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers, func(ctx context.Context, testContext testutils.TestContext) {
			talosVersion := "1.9.0"

			clusterName := "test-secure-boot-install-image"

			machineServices := testutils.NewMachineServices(t, testContext.State)

			_, machines := createCluster(ctx, t, testContext.State, machineServices, clusterName, 1, 0, options.WithTalosVersion(talosVersion))

			ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
				return m.Metadata().ID()
			})

			machineServices.ForEach(func(m *testutils.MachineServiceMock) {
				m.OnUpdate = upgradeHandler
			})

			awaitAllMachinesConfigured(ctx, t, testContext.State, clusterName)

			rmock.MockList[*omni.MachineConfigGenOptions](ctx, t, testContext.State,
				options.QueryIDs[*omni.MachineSetNode](),
				options.ItemOptions(
					options.Modify(func(res *omni.MachineConfigGenOptions) error {
						res.TypedSpec().Value.InstallImage.SecurityState = &specs.SecurityState{
							SecureBoot:    true,
							BootedWithUki: true,
						}
						res.TypedSpec().Value.InstallImage.SchematicId = "bbbb"

						return nil
					}),
				),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.MachineStatus, assert *assert.Assertions) {
				assert.Equal(res.TypedSpec().Value.Schematic.Id, "bbbb")
			})

			requests := machineServices.Get(ids[0]).GetUpgradeRequests()

			require.NotEmpty(t, requests)

			// expected installer image without the platform prepended to it, as the Talos version is < 1.10.0
			expectedImage := imageFactoryHost + "/installer-secureboot/bbbb:v" + talosVersion
			for _, r := range requests {
				if r.Image != expectedImage {
					assert.Equal(t, expectedImage, r.Image)
				}
			}
		})
	})

	// Generate the ClusterMachineConfig with the error.
	// Check that ClusterMachineConfigStatus has the error copied from the ClusterMachineConfig.
	t.Run("generationErrorPropagation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers, func(ctx context.Context, testContext testutils.TestContext) {
			clusterName := "test-generation-error-propagation"

			_, machines := createCluster(ctx, t, testContext.State, testutils.NewMachineServices(t, testContext.State), clusterName, 1, 0)

			rmock.Mock[*omni.ClusterMachineConfig](ctx, t, testContext.State,
				options.WithID(machines[0].Metadata().ID()),
				options.Modify(func(res *omni.ClusterMachineConfig) error {
					res.TypedSpec().Value.GenerationError = "TestGenerationErrorPropagation error"

					return nil
				}),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, []string{machines[0].Metadata().ID()},
				func(res *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
					assert.Equal("TestGenerationErrorPropagation error", res.TypedSpec().Value.LastConfigError)
				},
			)
		})
	})

	// Wait for the initial configs to be created.
	// Lock the machine.
	// Update the config, check that lock works.
	// Unlock the machine see the config being applied.
	t.Run("applyLocked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "apply-reset", 3, 3)

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.OnReset = resetHandler
				})

				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				rmock.MockList[*omni.MachineSetNode](ctx, t, testContext.State,
					options.IDs(ids),
					options.ItemOptions(
						options.Modify(func(r *omni.MachineSetNode) error {
							r.Metadata().Annotations().Set(omni.MachineLocked, "")

							return nil
						}),
					),
				)

				awaitAllMachinesConfigured(ctx, t, testContext.State, cluster.Metadata().ID())

				machineServices.ForEach(func(machineServer *testutils.MachineServiceMock) {
					assert.GreaterOrEqual(t, len(machineServer.GetApplyRequests()), 1)
				})

				rmock.MockList[*omni.ClusterMachineConfig](ctx, t, testContext.State,
					options.IDs(ids),
					options.ItemOptions(
						options.Modify(func(r *omni.ClusterMachineConfig) error {
							return r.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  network:
    kubespan:
      enabled: true`))
						}),
					),
				)

				rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.MachinePendingUpdates, assert *assert.Assertions) {
					assert.NotEmpty(res.TypedSpec().Value.ConfigDiff)
				})

				// unlock all machines
				rmock.MockList[*omni.MachineSetNode](ctx, t, testContext.State,
					options.IDs(ids),
					options.ItemOptions(
						options.Modify(func(r *omni.MachineSetNode) error {
							r.Metadata().Annotations().Delete(omni.MachineLocked)

							return nil
						}),
					),
				)

				for _, id := range ids {
					rtestutils.AssertNoResource[*omni.MachinePendingUpdates](ctx, t, testContext.State, id)
				}

				rmock.Destroy[*omni.ClusterMachineConfig](ctx, t, testContext.State, ids)

				for _, m := range ids {
					rtestutils.AssertNoResource[*omni.ClusterMachineConfigStatus](ctx, t, testContext.State, m)
				}

				machineServices.ForEach(func(machineServer *testutils.MachineServiceMock) {
					assert.GreaterOrEqual(t, len(machineServer.GetResetRequests()), 1)
				})
			},
		)
	})

	// Test graceful rollout with max parallelism.
	t.Run("gracefulConfigRollout", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				clusterName := "graceful"

				machineServices := testutils.NewMachineServices(t, testContext.State)

				_, machines := createCluster(ctx, t, testContext.State, machineServices, clusterName, 3, 3)

				var mu sync.Mutex

				mockReboot := map[string]struct{}{}

				awaitAllMachinesConfigured(ctx, t, testContext.State, clusterName)

				// faking reboot mode on the apply config calls
				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.OnApplyConfig = func(_ context.Context, _ *machine.ApplyConfigurationRequest, _ state.State, id string) (*machine.ApplyConfigurationResponse, error) {
						mu.Lock()

						_, reboot := mockReboot[id]

						mu.Unlock()

						mode := machine.ApplyConfigurationRequest_NO_REBOOT
						if reboot {
							mode = machine.ApplyConfigurationRequest_REBOOT
						}

						return &machine.ApplyConfigurationResponse{
							Messages: []*machine.ApplyConfiguration{
								{
									Mode: mode,
								},
							},
						}, nil
					}
				})

				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					mu.Lock()
					defer mu.Unlock()

					mockReboot[m.Metadata().ID()] = struct{}{}

					return m.Metadata().ID()
				})

				cfg := `machine:
  network:
    kubespan:
      enabled: true`

				rmock.MockList[*omni.ClusterMachineConfig](ctx, t, testContext.State,
					options.IDs(ids),
					options.ItemOptions(
						options.Modify(func(r *omni.ClusterMachineConfig) error {
							return r.TypedSpec().Value.SetUncompressedData([]byte(cfg))
						}),
					),
				)

				// as we have 3 machines do the routine 3 times
				for range 3 {
					getUpdatedID := func() string {
						var updatedID string

						require.EventuallyWithT(t, func(collect *assert.CollectT) {
							clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, testContext.State)
							require.NoError(t, err)

							for machine := range clusterMachines.All() {
								if machine.Metadata().Finalizers().Has(machineconfig.ConfigUpdateFinalizer) {
									updatedID = machine.Metadata().ID()

									break
								}
							}

							assert.NotEmpty(collect, updatedID)
						}, time.Second*5, time.Millisecond*100)

						return updatedID
					}

					// wait for the machine to start updating
					updatedID := getUpdatedID()

					// update was started, drop the id from the id list
					ids = slices.DeleteFunc(ids, func(id string) bool {
						return id == updatedID
					})

					if len(ids) != 0 {
						rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.MachinePendingUpdates, assert *assert.Assertions) {
							assert.NotEmpty(res.TypedSpec().Value.ConfigDiff)
						})
					}

					mu.Lock()
					delete(mockReboot, updatedID)
					mu.Unlock()

					// poke machine set node to trigger update after we switch the updatedID to NO_REBOOT mode
					rmock.Mock[*omni.MachineSetNode](ctx, t, testContext.State,
						options.WithID(updatedID),
						options.EmptyLabel("poke"),
					)

					rtestutils.AssertResources(ctx, t, testContext.State, []string{updatedID}, func(res *omni.ClusterMachine, assert *assert.Assertions) {
						assert.False(res.Metadata().Finalizers().Has(machineconfig.ConfigUpdateFinalizer))
					})
				}

				config, err := configloader.NewFromBytes([]byte(cfg))
				require.NoError(t, err)

				rawConfig, err := config.RedactSecrets(x509.Redacted).EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
				require.NoError(t, err)

				rtestutils.AssertAll(ctx, t, testContext.State, func(res *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
					buf, err := res.TypedSpec().Value.GetUncompressedData()

					assert.NoError(err)

					if err == nil {
						defer buf.Free()
					}

					assert.EqualValues(string(rawConfig), string(buf.Data()))
				})
			},
		)
	})
}

func createCluster(
	ctx context.Context,
	t *testing.T,
	st state.State,
	machineServices *testutils.MachineServices,
	clusterName string,
	controlPlanes, workers int,
	opts ...options.MockOption,
) (*omni.Cluster, []*omni.ClusterMachine) {
	clusterOptions := append([]options.MockOption{
		options.WithID(clusterName),
	}, opts...)

	cluster := rmock.Mock[*omni.Cluster](ctx, t, st,
		clusterOptions...,
	)

	rmock.Mock[*omni.ClusterSecrets](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.ClusterConfigVersion](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.TalosConfig](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.ClusterStatus](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.LoadBalancerConfig](ctx, t, st, options.SameID(cluster))

	cpMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.ControlPlanesResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelControlPlaneRole),
	)

	workersMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.WorkersResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelWorkerRole),
	)

	getIDs := func(count int) []string {
		res := make([]string, 0, count)

		for i := range count {
			res = append(res, fmt.Sprintf("node-%d", i))
		}

		return res
	}

	rmock.MockList[*omni.MachineSetStatus](ctx, t, st,
		options.IDs([]string{
			cpMachineSet.Metadata().ID(),
			workersMachineSet.Metadata().ID(),
		}),
	)

	// create control planes
	rmock.MockList[*omni.MachineSetNode](ctx, t, st,
		options.IDs(getIDs(controlPlanes)),
		options.ItemOptions(
			options.LabelCluster(cluster),
			options.LabelMachineSet(cpMachineSet),
			options.EmptyLabel(omni.LabelControlPlaneRole),
		),
	)

	if workers > 0 {
		// create workers
		rmock.MockList[*omni.MachineSetNode](ctx, t, st,
			options.IDs(getIDs(workers)),
			options.ItemOptions(
				options.LabelCluster(cluster),
				options.LabelMachineSet(workersMachineSet),
				options.EmptyLabel(omni.LabelWorkerRole),
			),
		)
	}

	machines := rmock.MockList[*omni.ClusterMachine](ctx, t, st,
		options.QueryIDs[*omni.MachineSetNode](),
		options.ItemOptions(
			options.LabelCluster(cluster),
			options.LabelMachineSet(workersMachineSet),
			options.Modify(
				func(res *omni.ClusterMachine) error {
					res.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

					return nil
				},
			),
		),
	)

	for _, machine := range machines {
		rmock.Mock[*siderolink.MachineJoinConfig](ctx, t, st, options.SameID(machine),
			options.Modify(func(res *siderolink.MachineJoinConfig) error {
				res.TypedSpec().Value.Config = &specs.JoinConfig{}

				return nil
			}),
		)

		rmock.Mock[*siderolink.Link](ctx, t, st, options.SameID(machine),
			options.Modify(func(res *siderolink.Link) error {
				res.TypedSpec().Value.Connected = true

				return nil
			}),
		)

		rmock.Mock[*omni.MachineStatus](ctx, t, st, options.SameID(machine),
			options.Modify(func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Maintenance = false

				return nil
			}),
			options.WithMachineServices(ctx, machineServices),
		)

		rmock.Mock[*omni.Machine](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.ClusterMachineConfigPatches](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.ClusterMachineConfig](ctx, t, st, options.SameID(machine))
	}

	return cluster, machines
}
