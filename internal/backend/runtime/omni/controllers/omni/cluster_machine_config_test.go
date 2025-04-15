// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	conf "github.com/siderolabs/omni/internal/pkg/config"
)

type ClusterMachineConfigSuite struct {
	OmniSuite
}

func (suite *ClusterMachineConfigSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))

	clusterName := "talos-default-2"
	cluster, machines := suite.createClusterWithTalosVersion(clusterName, 1, 1, "1.10.0")

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, machines[0].Metadata().ID()).Metadata(),
		func(config *omni.ClusterMachineConfigPatches) error {
			patches, err := config.TypedSpec().Value.GetUncompressedPatches()
			suite.Require().NoError(err)

			patches = append(patches, `machine:
  network:
    hostname: patched-node`)

			return config.TypedSpec().Value.SetUncompressedPatches(patches)
		},
	)

	suite.Require().NoError(err)

	for i, m := range machines {
		assertResource(
			&suite.OmniSuite,
			*omni.NewClusterMachineConfig(resources.DefaultNamespace, m.Metadata().ID()).Metadata(),
			func(cfg *omni.ClusterMachineConfig, assertions *assert.Assertions) {
				buffer, bufferErr := cfg.TypedSpec().Value.GetUncompressedData()
				suite.Require().NoError(bufferErr)

				defer buffer.Free()

				configData := buffer.Data()

				machineconfig, mcErr := configloader.NewFromBytes(configData)
				suite.Require().NoError(mcErr)

				expectedType := machine.TypeWorker
				if _, ok := m.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
					expectedType = machine.TypeControlPlane
				}

				disk := machineconfig.Machine().Install().Disk()

				assertions.Equal(expectedType, machineconfig.Machine().Type())
				assertions.Equal(testInstallDisk, disk)
				assertions.Equal(
					fmt.Sprintf("%s/%s-installer/%s:v%s", imageFactoryHost, talosconstants.PlatformMetal, defaultSchematic, cluster.TypedSpec().Value.TalosVersion),
					machineconfig.Machine().Install().Image(),
				)

				if i == 0 {
					assertions.Equal(machineconfig.Machine().Network().Hostname(), "patched-node")
				}
			},
		)
	}

	newImage := fmt.Sprintf("%s:v1.0.2", conf.Config.TalosRegistry)

	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, machines[0].Metadata().ID()).Metadata(),
		func(config *omni.ClusterMachineConfigPatches) error {
			patches, patchesErr := config.TypedSpec().Value.GetUncompressedPatches()
			suite.Require().NoError(patchesErr)

			patches = append(patches, `machine:
  install:
    image: `+newImage)

			return config.TypedSpec().Value.SetUncompressedPatches(patches)
		},
	)

	suite.Require().NoError(err)

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfig(resources.DefaultNamespace, machines[0].Metadata().ID()).Metadata(),
		func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			buffer, bufferErr := spec.GetUncompressedData()
			suite.Require().NoError(bufferErr)

			defer buffer.Free()

			configData := buffer.Data()

			machineconfig, configErr := configloader.NewFromBytes(configData)
			suite.Require().NoError(configErr)

			assertions.Equal(newImage, machineconfig.Machine().Install().Image())
		},
	)

	suite.destroyCluster(cluster)

	for _, m := range machines {
		suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(*omni.NewClusterMachineConfig(resources.DefaultNamespace, m.Metadata().ID()).Metadata()),
		))
	}
}

func (suite *ClusterMachineConfigSuite) TestGeneratePreserveFeatures() {
	suite.startRuntime()

	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))

	clusterName := "talos-default-old"
	cluster, machines := suite.createClusterWithTalosVersion(clusterName, 1, 1, "1.2.0")

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, machines[0].Metadata(), func(res *omni.ClusterMachine) error {
		res.Metadata().Annotations().Set(omni.PreserveApidCheckExtKeyUsage, "")
		res.Metadata().Annotations().Set(omni.PreserveDiskQuotaSupport, "")

		return nil
	})

	suite.Require().NoError(err)

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfig(resources.DefaultNamespace, machines[0].Metadata().ID()).Metadata(),
		func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			buffer, bufferErr := spec.GetUncompressedData()
			suite.Require().NoError(bufferErr)

			defer buffer.Free()

			configData := buffer.Data()

			machineconfig, configErr := configloader.NewFromBytes(configData)
			suite.Require().NoError(configErr)

			assertions.True(machineconfig.Machine().Features().ApidCheckExtKeyUsageEnabled())
			assertions.True(machineconfig.Machine().Features().DiskQuotaSupportEnabled())
		},
	)

	suite.destroyCluster(cluster)

	for _, m := range machines {
		suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(*omni.NewClusterMachineConfig(resources.DefaultNamespace, m.Metadata().ID()).Metadata()),
		))
	}
}

func (suite *ClusterMachineConfigSuite) TestGenerationError() {
	suite.startRuntime()

	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, 8090)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))

	clusterName := "test-generation-error"

	_, machines := suite.createCluster(clusterName, 1, 0)
	suite.Require().Greater(len(machines), 0)

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, machines[0].Metadata().ID()).Metadata(),
		func(config *omni.ClusterMachineConfigPatches) error {
			patches, err := config.TypedSpec().Value.GetUncompressedPatches()
			suite.Require().NoError(err)

			patches = append(patches, `machine:
  network:
    interfaces:
      - interface: eth42
        bridge: invalidValueType`)

			return config.TypedSpec().Value.SetUncompressedPatches(patches)
		},
	)

	suite.Require().NoError(err)

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfig(resources.DefaultNamespace, machines[0].Metadata().ID()).Metadata(),
		func(cfg *omni.ClusterMachineConfig, assert *assert.Assertions) {
			expectedError := "yaml: unmarshal errors"

			buffer, bufferErr := cfg.TypedSpec().Value.GetUncompressedData()
			suite.Require().NoError(bufferErr)

			defer buffer.Free()

			data := buffer.Data()

			assert.Contains(cfg.TypedSpec().Value.GenerationError, expectedError, string(data))
			assert.Empty(cfg.TypedSpec().Value.ClusterMachineVersion)
		},
	)
}

func TestClusterMachineConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterMachineConfigSuite))
}
