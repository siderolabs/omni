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

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	conf "github.com/siderolabs/omni/internal/pkg/config"
)

var testSiderolinkCfg = conf.SiderolinkService{
	EventSinkPort: new(8091),
	LogServerPort: new(8092),
}

var testMachineAPIURL = "http://127.0.0.1:8090"

type ClusterMachineConfigSuite struct {
	OmniSuite
}

func (suite *ClusterMachineConfigSuite) registerControllers() {
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController(suite.kubernetesRuntime)))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(&imageFactoryClientMock{})))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(imageFactoryHost, nil, "ghcr.io/siderolabs/installer")))
	suite.Require().NoError(suite.runtime.RegisterQController(secrets.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(secrets.NewSecretRotationStatusController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosUpgradeStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineJoinConfigController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSiderolinkAPIConfigController(testMachineAPIURL, testSiderolinkCfg)))
	suite.Require().NoError(suite.runtime.RegisterQController(newMockJoinTokenUsageController[*siderolink.Link]()))
}

func (suite *ClusterMachineConfigSuite) TestReconcile() {
	suite.startRuntime()

	createJoinParams(suite.ctx, suite.state, suite.T())

	suite.registerControllers()

	clusterName := "talos-default-2"
	cluster, machines := suite.createClusterWithTalosVersion(clusterName, 1, 1, "1.10.0")

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewClusterMachineConfigPatches(machines[0].Metadata().ID()).Metadata(),
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
			*omni.NewClusterMachineConfig(m.Metadata().ID()).Metadata(),
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
					assertions.Equal(machineconfig.NetworkHostnameConfig().Hostname(), "patched-node")
				}
			},
		)
	}

	newImage := fmt.Sprintf("%s:v1.0.2", conf.Default().Registries.GetTalos())

	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewClusterMachineConfigPatches(machines[0].Metadata().ID()).Metadata(),
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
		*omni.NewClusterMachineConfig(machines[0].Metadata().ID()).Metadata(),
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
			suite.assertNoResource(*omni.NewClusterMachineConfig(m.Metadata().ID()).Metadata()),
		))
	}
}

func (suite *ClusterMachineConfigSuite) TestGeneratePreserveFeatures() {
	suite.startRuntime()

	createJoinParams(suite.ctx, suite.state, suite.T())

	suite.registerControllers()

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
		*omni.NewClusterMachineConfig(machines[0].Metadata().ID()).Metadata(),
		func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			buffer, bufferErr := spec.GetUncompressedData()
			suite.Require().NoError(bufferErr)

			defer buffer.Free()

			configData := buffer.Data()

			machineconfig, configErr := configloader.NewFromBytes(configData)
			suite.Require().NoError(configErr)

			assertions.True(machineconfig.Machine().Features().DiskQuotaSupportEnabled())
		},
	)

	suite.destroyCluster(cluster)

	for _, m := range machines {
		suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(*omni.NewClusterMachineConfig(m.Metadata().ID()).Metadata()),
		))
	}
}

func (suite *ClusterMachineConfigSuite) TestGenerationError() {
	suite.startRuntime()

	createJoinParams(suite.ctx, suite.state, suite.T())

	suite.registerControllers()

	clusterName := "test-generation-error"

	_, machines := suite.createCluster(clusterName, 1, 0)
	suite.Require().Greater(len(machines), 0)

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewClusterMachineConfigPatches(machines[0].Metadata().ID()).Metadata(),
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
		*omni.NewClusterMachineConfig(machines[0].Metadata().ID()).Metadata(),
		func(cfg *omni.ClusterMachineConfig, assert *assert.Assertions) {
			expectedError := "yaml: construct errors"

			buffer, bufferErr := cfg.TypedSpec().Value.GetUncompressedData()
			suite.Require().NoError(bufferErr)

			defer buffer.Free()

			data := buffer.Data()

			assert.Contains(cfg.TypedSpec().Value.GenerationError, expectedError, string(data))
		},
	)
}

func (suite *ClusterMachineConfigSuite) TestConfigEncodingStability() {
	suite.startRuntime()

	createJoinParams(suite.ctx, suite.state, suite.T())

	suite.registerControllers()

	maxTalosVersion, err := semver.ParseTolerant(version.Tag)
	suite.Require().NoError(err)

	initialTalosMinorVersion := 2
	maxTalosMinorVersion := int(maxTalosVersion.Minor)

	var talosVersions []string

	for i := initialTalosMinorVersion; i <= maxTalosMinorVersion; i++ {
		talosVersions = append(talosVersions, fmt.Sprintf("1.%d.1", i)) // use .1 as the patch version instead of .0 so that the goconst linter does not complain
	}

	for i, initialVersion := range talosVersions {
		suite.Run("initial-"+initialVersion, func() {
			suite.testConfigEncodingStabilityFrom(talosVersions[i:])
		})
	}
}

func (suite *ClusterMachineConfigSuite) TestGenerateWithoutComments() {
	suite.startRuntime()

	createJoinParams(suite.ctx, suite.state, suite.T())

	suite.registerControllers()

	clusterName := "talos-default"
	cluster, machines := suite.createClusterWithTalosVersion(clusterName, 1, 1, "1.10.0")

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfigStatus(machines[0].Metadata().ID()).Metadata(),
		func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
			assertions.True(res.TypedSpec().Value.WithoutComments)
		},
	)

	cmc, err := safe.StateUpdateWithConflicts(
		suite.ctx,
		suite.state,
		*omni.NewClusterMachineConfig(machines[0].Metadata().ID()).Metadata(),
		func(res *omni.ClusterMachineConfig) error {
			res.TypedSpec().Value.WithoutComments = false

			return nil
		},
		state.WithUpdateOwner(omnictrl.ClusterMachineConfigControllerName),
	)
	suite.Require().NoError(err)

	_, err = safe.StateUpdateWithConflicts(
		suite.ctx,
		suite.state,
		cluster.Metadata(),
		func(res *omni.Cluster) error {
			res.Metadata().Labels().Set("test-label", "test-value")

			return nil
		},
	)
	suite.Require().NoError(err)

	inputVersionOld, _ := cmc.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
	dataOld, err := cmc.TypedSpec().Value.GetUncompressedData()
	suite.Require().NoError(err)

	defer dataOld.Free()

	oldConf, err := configloader.NewFromBytes(dataOld.Data())
	suite.Require().NoError(err)

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfig(machines[0].Metadata().ID()).Metadata(),
		func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
			assertions.NotEqual(res.Metadata().Version(), cmc.Metadata().Version())
			inputVersionNew, _ := res.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
			assertions.NotEqual(inputVersionOld, inputVersionNew)

			data, dataErr := res.TypedSpec().Value.GetUncompressedData()
			suite.Require().NoError(dataErr)

			defer data.Free()

			newConf, newConfErr := configloader.NewFromBytes(data.Data())
			suite.Require().NoError(newConfErr)

			assertions.Equal(oldConf, newConf)
			assertions.False(res.TypedSpec().Value.WithoutComments)
		},
	)

	cmcNew, err := safe.ReaderGetByID[*omni.ClusterMachineConfig](suite.ctx, suite.state, machines[0].Metadata().ID())
	suite.Require().NoError(err)

	inputVersionOld, _ = cmcNew.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
	dataOld, err = cmcNew.TypedSpec().Value.GetUncompressedData()
	suite.Require().NoError(err)

	defer dataOld.Free()

	oldConf, err = configloader.NewFromBytes(dataOld.Data())
	suite.Require().NoError(err)

	newImage := fmt.Sprintf("%s:v1.10.1", conf.Default().Registries.GetTalos())

	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewClusterMachineConfigPatches(machines[0].Metadata().ID()).Metadata(),
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
		*omni.NewClusterMachineConfig(machines[0].Metadata().ID()).Metadata(),
		func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
			inputVersionNew, _ := res.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
			assertions.NotEqual(inputVersionOld, inputVersionNew)

			data, dataErr := res.TypedSpec().Value.GetUncompressedData()
			suite.Require().NoError(dataErr)

			defer data.Free()

			newConf, newConfErr := configloader.NewFromBytes(data.Data())
			suite.Require().NoError(newConfErr)

			assertions.NotEqual(oldConf, newConf)
			assertions.True(res.TypedSpec().Value.WithoutComments)
		},
	)

	suite.destroyCluster(cluster)

	for _, m := range machines {
		suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(*omni.NewClusterMachineConfig(m.Metadata().ID()).Metadata()),
		))
	}
}

func (suite *ClusterMachineConfigSuite) testConfigEncodingStabilityFrom(talosVersions []string) {
	initialVersion := talosVersions[0]
	upgradeVersions := talosVersions[1:]

	clusterName := "test-config-encoding-stability-from-" + initialVersion
	cluster, machines := suite.createClusterWithTalosVersion(clusterName, 1, 0, initialVersion)

	var (
		previousTalosVersion = initialVersion
		previousConfig       config.Provider
		err                  error
	)

	assertResource( // assert the initialConfig and initialize the previousConfig with it
		&suite.OmniSuite,
		*omni.NewClusterMachineConfig(machines[0].Metadata().ID()).Metadata(),
		func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
			buffer, bufferErr := res.TypedSpec().Value.GetUncompressedData()
			suite.Require().NoError(bufferErr)

			defer buffer.Free()

			configData := buffer.Data()

			previousConfig, err = configloader.NewFromBytes(configData)
			suite.Require().NoError(err)

			assertions.Contains(previousConfig.Machine().Install().Image(), initialVersion)
		},
	)

	for _, upgradeVersion := range upgradeVersions { // simulate upgrades and assert the config stability
		suite.Run(fmt.Sprintf("from-%s-to-%s", previousTalosVersion, upgradeVersion), func() {
			currentConfig := suite.testConfigEncodingStabilityTo(previousTalosVersion, upgradeVersion, cluster.Metadata().ID(), machines[0].Metadata().ID(), previousConfig)

			previousConfig = currentConfig
			previousTalosVersion = upgradeVersion
		})
	}

	// assert that no unexpected features were enabled on the final config - we test the regressions in https://github.com/siderolabs/omni/issues/1095#issue-2993591967
	finalConfig := previousConfig

	// initialize the default feature values for assertions
	manifestDirectoryDisabled := true
	legacyMirrorRemoved := true
	diskQuotaSupportEnabled := true
	kubePrismEnabled := true
	hostDNSEnabled := true
	hostDNSForwardKubeDNSToHost := true
	nodeHasLabelsSet := true
	grubUseUkiCmdlineSet := true

	// invert the features which were not available/default at the time of the initial version
	switch initialVersion {
	case "1.2.1":
		manifestDirectoryDisabled = false
		legacyMirrorRemoved = false

		fallthrough
	case "1.3.1":
		fallthrough
	case "1.4.1":
		diskQuotaSupportEnabled = false
		kubePrismEnabled = false // kubeprism gets special treatment - even though it is enabled by default only for >=1.6, we enable it explicitly for >=1.5 in Omni

		fallthrough
	case "1.5.1":
		fallthrough
	case "1.6.1":
		hostDNSEnabled = false

		fallthrough
	case "1.7.1":
		hostDNSForwardKubeDNSToHost = false
		nodeHasLabelsSet = false

		fallthrough
	case "1.8.1":
		fallthrough
	case "1.9.1":
		fallthrough
	case "1.10.1":
		fallthrough
	case "1.11.1":
		grubUseUkiCmdlineSet = false

		fallthrough
	case "1.12.1":
	case "1.13.1":
	default:
		suite.T().Fatalf("untested initial version: %s", initialVersion)
	}

	suite.Equal(manifestDirectoryDisabled, finalConfig.Machine().Kubelet().DisableManifestsDirectory(), "disableManifestsDirectory value has changed unexpectedly")
	suite.Equal(legacyMirrorRemoved, len(finalConfig.RegistryMirrorConfigs()) == 0, "legacy registry mirror value has changed unexpectedly")
	suite.Equal(diskQuotaSupportEnabled, finalConfig.Machine().Features().DiskQuotaSupportEnabled(), "diskQuotaSupport feature value has changed unexpectedly")
	suite.Equal(hostDNSEnabled, finalConfig.Machine().Features().HostDNS().Enabled(), "hostDNS feature value has changed unexpectedly")
	suite.Equal(hostDNSForwardKubeDNSToHost, finalConfig.Machine().Features().HostDNS().ForwardKubeDNSToHost(), "hostDNS.forwardKubeDNSToHost value has changed unexpectedly")
	suite.Equal(kubePrismEnabled, finalConfig.Machine().Features().KubePrism().Enabled(), "kubePrism feature value has changed unexpectedly")
	suite.Equal(nodeHasLabelsSet, len(finalConfig.Machine().NodeLabels()) > 0, "node labels value has changed unexpectedly")
	suite.Equal(grubUseUkiCmdlineSet, finalConfig.Machine().Install().GrubUseUKICmdline(), "grubUseUkiCmdline value has changed unexpectedly")
}

func (suite *ClusterMachineConfigSuite) testConfigEncodingStabilityTo(previousTalosVersion, upgradeTalosVersion string,
	clusterID, machineID resource.ID, previousConfig config.Provider,
) (currentConfig config.Provider) {
	suite.T().Logf("upgrade %s->%s", previousTalosVersion, upgradeTalosVersion)
	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, omni.NewCluster(clusterID).Metadata(), func(res *omni.Cluster) error {
		res.TypedSpec().Value.TalosVersion = upgradeTalosVersion

		return nil
	})
	suite.Require().NoError(err)

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineConfig(machineID).Metadata(),
		func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			buffer, bufferErr := spec.GetUncompressedData()
			suite.Require().NoError(bufferErr)

			defer buffer.Free()

			configData := buffer.Data()

			currentConfig, err = configloader.NewFromBytes(configData)
			suite.Require().NoError(err)

			previousInstallImage := previousConfig.Machine().Install().Image()
			currentInstallImage := currentConfig.Machine().Install().Image()

			if !assertions.Containsf(currentInstallImage, upgradeTalosVersion, "the install image in the config is not updated yet to have the new version %q", upgradeTalosVersion) {
				return
			}

			suite.T().Logf("compare configs %s<>%s", previousTalosVersion, upgradeTalosVersion)

			suite.Require().Contains(previousInstallImage, previousTalosVersion) // make sure that we didn't overwrite the previous image, so we compare the correct two things
			suite.configsAreEqual(previousConfig, currentConfig)
		},
	)

	return currentConfig
}

func (suite *ClusterMachineConfigSuite) configsAreEqual(first, second config.Provider) {
	// clone both configs to:
	// - avoid modifying the original ones
	// - be able to overwrite the install images, as the original ones are read-only
	first = first.Clone()
	second = second.Clone()

	first.RawV1Alpha1().MachineConfig.MachineInstall.InstallImage = ""
	second.RawV1Alpha1().MachineConfig.MachineInstall.InstallImage = ""

	firstData, err := first.EncodeString(encoder.WithComments(encoder.CommentsDisabled))
	suite.Require().NoError(err)

	secondData, err := second.EncodeString(encoder.WithComments(encoder.CommentsDisabled))
	suite.Require().NoError(err)

	suite.Equal(firstData, secondData)
}

func TestClusterMachineConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterMachineConfigSuite))
}

func createJoinParams(ctx context.Context, state state.State, t *testing.T) {
	params := siderolink.NewDefaultJoinToken()
	params.TypedSpec().Value.TokenId = "testtoken"

	require.NoError(t, state.Create(ctx, params))
	require.NoError(t, state.Create(ctx, siderolink.NewConfig()))
}
