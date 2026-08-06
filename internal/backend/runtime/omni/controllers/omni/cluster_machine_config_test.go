// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"maps"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/installdisk"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
	conf "github.com/siderolabs/omni/internal/pkg/config"
)

var testSiderolinkCfg = conf.SiderolinkService{
	EventSinkPort: new(8091),
	LogServerPort: new(8092),
}

var testMachineAPIURL = "http://127.0.0.1:8090"

// registerClusterMachineConfigControllers registers the controller under test plus the siderolink
// join config chain. Talos 1.5+ carries the join config as a document of the generated machine
// config, so the real controllers have to produce it instead of a fixture.
func registerClusterMachineConfigControllers(t *testing.T) testutils.TestFunc {
	return func(_ context.Context, tc testutils.TestContext) {
		require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController(nil, "ghcr.io/siderolabs/installer", conf.Registries{})))
		require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewMachineJoinConfigController()))
		require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewSiderolinkAPIConfigController(testMachineAPIURL, testSiderolinkCfg)))
		require.NoError(t, tc.Runtime.RegisterQController(newMockJoinTokenUsageController[*siderolink.Link]()))
	}
}

// createConfigTestCluster creates everything the ClusterMachineConfigController reads for a cluster
// of one control plane and the requested number of workers. The control plane comes first in the
// returned machines.
func createConfigTestCluster(ctx context.Context, t *testing.T, st state.State, clusterName string, workers int, talosVersion string) (
	*omni.Cluster, []*omni.ClusterMachine,
) {
	const controlPlanes = 1

	createJoinParams(ctx, st, t)

	cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName), options.WithTalosVersion(talosVersion))

	rmock.Mock[*omni.ClusterSecrets](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.ClusterConfigVersion](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.LoadBalancerConfig](ctx, t, st, options.SameID(cluster))

	cpMachineSet := rmock.Mock[*omni.MachineSet](
		ctx, t, st,
		options.WithID(omni.ControlPlanesResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelControlPlaneRole),
	)

	workersMachineSet := rmock.Mock[*omni.MachineSet](
		ctx, t, st,
		options.WithID(omni.WorkersResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelWorkerRole),
	)

	ids := make([]string, 0, controlPlanes+workers)

	for i := range controlPlanes + workers {
		ids = append(ids, fmt.Sprintf("%s-node-%d", clusterName, i))
	}

	rmock.MockList[*omni.MachineSetNode](
		ctx, t, st,
		options.IDs(ids[:controlPlanes]),
		options.ItemOptions(
			options.LabelCluster(cluster),
			options.LabelMachineSet(cpMachineSet),
			options.EmptyLabel(omni.LabelControlPlaneRole),
		),
	)

	if workers > 0 {
		rmock.MockList[*omni.MachineSetNode](
			ctx, t, st,
			options.IDs(ids[controlPlanes:]),
			options.ItemOptions(
				options.LabelCluster(cluster),
				options.LabelMachineSet(workersMachineSet),
				options.EmptyLabel(omni.LabelWorkerRole),
			),
		)
	}

	machines := rmock.MockList[*omni.ClusterMachine](
		ctx, t, st,
		options.IDs(ids),
		options.ItemOptions(
			options.Modify(func(res *omni.ClusterMachine) error {
				res.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

				return nil
			}),
		),
	)

	for _, m := range machines {
		rmock.Mock[*omni.ClusterMachineSecrets](ctx, t, st, options.SameID(m))
		rmock.Mock[*omni.Machine](ctx, t, st, options.SameID(m))
		rmock.Mock[*omni.ClusterMachineConfigPatches](ctx, t, st, options.SameID(m))
		rmock.Mock[*omni.MachineInstallDiskStatus](ctx, t, st, options.SameID(m))

		rmock.Mock[*siderolink.Link](
			ctx, t, st, options.SameID(m),
			options.Modify(func(res *siderolink.Link) error {
				res.TypedSpec().Value.Connected = true

				return nil
			}),
		)

		mockGenOptions(ctx, t, st, m.Metadata().ID(), talosVersion)
	}

	return cluster, machines
}

// mockGenOptions points the machine at the given Talos version. That is all an upgrade amounts to
// for the config generation.
func mockGenOptions(ctx context.Context, t *testing.T, st state.State, machineID, talosVersion string) {
	rmock.Mock[*omni.MachineConfigGenOptions](
		ctx, t, st, options.WithID(machineID),
		options.Modify(func(res *omni.MachineConfigGenOptions) error {
			res.TypedSpec().Value.InstallImage.TalosVersion = talosVersion
			res.TypedSpec().Value.InstallImage.ImageFactoryHost = imageFactoryHost

			return nil
		}),
	)
}

func appendPatch(ctx context.Context, t *testing.T, st state.State, machineID, patch string) {
	_, err := safe.StateUpdateWithConflicts(
		ctx, st, omni.NewClusterMachineConfigPatches(machineID).Metadata(),
		func(config *omni.ClusterMachineConfigPatches) error {
			patches, err := config.TypedSpec().Value.GetUncompressedPatches()
			if err != nil {
				return err
			}

			return config.TypedSpec().Value.SetUncompressedPatches(append(patches, patch))
		},
	)
	require.NoError(t, err)
}

// awaitGeneratedConfig waits for the controller to generate the config of the machine and returns
// it. Assertions on the contents then run once instead of on every poll.
func awaitGeneratedConfig(ctx context.Context, t *testing.T, st state.State, machineID string) *omni.ClusterMachineConfig {
	rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
		assertions.Empty(res.TypedSpec().Value.GenerationError)
	})

	res, err := safe.StateGetByID[*omni.ClusterMachineConfig](ctx, st, machineID)
	require.NoError(t, err)

	return res
}

// machineConfigOf loads the generated Talos config of the machine.
func machineConfigOf(t *testing.T, res *omni.ClusterMachineConfig) config.Provider {
	buffer, err := res.TypedSpec().Value.GetUncompressedData()
	require.NoError(t, err)

	defer buffer.Free()

	cfg, err := configloader.NewFromBytes(buffer.Data())
	require.NoError(t, err)

	return cfg
}

func TestClusterMachineConfigReconcile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, registerClusterMachineConfigControllers(t),
		func(ctx context.Context, tc testutils.TestContext) {
			const talosVersion = "1.10.0"

			_, machines := createConfigTestCluster(ctx, t, tc.State, "talos-default-2", 1, talosVersion)

			appendPatch(ctx, t, tc.State, machines[0].Metadata().ID(), `machine:
  network:
    hostname: patched-node`)

			for i, m := range machines {
				rtestutils.AssertResource(ctx, t, tc.State, m.Metadata().ID(), func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
					cfg := machineConfigOf(t, res)

					expectedType := machine.TypeWorker
					if _, ok := m.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
						expectedType = machine.TypeControlPlane
					}

					assertions.Equal(expectedType, cfg.Machine().Type())
					assertions.Equal(testInstallDisk, cfg.Machine().Install().Disk())
					assertions.Equal(testInstallDisk, res.TypedSpec().Value.InstallDisk)
					assertions.Equal(
						fmt.Sprintf("%s/%s-installer/%s:v%s", imageFactoryHost, talosconstants.PlatformMetal, defaultSchematic, talosVersion),
						cfg.Machine().Install().Image(),
					)

					if i == 0 {
						assertions.Equal("patched-node", cfg.NetworkHostnameConfig().Hostname())
					}
				})
			}

			newImage := fmt.Sprintf("%s:v1.0.2", conf.Default().Registries.GetTalos())

			appendPatch(ctx, t, tc.State, machines[0].Metadata().ID(), `machine:
  install:
    image: `+newImage)

			rtestutils.AssertResource(ctx, t, tc.State, machines[0].Metadata().ID(), func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
				assertions.Equal(newImage, machineConfigOf(t, res).Machine().Install().Image())
			})

			for _, m := range machines {
				rmock.Destroy[*omni.ClusterMachine](ctx, t, tc.State, []string{m.Metadata().ID()})

				rtestutils.AssertNoResource[*omni.ClusterMachineConfig](ctx, t, tc.State, m.Metadata().ID())
			}
		},
	)
}

func TestClusterMachineConfigGeneratePreserveFeatures(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, registerClusterMachineConfigControllers(t),
		func(ctx context.Context, tc testutils.TestContext) {
			_, machines := createConfigTestCluster(ctx, t, tc.State, "talos-default-old", 1, "1.2.0")

			_, err := safe.StateUpdateWithConflicts(ctx, tc.State, machines[0].Metadata(), func(res *omni.ClusterMachine) error {
				res.Metadata().Annotations().Set(omni.PreserveApidCheckExtKeyUsage, "")
				res.Metadata().Annotations().Set(omni.PreserveDiskQuotaSupport, "")

				return nil
			}, state.WithUpdateOwner(rmock.GetOwner[*omni.ClusterMachine]()))
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, tc.State, machines[0].Metadata().ID(), func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
				assertions.True(machineConfigOf(t, res).Machine().Features().DiskQuotaSupportEnabled())
			})
		},
	)
}

func TestClusterMachineConfigGenerationError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, registerClusterMachineConfigControllers(t),
		func(ctx context.Context, tc testutils.TestContext) {
			_, machines := createConfigTestCluster(ctx, t, tc.State, "test-generation-error", 0, TalosVersion)

			appendPatch(ctx, t, tc.State, machines[0].Metadata().ID(), `machine:
  network:
    interfaces:
      - interface: eth42
        bridge: invalidValueType`)

			rtestutils.AssertResource(ctx, t, tc.State, machines[0].Metadata().ID(), func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
				assertions.Contains(res.TypedSpec().Value.GenerationError, "yaml: construct errors")
			})
		},
	)
}

func TestClusterMachineConfigGenerateWithoutComments(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, registerClusterMachineConfigControllers(t),
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			cluster, machines := createConfigTestCluster(ctx, t, st, "talos-default", 1, "1.10.0")
			machineID := machines[0].Metadata().ID()

			rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
				assertions.True(res.TypedSpec().Value.WithoutComments)
			})

			cmc, err := safe.StateUpdateWithConflicts(
				ctx, st, omni.NewClusterMachineConfig(machineID).Metadata(),
				func(res *omni.ClusterMachineConfig) error {
					res.TypedSpec().Value.WithoutComments = false

					return nil
				},
				state.WithUpdateOwner(omnictrl.ClusterMachineConfigControllerName),
			)
			require.NoError(t, err)

			_, err = safe.StateUpdateWithConflicts(ctx, st, cluster.Metadata(), func(res *omni.Cluster) error {
				res.Metadata().Labels().Set("test-label", "test-value")

				return nil
			})
			require.NoError(t, err)

			inputVersionOld, _ := cmc.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
			oldConf := machineConfigOf(t, cmc)

			// re-generating over a config whose comments were stripped must not change the config itself
			rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
				assertions.NotEqual(res.Metadata().Version(), cmc.Metadata().Version())

				inputVersionNew, _ := res.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
				assertions.NotEqual(inputVersionOld, inputVersionNew)

				assertions.Equal(oldConf, machineConfigOf(t, res))
				assertions.False(res.TypedSpec().Value.WithoutComments)
			})

			cmcNew, err := safe.ReaderGetByID[*omni.ClusterMachineConfig](ctx, st, machineID)
			require.NoError(t, err)

			inputVersionOld, _ = cmcNew.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
			oldConf = machineConfigOf(t, cmcNew)

			newImage := fmt.Sprintf("%s:v1.10.1", conf.Default().Registries.GetTalos())

			appendPatch(ctx, t, st, machineID, `machine:
  install:
    image: `+newImage)

			rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
				inputVersionNew, _ := res.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
				assertions.NotEqual(inputVersionOld, inputVersionNew)

				assertions.NotEqual(oldConf, machineConfigOf(t, res))
				assertions.True(res.TypedSpec().Value.WithoutComments)
			})
		},
	)
}

// TestClusterMachineConfigGrubUseUKICmdline covers the flag that gates kernel args updates on
// GRUB-booted machines. It comes from .machine.install, which Talos 1.14+ no longer generates.
func TestClusterMachineConfigGrubUseUKICmdline(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		talosVersion string
		expected     bool
	}{
		{talosVersion: "1.11.1", expected: false}, // predates grubUseUKICmdline
		{talosVersion: "1.13.1", expected: true},  // carries grubUseUKICmdline in .machine.install
		{talosVersion: "1.14.1", expected: true},  // no .machine.install, the LifecycleService install always uses the UKI cmdline
	} {
		t.Run(test.talosVersion, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(
				ctx, t, testutils.TestOptions{}, registerClusterMachineConfigControllers(t),
				func(ctx context.Context, tc testutils.TestContext) {
					_, machines := createConfigTestCluster(ctx, t, tc.State, "grub-uki-cmdline-"+test.talosVersion, 0, test.talosVersion)

					clusterMachineConfig := awaitGeneratedConfig(ctx, t, tc.State, machines[0].Metadata().ID())

					assert.Equal(t, test.expected, clusterMachineConfig.TypedSpec().Value.GrubUseUkiCmdline)
				},
			)
		})
	}
}

func TestClusterMachineConfigEncodingStability(t *testing.T) {
	t.Parallel()

	maxTalosVersion, err := semver.ParseTolerant(version.Tag)
	require.NoError(t, err)

	const initialTalosMinorVersion = 2

	var talosVersions []string

	for i := initialTalosMinorVersion; i <= int(maxTalosVersion.Minor); i++ {
		talosVersions = append(talosVersions, fmt.Sprintf("1.%d.1", i)) // use .1 as the patch version instead of .0 so that the goconst linter does not complain
	}

	for i, initialVersion := range talosVersions {
		t.Run("initial-"+initialVersion, func(t *testing.T) {
			t.Parallel()

			testConfigEncodingStabilityFrom(t, talosVersions[i:])
		})
	}
}

// testConfigEncodingStabilityFrom creates a cluster on the first of the given versions and walks it
// through the later ones. Nothing but the Talos version may move in the config.
func testConfigEncodingStabilityFrom(t *testing.T, talosVersions []string) {
	initialVersion := talosVersions[0]

	versionContract, err := config.ParseContractFromVersion(initialVersion)
	require.NoError(t, err)

	// Talos 1.14 moved installation out of the machine config. Omni installs over the
	// LifecycleService API, so a cluster created on 1.14+ gets neither a .machine.install section
	// nor the UnattendedInstallConfig document, which exists for standalone Talos.
	omniOwnsInstall := versionContract.UnattendedInstallConfig()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, registerClusterMachineConfigControllers(t),
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			_, machines := createConfigTestCluster(ctx, t, st, "encoding-stability-from-"+initialVersion, 0, initialVersion)
			machineID := machines[0].Metadata().ID()

			previousConfig := machineConfigOf(t, awaitGeneratedConfig(ctx, t, st, machineID))

			if omniOwnsInstall {
				assertOmniOwnsInstall(t, previousConfig)
			} else {
				assert.Contains(t, previousConfig.Machine().Install().Image(), initialVersion)
			}

			previousTalosVersion := initialVersion

			for _, upgradeVersion := range talosVersions[1:] { // simulate upgrades and assert the config stability
				t.Run(fmt.Sprintf("from-%s-to-%s", previousTalosVersion, upgradeVersion), func(t *testing.T) {
					previousConfig = assertUpgradeKeepsConfigStable(ctx, t, st, machineID, previousTalosVersion, upgradeVersion, previousConfig, omniOwnsInstall)
					previousTalosVersion = upgradeVersion
				})
			}

			// assert that no unexpected features were enabled on the final config - we test the regressions in https://github.com/siderolabs/omni/issues/1095#issue-2993591967
			assertFinalConfigFeatures(t, previousConfig, initialVersion, omniOwnsInstall)
		},
	)
}

// assertUpgradeKeepsConfigStable bumps the machine to the given Talos version and asserts that the
// regenerated config differs from the previous one only in the install image.
func assertUpgradeKeepsConfigStable(ctx context.Context, t *testing.T, st state.State, machineID, previousTalosVersion, upgradeTalosVersion string,
	previousConfig config.Provider, omniOwnsInstall bool,
) config.Provider {
	t.Logf("upgrade %s->%s", previousTalosVersion, upgradeTalosVersion)

	previousResource, err := safe.StateGetByID[*omni.ClusterMachineConfig](ctx, st, machineID)
	require.NoError(t, err)

	previousInputVersion, _ := previousResource.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)

	mockGenOptions(ctx, t, st, machineID, upgradeTalosVersion)

	var currentConfig config.Provider

	rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
		currentConfig = machineConfigOf(t, res)

		// on Talos 1.14+ the install image is no longer part of the config, so the input versions
		// annotation is the only sign that the controller picked up the new version
		if omniOwnsInstall {
			inputVersion, _ := res.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)

			assertions.NotEqual(previousInputVersion, inputVersion, "the new install image is not picked up yet")

			return
		}

		assertions.Containsf(currentConfig.Machine().Install().Image(), upgradeTalosVersion,
			"the install image in the config is not updated yet to have the new version %q", upgradeTalosVersion)
	})

	t.Logf("compare configs %s<>%s", previousTalosVersion, upgradeTalosVersion)

	if omniOwnsInstall {
		assertOmniOwnsInstall(t, currentConfig)

		// nothing else in the config depends on the Talos version, so it has to stay byte-identical
		assert.Equal(t, previousConfig, currentConfig)

		return currentConfig
	}

	// make sure that we didn't overwrite the previous image, so we compare the correct two things
	require.Contains(t, previousConfig.Machine().Install().Image(), previousTalosVersion)

	assertConfigsAreEqual(t, previousConfig, currentConfig)

	return currentConfig
}

// assertOmniOwnsInstall asserts that the config carries no installation instructions. Omni installs
// over the LifecycleService API, and UnattendedInstallConfig is meant for standalone Talos.
func assertOmniOwnsInstall(t *testing.T, cfg config.Provider) {
	assert.Nil(t, cfg.RawV1Alpha1().MachineConfig.MachineInstall, ".machine.install must not be generated for Talos 1.14+") //nolint:staticcheck
	assert.Nil(t, cfg.UnattendedInstallConfig(), "UnattendedInstallConfig must not be generated, Omni performs the install itself")
}

func assertFinalConfigFeatures(t *testing.T, finalConfig config.Provider, initialVersion string, omniOwnsInstall bool) {
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
	case "1.14.1":
	default:
		t.Fatalf("untested initial version: %s", initialVersion)
	}

	assert.Equal(t, manifestDirectoryDisabled, finalConfig.K8sKubeletConfig().DisableManifestsDirectory(), "disableManifestsDirectory value has changed unexpectedly")
	assert.Equal(t, legacyMirrorRemoved, len(finalConfig.RegistryMirrorConfigs()) == 0, "legacy registry mirror value has changed unexpectedly")
	assert.Equal(t, diskQuotaSupportEnabled, finalConfig.Machine().Features().DiskQuotaSupportEnabled(), "diskQuotaSupport feature value has changed unexpectedly")

	hostDNSConfig := finalConfig.NetworkHostDNSConfig()
	assert.Equal(t, hostDNSEnabled, hostDNSConfig != nil && hostDNSConfig.HostDNSEnabled(), "hostDNS feature value has changed unexpectedly")
	assert.Equal(t, hostDNSForwardKubeDNSToHost, hostDNSConfig != nil && hostDNSConfig.ForwardKubeDNSToHost(), "hostDNS.forwardKubeDNSToHost value has changed unexpectedly")

	kubePrismConfig := finalConfig.K8sKubePrismConfig()
	assert.Equal(t, kubePrismEnabled, kubePrismConfig != nil, "kubePrism feature value has changed unexpectedly")

	// the control-plane role label is always derived for control-plane nodes (v1alpha1 shim and the
	// multidoc KubeNodeConfig alike), so exclude it to check only the labels Omni actually sets in the config.
	nodeLabels := maps.Clone(finalConfig.K8sNodeConfig().Labels())
	delete(nodeLabels, talosconstants.LabelNodeRoleControlPlane)
	assert.Equal(t, nodeHasLabelsSet, len(nodeLabels) > 0, "node labels value has changed unexpectedly")

	if omniOwnsInstall {
		assertOmniOwnsInstall(t, finalConfig)

		return
	}

	assert.Equal(t, grubUseUkiCmdlineSet, finalConfig.Machine().Install().GrubUseUKICmdline(), "grubUseUkiCmdline value has changed unexpectedly")
}

func assertConfigsAreEqual(t *testing.T, first, second config.Provider) {
	// clone both configs to:
	// - avoid modifying the original ones
	// - be able to overwrite the install images, as the original ones are read-only
	first = first.Clone()
	second = second.Clone()

	first.RawV1Alpha1().MachineConfig.MachineInstall.InstallImage = ""  //nolint:staticcheck
	second.RawV1Alpha1().MachineConfig.MachineInstall.InstallImage = "" //nolint:staticcheck

	firstData, err := first.EncodeString(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)

	secondData, err := second.EncodeString(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)

	assert.Equal(t, firstData, secondData)
}

func createJoinParams(ctx context.Context, st state.State, t *testing.T) {
	params := siderolink.NewDefaultJoinToken()
	params.TypedSpec().Value.TokenId = "testtoken"

	require.NoError(t, st.Create(ctx, params))
	require.NoError(t, st.Create(ctx, siderolink.NewConfig()))
}

// TestClusterMachineConfigInstallDiskSync verifies the install disk sync check: the config render
// is held while the install disk resolution does not reflect the current selection (the selection
// hash on the status does not match the selection), and the very first rendered config version
// already carries the disk of the caught-up resolution. A config rendered from the stale
// resolution would show up as an extra version carrying the stale disk.
func TestClusterMachineConfigInstallDiskSync(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, registerClusterMachineConfigControllers(t),
		func(ctx context.Context, tc testutils.TestContext) {
			const (
				selector = `disk.dev_path == "/dev/vdz"`

				// the machine IDs of createConfigTestCluster are deterministic
				machineID = "talos-default-disk-sync-node-0"
			)

			// The user's selection exists BEFORE anything of the cluster: the fixture then mocks
			// the resolution with an empty selection hash, so no moment exists at which the
			// renderer could produce a config from the selection-less resolution without the
			// sync check holding it.
			installDiskConfig := omni.NewMachineInstallDiskConfig(machineID)
			installDiskConfig.TypedSpec().Value.DiskSelector = selector
			require.NoError(t, tc.State.Create(ctx, installDiskConfig))

			_, machines := createConfigTestCluster(ctx, t, tc.State, "talos-default-disk-sync", 0, "1.10.0")

			require.Equal(t, machineID, machines[0].Metadata().ID())

			// the resolution catches up: re-mock the status with the matching selection hash and
			// the disk the selector picks
			rmock.Mock[*omni.MachineInstallDiskStatus](ctx, t, tc.State, options.WithID(machineID), options.Modify(func(res *omni.MachineInstallDiskStatus) error {
				res.TypedSpec().Value.Disk = "/dev/vdz"
				res.TypedSpec().Value.SelectionHash = installdisk.SelectionHash(installDiskConfig)

				return nil
			}))

			rtestutils.AssertResource(ctx, t, tc.State, machineID, func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
				assertions.Equal("/dev/vdz", machineConfigOf(t, res).Machine().Install().Disk())
				assertions.Equal("/dev/vdz", res.TypedSpec().Value.InstallDisk)

				// the held renders must not have produced a config from the stale resolution: the
				// caught-up disk must be there from the very first version
				assertions.Equal("1", res.Metadata().Version().String())
			})
		},
	)
}

// TestClusterMachineConfigInstallDiskReassert verifies that a config patch cannot make the
// rendered bytes carry a different install disk than the one recorded on the config: new patches
// carrying .machine.install.disk are rejected at the API, but patches written before that
// restriction still flow through the patch collection, and the disk config generation produced
// must win over them in both directions. A patch deleting the whole install section is the
// accepted exception: the section stays deleted, and such a config cannot install anywhere.
func TestClusterMachineConfigInstallDiskReassert(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name         string
		cluster      string
		talosVersion string
		patch        string
		expectedDisk string
	}{
		{
			// the patch overrides the generated disk: the verified disk must win
			name:         "override",
			cluster:      "talos-default-disk-reassert-override",
			talosVersion: "1.10.0",
			patch: `machine:
  install:
    disk: /dev/not-the-verified-disk
  network:
    hostname: reassert-marker`,
			expectedDisk: testInstallDisk,
		},
		{
			// the patch deletes the install section: it stays deleted, nothing to disagree with
			name:         "section deleted",
			cluster:      "talos-default-disk-reassert-deleted",
			talosVersion: "1.10.0",
			patch: `machine:
  install:
    $patch: delete
  network:
    hostname: reassert-marker`,
			expectedDisk: "",
		},
		{
			// a 1.14+ contract renders no install section: a patch must not smuggle a disk in
			name:         "injected into a section-less contract",
			cluster:      "talos-default-disk-reassert-injected",
			talosVersion: "1.14.1",
			patch: `machine:
  install:
    disk: /dev/not-the-verified-disk
  network:
    hostname: reassert-marker`,
			expectedDisk: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(
				ctx, t, testutils.TestOptions{}, registerClusterMachineConfigControllers(t),
				func(ctx context.Context, tc testutils.TestContext) {
					_, machines := createConfigTestCluster(ctx, t, tc.State, tt.cluster, 0, tt.talosVersion)

					machineID := machines[0].Metadata().ID()

					appendPatch(ctx, t, tc.State, machineID, tt.patch)

					rtestutils.AssertResource(ctx, t, tc.State, machineID, func(res *omni.ClusterMachineConfig, assertions *assert.Assertions) {
						assertions.Empty(res.TypedSpec().Value.GenerationError)

						cfg := machineConfigOf(t, res)

						// the hostname proves this render consumed the patch
						assertions.Equal("reassert-marker", cfg.RawV1Alpha1().MachineConfig.MachineNetwork.NetworkHostname) //nolint:staticcheck
						assertions.Equal(tt.expectedDisk, cfg.Machine().Install().Disk())

						// the recorded disk always carries the verified value, whatever the bytes say
						assertions.Equal(testInstallDisk, res.TypedSpec().Value.InstallDisk)
					})
				},
			)
		})
	}
}
