// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterMachineConfigStatusSuite struct {
	OmniSuite
}

func (suite *ClusterMachineConfigStatusSuite) TestApplyReset() {
	suite.startRuntime()

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "talos-default-4"

	cluster, machines := suite.createCluster(clusterName, 3, 0)

	suite.prepareMachines(machines)

	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata(),
			func(res *omni.ClusterMachineConfigStatus, assertions *assert.Assertions) {
				assertions.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256, "the machine is not configured yet")
			},
		)
	}

	for _, m := range machines {
		statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, statusSnapshot.Metadata(), func(res *omni.MachineStatusSnapshot) error {
			res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
				Stage: machine.MachineStatusEvent_RUNNING,
			}

			return nil
		})

		suite.Require().NoError(err)
	}

	suite.Assert().GreaterOrEqual(len(suite.machineService.getApplyRequests()), len(machines))

	go func() {
		for _, m := range machines {
			<-suite.machineService.resetChan

			suite.T().Logf("setting maintenance for %v", m.Metadata().ID())

			statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

			_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, statusSnapshot.Metadata(),
				func(res *omni.MachineStatusSnapshot) error {
					res.TypedSpec().Value.MachineStatus.Stage = machine.MachineStatusEvent_MAINTENANCE

					return nil
				},
			)

			suite.Require().NoError(err)
		}
	}()

	time.Sleep(time.Second * 1)
	suite.destroyCluster(cluster)

	for _, m := range machines {
		suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(*omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata()),
		))
	}

	suite.Assert().GreaterOrEqual(len(suite.machineService.getResetRequests()), len(machines))
}

func (suite *ClusterMachineConfigStatusSuite) TestResetMachineRemoved() {
	suite.startRuntime()

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "machine-ungraceful-reset"

	cluster, machines := suite.createCluster(clusterName, 2, 0)

	suite.prepareMachines(machines)

	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata(),
			func(res *omni.ClusterMachineConfigStatus, assertions *assert.Assertions) {
				assertions.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256, "the machine is not configured yet")
			},
		)
	}

	for _, m := range machines {
		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, resource.NewMetadata(
			m.Metadata().Namespace(),
			omni.MachineStatusType,
			m.Metadata().ID(),
			resource.VersionUndefined,
		),
			func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Connected = false

				return nil
			},
		)

		suite.Require().NoError(err)
	}

	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata(),
			func(*omni.ClusterMachineConfigStatus, *assert.Assertions) {},
		)
	}

	rtestutils.Teardown[*omni.Cluster](suite.ctx, suite.T(), suite.state, []string{cluster.Metadata().ID()})

	for _, m := range machines {
		rtestutils.Teardown[*omni.ClusterMachine](suite.ctx, suite.T(), suite.state, []string{m.Metadata().ID()})
	}

	for _, m := range machines {
		rtestutils.Destroy[*omni.MachineStatus](suite.ctx, suite.T(), suite.state, []string{m.Metadata().ID()})
		rtestutils.Teardown[*omni.Machine](suite.ctx, suite.T(), suite.state, []string{m.Metadata().ID()})

		suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(*omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata()),
		))
	}
}

func (suite *ClusterMachineConfigStatusSuite) prepareMachines(machines []*omni.ClusterMachine) {
	for _, m := range machines {
		statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

		statusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
			Stage: machine.MachineStatusEvent_RUNNING,
		}

		suite.Require().NoError(suite.state.Create(suite.ctx, statusSnapshot))

		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, resource.NewMetadata(
			m.Metadata().Namespace(),
			omni.MachineStatusType,
			m.Metadata().ID(),
			resource.VersionUndefined,
		),
			func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Connected = true
				res.TypedSpec().Value.Maintenance = false
				res.TypedSpec().Value.SecurityState = &specs.SecurityState{}
				res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
					Platform: talosconstants.PlatformMetal,
				}

				return nil
			},
		)

		suite.Require().NoError(err)
	}
}

func (suite *ClusterMachineConfigStatusSuite) TestResetUngraceful() {
	suite.startRuntime()

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "talos-default-5"

	machineServices := map[resource.ID]*machineService{}

	cluster, machines := suite.createCluster(clusterName, 3, 0)

	brokenEtcdMachine := 2

	for i, m := range machines {
		statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

		machineService, err := suite.newServer(m.Metadata().ID())
		suite.Require().NoError(err)

		if i == brokenEtcdMachine {
			machineService.lock.Lock()
			machineService.etcdLeaveClusterHandler = func(context.Context, *machine.EtcdLeaveClusterRequest) (*machine.EtcdLeaveClusterResponse, error) {
				return nil, errors.New("sowwy I'm bwoken")
			}
			machineService.lock.Unlock()
		}

		statusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
			Stage: machine.MachineStatusEvent_BOOTING,
		}

		suite.Require().NoError(suite.state.Create(suite.ctx, statusSnapshot))

		_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, resource.NewMetadata(
			m.Metadata().Namespace(),
			omni.MachineStatusType,
			m.Metadata().ID(),
			resource.VersionUndefined,
		),
			func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Connected = true
				res.TypedSpec().Value.Maintenance = false
				res.TypedSpec().Value.ManagementAddress = unixSocket + machineService.address
				res.TypedSpec().Value.SecurityState = &specs.SecurityState{}
				res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
					Platform: talosconstants.PlatformMetal,
				}

				return nil
			},
		)

		suite.Require().NoError(err)

		machineServices[m.Metadata().ID()] = machineService
	}

	for _, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata(),
			func(res *omni.ClusterMachineConfigStatus, assertions *assert.Assertions) {
				assertions.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256, "the machine is not configured yet")
			},
		)

		statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, statusSnapshot.Metadata(), func(res *omni.MachineStatusSnapshot) error {
			res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
				Stage: machine.MachineStatusEvent_RUNNING,
			}

			return nil
		})

		suite.Require().NoError(err)
	}

	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()

	for _, m := range machines {
		machineService := machineServices[m.Metadata().ID()]
		id := m.Metadata().ID()

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case req := <-machineService.resetChan:
					statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, id)

					_, err := safe.StateUpdateWithConflicts(ctx, suite.state, statusSnapshot.Metadata(),
						func(*omni.MachineStatusSnapshot) error {
							// poke machine status to trigger reconciliation
							_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, statusSnapshot.Metadata(),
								func(res *omni.MachineStatusSnapshot) error {
									if req.Graceful {
										res.Metadata().Annotations().Set("random", uuid.New().String())

										return nil
									}

									suite.T().Logf("setting maintenance for %v", id)

									// set to maintenance only if non graceful reset was requested
									res.TypedSpec().Value.MachineStatus.Stage = machine.MachineStatusEvent_MAINTENANCE

									return nil
								},
							)

							return err
						},
					)

					suite.Require().NoError(err)
				}
			}
		}()
	}

	time.Sleep(time.Second * 1)
	suite.destroyCluster(cluster)

	for _, m := range machines {
		suite.Assert().NoError(retry.Constant(30*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(*omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata()),
		))
	}

	for _, m := range machines {
		count := 5

		suite.Assert().Len(machineServices[m.Metadata().ID()].getResetRequests(), count)
	}
}

func (suite *ClusterMachineConfigStatusSuite) TestForceDestroy() {
	suite.startRuntime()

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "force-destroy"
	_, machines := suite.createCluster(clusterName, 3, 0)
	brokenMachineID := machines[2].Metadata().ID()

	for _, m := range machines {
		machineSvc, err := suite.newServer(m.Metadata().ID())
		suite.Require().NoError(err)

		statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

		if m.Metadata().ID() == brokenMachineID {
			statusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{ // set the broken machine stuck in the booting stage
				Stage: machine.MachineStatusEvent_BOOTING,
			}

			machineSvc.lock.Lock()
			machineSvc.etcdLeaveClusterHandler = func(context.Context, *machine.EtcdLeaveClusterRequest) (*machine.EtcdLeaveClusterResponse, error) {
				return nil, errors.New("cannot leave, wasn't part of it")
			}
			machineSvc.onReset = func(context.Context, *machine.ResetRequest) (*machine.ResetResponse, error) {
				_, updateErr := safe.StateUpdateWithConflicts(suite.ctx, suite.state, statusSnapshot.Metadata(), func(res *omni.MachineStatusSnapshot) error { //nolint:contextcheck
					res.TypedSpec().Value.MachineStatus.Stage = machine.MachineStatusEvent_MAINTENANCE

					return nil
				})

				return &machine.ResetResponse{}, updateErr
			}
			machineSvc.lock.Unlock()
		} else {
			statusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{ // set the healthy machines to the running stage
				Stage: machine.MachineStatusEvent_RUNNING,
			}
		}

		suite.Require().NoError(suite.state.Create(suite.ctx, statusSnapshot))

		_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, resource.NewMetadata(
			m.Metadata().Namespace(),
			omni.MachineStatusType,
			m.Metadata().ID(),
			resource.VersionUndefined,
		),
			func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Connected = true
				res.TypedSpec().Value.Maintenance = false
				res.TypedSpec().Value.ManagementAddress = unixSocket + machineSvc.address
				res.TypedSpec().Value.SecurityState = &specs.SecurityState{}
				res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
					Platform: talosconstants.PlatformMetal,
				}

				return nil
			},
		)

		suite.Require().NoError(err)
	}

	require.NoError(suite.T(), suite.state.Create(suite.ctx, omni.NewNodeForceDestroyRequest(brokenMachineID)))       // create a force-destroy request
	rtestutils.Destroy[*omni.ClusterMachine](suite.ctx, suite.T(), suite.state, []string{brokenMachineID})            // destroy the broken machine
	rtestutils.AssertNoResource[*omni.ClusterMachineConfigStatus](suite.ctx, suite.T(), suite.state, brokenMachineID) // assert the broken machine is reset
	rtestutils.AssertNoResource[*omni.NodeForceDestroyRequest](suite.ctx, suite.T(), suite.state, brokenMachineID)    // assert the force-destroy request is cleaned up after force-destroy was done
}

func (suite *ClusterMachineConfigStatusSuite) TestUpgrades() {
	suite.startRuntime()

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "test-upgrades"

	cluster, machines := suite.createCluster(clusterName, 1, 0)

	for _, m := range machines {
		talosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, m.Metadata().ID())
		talosVersion.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
		suite.Require().NoError(suite.state.Create(suite.ctx, talosVersion))

		statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

		statusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
			Stage: machine.MachineStatusEvent_RUNNING,
		}

		suite.Require().NoError(suite.state.Create(suite.ctx, statusSnapshot))

		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, resource.NewMetadata(
			m.Metadata().Namespace(),
			omni.MachineStatusType,
			m.Metadata().ID(),
			resource.VersionUndefined,
		),
			func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Connected = true
				res.TypedSpec().Value.Maintenance = false
				res.TypedSpec().Value.SecurityState = &specs.SecurityState{}
				res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
					Platform: talosconstants.PlatformMetal,
				}

				return nil
			},
		)

		suite.Require().NoError(err)
	}

	const (
		expectedTalosVersion = "1.10.1"
		expectedSchematicID  = "cccc"
	)

	for _, m := range machines {
		talosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, m.Metadata().ID())
		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, talosVersion.Metadata(), func(res *omni.ClusterMachineTalosVersion) error {
			res.TypedSpec().Value.TalosVersion = expectedTalosVersion
			res.TypedSpec().Value.SchematicId = expectedSchematicID

			return nil
		})
		suite.Require().NoError(err)
	}

	suite.Require().NoError(retry.Constant(time.Second * 5).Retry(func() error {
		requests := suite.machineService.getUpgradeRequests()
		if len(requests) == 0 {
			return retry.ExpectedErrorf("no upgrade requests received")
		}

		expectedImage := fmt.Sprintf("factory-test.talos.dev/metal-installer/%s:v%s", expectedSchematicID, expectedTalosVersion)
		for i, r := range requests {
			if r.Image != expectedImage {
				return fmt.Errorf("%d request image is invalid: expected %q got %q", i, expectedImage, r.Image)
			}

			if r.Stage {
				suite.Require().Fail("unexpected stage request")
			}
		}

		return nil
	}))

	// simulate a finished upgrade by setting related resources to have expected values.
	// this satisfies (versionMismatch==false), and makes the controller proceed with the post-upgrade steps.
	for _, m := range machines {
		cmcs := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID())

		cmcs.TypedSpec().Value.TalosVersion = expectedTalosVersion
		cmcs.TypedSpec().Value.SchematicId = expectedSchematicID

		suite.Require().NoError(suite.state.Create(suite.ctx, cmcs))

		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state,
			omni.NewMachineStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata(),
			func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.TalosVersion = expectedTalosVersion
				res.TypedSpec().Value.Schematic.Id = expectedSchematicID

				return nil
			})
		suite.Require().NoError(err)
	}

	require.EventuallyWithT(suite.T(), func(collect *assert.CollectT) {
		metaDeletes := suite.machineService.getMetaDeleteKeyToCount()

		if !assert.Len(collect, metaDeletes, 1) {
			return
		}

		count, ok := metaDeletes[meta.Upgrade]

		if !assert.True(collect, ok) {
			return
		}

		if assert.Greater(collect, count, 0) {
			suite.T().Logf("upgrade meta key deletes: %v", count)
		}
	}, time.Second*5, 50*time.Millisecond)
}

func (suite *ClusterMachineConfigStatusSuite) TestStagedUpgrade() {
	suite.startRuntime()

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	// choose a Talos version affected by the readiness bug, so it will trigger a staged upgrade
	actualTalosVersion := "1.9.1"

	clusterName := "test-staged-upgrade"
	cluster, machines := suite.createClusterWithTalosVersion(clusterName, 1, 0, actualTalosVersion)

	suite.Require().Len(machines, 1)

	clusterMachine := machines[0]

	talosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, clusterMachine.Metadata().ID())
	talosVersion.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
	suite.Require().NoError(suite.state.Create(suite.ctx, talosVersion))

	statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, clusterMachine.Metadata().ID())

	statusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
		Stage: machine.MachineStatusEvent_RUNNING,
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, statusSnapshot))

	machineSvc, err := suite.newServerWithTalosVersion("-staged-upgrade", actualTalosVersion)
	suite.Require().NoError(err)

	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(), func(res *omni.MachineStatus) error {
		res.TypedSpec().Value.Connected = true
		res.TypedSpec().Value.Maintenance = false
		res.TypedSpec().Value.ManagementAddress = unixSocket + machineSvc.address
		res.TypedSpec().Value.SecurityState = &specs.SecurityState{}
		res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
			Platform: talosconstants.PlatformMetal,
		}

		return nil
	},
	)
	suite.Require().NoError(err)

	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, talosVersion.Metadata(), func(res *omni.ClusterMachineTalosVersion) error {
		res.TypedSpec().Value.TalosVersion = "1.9.3"

		return nil
	})
	suite.Require().NoError(err)

	require.EventuallyWithT(suite.T(), func(collect *assert.CollectT) {
		suite.Require().NoError(suite.ctx.Err())

		requests := machineSvc.getUpgradeRequests()

		if assert.NotEmpty(collect, requests, "no upgrade requests received") {
			for _, request := range requests {
				assert.True(collect, request.Stage, "expected staged upgrade")
			}
		}
	}, time.Second*30, 100*time.Millisecond)
}

func (suite *ClusterMachineConfigStatusSuite) TestSchematicChanges() {
	suite.startRuntime()

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "test-upgrades"

	cluster, machines := suite.createClusterWithTalosVersion(clusterName, 1, 0, "1.10.0")

	for _, m := range machines {
		talosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, m.Metadata().ID())
		talosVersion.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
		suite.Require().NoError(suite.state.Create(suite.ctx, talosVersion))

		statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

		statusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
			Stage: machine.MachineStatusEvent_RUNNING,
		}

		suite.Require().NoError(suite.state.Create(suite.ctx, statusSnapshot))

		_, err := safe.StateUpdateWithConflicts[*omni.ClusterMachineTalosVersion](suite.ctx, suite.state, talosVersion.Metadata(), func(res *omni.ClusterMachineTalosVersion) error {
			res.TypedSpec().Value.SchematicId = "ffff"

			return nil
		})

		suite.Require().NoError(err)

		_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, resource.NewMetadata(
			m.Metadata().Namespace(),
			omni.MachineStatusType,
			m.Metadata().ID(),
			resource.VersionUndefined,
		),
			func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Connected = true
				res.TypedSpec().Value.Maintenance = false
				res.TypedSpec().Value.SecurityState = &specs.SecurityState{}
				res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
					Platform: talosconstants.PlatformMetal,
				}

				return nil
			},
		)

		suite.Require().NoError(err)
	}

	for _, m := range machines {
		talosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, m.Metadata().ID())
		_, err := safe.StateUpdateWithConflicts[*omni.ClusterMachineTalosVersion](suite.ctx, suite.state, talosVersion.Metadata(), func(res *omni.ClusterMachineTalosVersion) error {
			res.TypedSpec().Value.SchematicId = "bbbb"

			return nil
		})

		suite.Require().NoError(err)
		suite.Require().NoError(err)
	}

	expectedFactoryImage := imageFactoryHost + "/metal-installer/bbbb:v1.10.0" // expected image has the platform prefixed, as the Talos version is >= 1.10.0

	suite.Require().NoError(retry.Constant(time.Second * 5).Retry(func() error {
		requests := suite.machineService.getUpgradeRequests()
		if len(requests) == 0 {
			return retry.ExpectedErrorf("no upgrade requests received")
		}

		for i, r := range requests {
			if r.Image != expectedFactoryImage {
				return fmt.Errorf("%d request image is invalid: expected %q got %q", i, expectedFactoryImage, r.Image)
			}
		}

		return nil
	}))

	// check fallback to ghcr image if the schematic is invalid
	for _, m := range machines {
		machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, m.Metadata().ID())
		_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
			res.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
				Invalid: true,
			}

			return nil
		})
		suite.Require().NoError(err)
	}

	trimLeadingImages := func(images []string, trim string) []string {
		for i, image := range images {
			if image != trim {
				return images[i:]
			}
		}

		return nil
	}

	suite.Require().NoError(retry.Constant(time.Second * 5).Retry(func() error {
		requests := suite.machineService.getUpgradeRequests()
		images := xslices.Map(requests, func(r *machine.UpgradeRequest) string { return r.Image })
		trimmedImages := trimLeadingImages(images, expectedFactoryImage)

		if len(trimmedImages) == 0 {
			return retry.ExpectedErrorf("no new upgrade requests received")
		}

		expectedImage := "ghcr.io/siderolabs/installer:v1.10.0"
		for i, image := range trimmedImages {
			if image != expectedImage {
				return fmt.Errorf("%d request image is invalid: expected %q got %q", i, expectedImage, image)
			}
		}

		return nil
	}))
}

func (suite *ClusterMachineConfigStatusSuite) TestSecureBootInstallImage() {
	suite.T().Cleanup(suite.machineService.clearUpgradeRequests)

	suite.startRuntime()

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "test-secure-boot-install-image"

	cluster, machines := suite.createCluster(clusterName, 1, 0)

	for _, m := range machines {
		talosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, m.Metadata().ID())
		talosVersion.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
		suite.Require().NoError(suite.state.Create(suite.ctx, talosVersion))

		statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, m.Metadata().ID())

		statusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
			Stage: machine.MachineStatusEvent_RUNNING,
		}

		suite.Require().NoError(suite.state.Create(suite.ctx, statusSnapshot))

		_, err := safe.StateUpdateWithConflicts[*omni.ClusterMachineTalosVersion](suite.ctx, suite.state, talosVersion.Metadata(), func(res *omni.ClusterMachineTalosVersion) error {
			res.TypedSpec().Value.SchematicId = "abcd"

			return nil
		})

		suite.Require().NoError(err)

		_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, resource.NewMetadata(
			m.Metadata().Namespace(),
			omni.MachineStatusType,
			m.Metadata().ID(),
			resource.VersionUndefined,
		),
			func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Connected = true
				res.TypedSpec().Value.Maintenance = false
				res.TypedSpec().Value.SecurityState = &specs.SecurityState{
					SecureBoot:    true,
					BootedWithUki: true,
				}
				res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
					Platform: talosconstants.PlatformMetal,
				}
				res.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
					FullId: "abcd",
				}

				return nil
			},
		)

		suite.Require().NoError(err)
	}

	suite.Require().NoError(retry.Constant(time.Second * 5).Retry(func() error {
		requests := suite.machineService.getUpgradeRequests()
		if len(requests) == 0 {
			return retry.ExpectedErrorf("no upgrade requests received")
		}

		expectedImage := imageFactoryHost + "/installer-secureboot/abcd:v1.3.0" // expected installer image without the platform prepended to it, as the Talos version is < 1.10.0
		for i, r := range requests {
			if r.Image != expectedImage {
				return fmt.Errorf("%d request image is invalid: expected %q got %q", i, expectedImage, r.Image)
			}
		}

		return nil
	}))
}

func (suite *ClusterMachineConfigStatusSuite) TestGenerationErrorPropagation() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigStatusController(imageFactoryHost)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))

	clusterName := "test-generation-error-propagation"

	_, machines := suite.createCluster(clusterName, 1, 0)

	suite.Require().Greater(len(machines), 0)

	m := machines[0]

	clusterMachineConfig := omni.NewClusterMachineConfig(resources.DefaultNamespace, m.Metadata().ID())
	clusterMachineConfig.TypedSpec().Value.GenerationError = "TestGenerationErrorPropagation error"

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachineConfig))

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, m.Metadata().ID()).Metadata(),
		func(sts *omni.ClusterMachineConfigStatus, assertions *assert.Assertions) {
			assertions.Equal("TestGenerationErrorPropagation error", sts.TypedSpec().Value.LastConfigError)
		},
	)
}

func TestClusterMachineConfigStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterMachineConfigStatusSuite))
}
