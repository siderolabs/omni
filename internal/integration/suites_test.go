// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/integration/workloadproxy"
	"github.com/siderolabs/omni/internal/pkg/clientconfig"
)

type assertClusterReadyOptions struct {
	talosVersion      string
	kubernetesVersion string
}

type assertClusterReadyOption func(*assertClusterReadyOptions)

func withTalosVersion(version string) assertClusterReadyOption {
	return func(acro *assertClusterReadyOptions) {
		acro.talosVersion = version
	}
}

func withKubernetesVersion(version string) assertClusterReadyOption {
	return func(acro *assertClusterReadyOptions) {
		acro.kubernetesVersion = version
	}
}

func assertClusterAndAPIReady(t *testing.T, clusterName string, options *TestOptions, opts ...assertClusterReadyOption) {
	optionsStruct := assertClusterReadyOptions{
		talosVersion:      options.MachineOptions.TalosVersion,
		kubernetesVersion: options.MachineOptions.KubernetesVersion,
	}

	for _, o := range opts {
		o(&optionsStruct)
	}

	runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
		t.Context(),
		options.omniClient,
		clusterName,
		optionsStruct.talosVersion,
		optionsStruct.kubernetesVersion,
	))
}

func testCleanState(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		ctx := t.Context()

		t.Log(`
Bring the state of Omni to a clean state by removing all clusters, config patches, etc. which might have been left from previous runs.
Wait for all expected machines to join and be in maintenance mode.`)

		t.Run(
			"DestroyAllClusterRelatedResources",
			DestroyAllClusterRelatedResources(ctx, options.omniClient.Omni().State()),
		)

		// machine discovery, all machines should be in maintenance mode
		t.Run(
			"LinkCountShouldMatchExpectedMachines",
			AssertNumberOfLinks(ctx, options.omniClient.Omni().State(), expectedMachines),
		)

		t.Run(
			"LinksShouldBeConnected",
			AssertLinksConnected(ctx, options.omniClient.Omni().State()),
		)

		t.Run(
			"LinksShouldMatchMachines",
			AssertMachinesMatchLinks(ctx, options.omniClient.Omni().State()),
		)

		t.Run(
			"MachinesShouldHaveLogs",
			AssertMachinesHaveLogs(ctx, options.omniClient.Omni().State(), options.omniClient.Management()),
		)

		t.Run(
			"MachinesShouldBeReachableInMaintenanceMode",
			AssertTalosMaintenanceAPIAccessViaOmni(ctx, options.omniClient),
		)

		t.Run(
			"MachinesShouldBeInMaintenanceMode",
			AssertMachineStatus(ctx, options.omniClient.Omni().State(), true, "", map[string]string{
				omni.MachineStatusLabelConnected:       "",
				omni.MachineStatusLabelReportingEvents: "",
				omni.MachineStatusLabelAvailable:       "",
				// QEMU-specific labels which should always match, others are specific to the settings (number of cores, etc.)
				omni.MachineStatusLabelCPU:      "qemu",
				omni.MachineStatusLabelArch:     "amd64",
				omni.MachineStatusLabelPlatform: "metal",
			}, nil),
		)
	}
}

func testImageGeneration(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Parallel()

		t.Log(`
Generate various Talos images with Omni and try to download them.`)

		t.Run(
			"TalosImagesShouldBeDownloadableUsingCLI",
			AssertDownloadUsingCLI(t.Context(), options.omniClient, options.OmnictlPath, options.HTTPEndpoint),
		)

		t.Run(
			"TalosImagesShouldBeDownloadable",
			AssertSomeImagesAreDownloadable(t.Context(), options.omniClient, func(ctx context.Context, req *http.Request) error {
				return clientconfig.SignHTTPRequest(ctx, options.omniClient, req)
			}, options.HTTPEndpoint),
		)
	}
}

func testCLICommands(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Parallel()

		t.Log(`
Verify various omnictl commands.`)

		t.Run(
			"OmnictlUserCLIShouldWork",
			AssertUserCLI(t.Context(), options.omniClient, options.OmnictlPath, options.HTTPEndpoint),
		)
	}
}

func testKubernetesNodeAudit(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Parallel()

		clusterName := "integration-k8s-node-audit"

		options.claimMachines(t, 2)

		t.Log(`
Test the auditing of the Kubernetes nodes, i.e. when a node is gone from the Omni perspective but still exists on the Kubernetes cluster.`)

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       1,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		runTests(
			t,
			AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
				t.Context(),
				options.omniClient,
				clusterName,
				options.MachineOptions.TalosVersion,
				options.MachineOptions.KubernetesVersion,
			),
		)

		t.Run(
			"KubernetesNodeAuditShouldBePerformed",
			AssertKubernetesNodeAudit(
				t.Context(),
				clusterName,
				options,
			),
		)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testForcedMachineRemoval(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests different scenarios for forced Machine removal (vs. graceful removing from a cluster):

- force remove a Machine which is not allocated (not part of any cluster)
- force remove a worker Machine which is part of the cluster
- force remove a control plane Machine which is part of the cluster, and replace with a new Machine.

These tests simulate a hardware failure of a Machine which requires a forced removal from Omni.

In the tests, we wipe and reboot the VMs to bring them back as available for the next test.`)

		t.Parallel()

		options.claimMachines(t, 4)

		clusterName := "integration-forced-removal"

		assertClusterReady := func() {
			runTests(t, AssertBlockClusterShouldBeReady(
				t.Context(),
				options.omniClient,
				clusterName,
				options.MachineOptions.TalosVersion,
			))
		}

		t.Run(
			"UnallocatedMachinesShouldBeDestroyable",
			AssertUnallocatedMachineDestroyFlow(t.Context(), options.omniClient.Omni().State(), options.RestartAMachineFunc),
		)

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 3,
				Workers:       1,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterReady()

		t.Run(
			"WorkerNodesShouldBeForceRemovable",
			AssertForceRemoveWorkerNode(t.Context(), options.omniClient.Omni().State(), clusterName, options.FreezeAMachineFunc, options.WipeAMachineFunc),
		)

		assertClusterReady()

		t.Run(
			"ControlPlaneNodeShouldBeForceReplaceable",
			AssertControlPlaneForceReplaceMachine(
				t.Context(),
				options.omniClient.Omni().State(),
				clusterName,
				options.Options,
			),
		)

		assertClusterReady()

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testImmediateClusterDestruction(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Regression test: create a cluster and destroy it without waiting for the cluster to reach any state.`)

		t.Parallel()

		options.claimMachines(t, 3)

		clusterName := "integration-immediate"

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       2,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testDefaultCluster(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Create a regular 3 + 2 cluster with HA controlplane, assert that the cluster is ready and accessible.
Don't do any changes to the cluster.`)

		t.Parallel()

		clusterOptions := ClusterOptions{
			Name:          "integration-default",
			ControlPlanes: 3,
			Workers:       2,

			MachineOptions: options.MachineOptions,
		}

		options.claimMachines(t, clusterOptions.ControlPlanes+clusterOptions.Workers)

		runTests(t, AssertClusterCreateAndReady(t.Context(), options.omniClient, clusterOptions))
	}
}

func testEncryptedCluster(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Create a 1 + 1 cluster and enable disk encryption via Omni as a KMS.
Don't do any changes to the cluster.`)

		t.Parallel()

		clusterOptions := ClusterOptions{
			Name:          "integration-encrypted",
			ControlPlanes: 1,
			Workers:       1,

			MachineOptions: options.MachineOptions,
			Features: &specs.ClusterSpec_Features{
				DiskEncryption: true,
			},
		}

		options.claimMachines(t, clusterOptions.ControlPlanes+clusterOptions.Workers)

		runTests(t, AssertClusterCreateAndReady(t.Context(), options.omniClient, clusterOptions))
	}
}

func testSinglenodeCluster(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Create a single node cluster.
Don't do any changes to the cluster.`)

		t.Parallel()

		clusterOptions := ClusterOptions{
			Name:          "integration-singlenode",
			ControlPlanes: 1,
			Workers:       0,

			MachineOptions: options.MachineOptions,
		}

		options.claimMachines(t, clusterOptions.ControlPlanes+clusterOptions.Workers)

		runTests(t, AssertClusterCreateAndReady(t.Context(), options.omniClient, clusterOptions))
	}
}

func testScaleUpAndDown(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests scaling up and down a cluster:

- create a 1+0 cluster
- scale up to 1+1
- scale up to 3+1
- scale down to 3+0
- scale down to 1+0

In between the scaling operations, assert that the cluster is ready and accessible.`)

		t.Parallel()

		options.claimMachines(t, 4)

		clusterName := "integration-scaling"

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       0,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"OneWorkerShouldBeAdded",
			ScaleClusterUp(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  0,
				Workers:        1,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"TwoControlPlanesShouldBeAdded",
			ScaleClusterUp(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  2,
				Workers:        0,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"OneWorkerShouldBeRemoved",
			ScaleClusterDown(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  0,
				Workers:        -1,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"TwoControlPlanesShouldBeRemoved",
			ScaleClusterDown(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  -2,
				Workers:        0,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testScaleUpAndDownMachineClassBasedMachineSets(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests scaling up and down a cluster using machine classes:

- create a 1+0 cluster
- scale up to 1+1
- scale up to 3+1
- scale down to 3+0
- scale down to 1+0

In between the scaling operations, assert that the cluster is ready and accessible.`)

		t.Parallel()

		options.claimMachines(t, 4)

		clusterName := "integration-scaling-machine-class-based-machine-sets"

		t.Run(
			"ClusterShouldBeCreated",
			CreateClusterWithMachineClass(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       0,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"OneWorkerShouldBeAdded",
			ScaleClusterMachineSets(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  0,
				Workers:        1,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"TwoControlPlanesShouldBeAdded",
			ScaleClusterMachineSets(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  2,
				Workers:        0,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"OneWorkerShouldBeRemoved",
			ScaleClusterMachineSets(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  0,
				Workers:        -1,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"TwoControlPlanesShouldBeRemoved",
			ScaleClusterMachineSets(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  -2,
				Workers:        0,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testScaleUpAndDownAutoProvisionMachineSets(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests scaling up and down a cluster using infrastructure provisioner:

- create a 1+0 cluster
- scale up to 1+1
- scale up to 3+1
- scale down to 3+0
- scale down to 1+0

In between the scaling operations, assert that the cluster is ready and accessible.`)

		t.Parallel()

		clusterName := "integration-scaling-auto-provision"

		t.Run(
			"ClusterShouldBeCreated",
			CreateClusterWithMachineClass(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       0,
				InfraProvider: options.defaultInfraProvider(),

				MachineOptions: options.MachineOptions,
				ProviderData:   options.defaultProviderData(),
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"OneWorkerShouldBeAdded",
			ScaleClusterMachineSets(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  0,
				Workers:        1,
				InfraProvider:  options.defaultInfraProvider(),
				MachineOptions: options.MachineOptions,
				ProviderData:   options.defaultProviderData(),
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"TwoControlPlanesShouldBeAdded",
			ScaleClusterMachineSets(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  2,
				Workers:        0,
				MachineOptions: options.MachineOptions,
				ProviderData:   options.defaultInfraProvider(),
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"OneWorkerShouldBeRemoved",
			ScaleClusterMachineSets(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  0,
				Workers:        -1,
				InfraProvider:  options.defaultInfraProvider(),
				MachineOptions: options.MachineOptions,
				ProviderData:   options.defaultProviderData(),
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"TwoControlPlanesShouldBeRemoved",
			ScaleClusterMachineSets(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  -2,
				Workers:        0,
				InfraProvider:  options.defaultInfraProvider(),
				MachineOptions: options.MachineOptions,
				ProviderData:   options.defaultProviderData(),
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, true, false),
		)
	}
}

func testRollingUpdateParallelism(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests rolling update & scale down strategies for concurrency control for worker machine sets.

- create a 1+3 cluster
- update the worker configs with rolling strategy using maxParallelism of 2
- scale down the workers to 0 with rolling strategy using maxParallelism of 2
- assert that the maxParallelism of 2 was respected and used in both operations,`)

		t.Parallel()

		clusterName := "integration-rolling-update-parallelism"

		options.claimMachines(t, 4)

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       3,

				MachineOptions: options.MachineOptions,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"WorkersUpdateShouldBeRolledOutWithMaxParallelism",
			AssertWorkerNodesRollingConfigUpdate(t.Context(), options.omniClient, clusterName, 2),
		)

		t.Run(
			"WorkersShouldScaleDownWithMaxParallelism",
			AssertWorkerNodesRollingScaleDown(t.Context(), options.omniClient, clusterName, 2),
		)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testReplaceControlPlanes(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests replacing control plane nodes:

- create a 1+0 cluster
- scale up to 2+0, and immediately remove the first control plane node

In between the scaling operations, assert that the cluster is ready and accessible.`)

		t.Parallel()

		options.claimMachines(t, 2)

		clusterName := "integration-replace-cp"

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       0,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ControlPlanesShouldBeReplaced",
			ReplaceControlPlanes(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name: clusterName,

				MachineOptions: options.MachineOptions,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testConfigPatching(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests applying various config patching, including "broken" config patches which should not apply.`)

		t.Parallel()

		options.claimMachines(t, 4)

		clusterName := "integration-config-patching"

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 3,
				Workers:       1,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"LargeImmediateConfigPatchShouldBeAppliedAndRemoved",
			AssertLargeImmediateConfigApplied(t.Context(), options.omniClient, clusterName),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"MachineSetConfigPatchShouldBeAppliedAndRemoved",
			AssertConfigPatchMachineSet(t.Context(), options.omniClient, clusterName),
		)

		t.Run(
			"SingleClusterMachineConfigPatchShouldBeAppliedAndRemoved",
			AssertConfigPatchSingleClusterMachine(t.Context(), options.omniClient, clusterName),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ConfigPatchWithRebootShouldBeApplied",
			AssertConfigPatchWithReboot(t.Context(), options.omniClient, clusterName),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"InvalidConfigPatchShouldNotBeApplied",
			AssertConfigPatchWithInvalidConfig(t.Context(), options.omniClient, clusterName),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testTalosUpgrades(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests upgrading Talos version, including reverting a failed upgrade.`)

		t.Parallel()

		options.claimMachines(t, 4)

		clusterName := "integration-talos-upgrade"

		machineOptions := MachineOptions{
			TalosVersion:      options.AnotherTalosVersion,
			KubernetesVersion: options.AnotherKubernetesVersion, // use older Kubernetes compatible with AnotherTalosVersion
		}

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 3,
				Workers:       1,

				MachineOptions: machineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options, withTalosVersion(machineOptions.TalosVersion), withKubernetesVersion(machineOptions.KubernetesVersion))

		if !options.SkipExtensionsCheckOnCreate {
			t.Run(
				"HelloWorldServiceExtensionShouldBePresent",
				AssertExtensionIsPresent(t.Context(), options.omniClient, clusterName, HelloWorldServiceExtensionName),
			)
		}

		t.Run(
			"TalosSchematicUpdateShouldSucceed",
			AssertTalosSchematicUpdateFlow(t.Context(), options.omniClient, clusterName),
		)

		t.Run(
			"QemuGuestAgentExtensionShouldBePresent",
			AssertExtensionIsPresent(t.Context(), options.omniClient, clusterName, QemuGuestAgentExtensionName),
		)

		t.Run(
			"ClusterBootstrapManifestSyncShouldBeSuccessful",
			KubernetesBootstrapManifestSync(t.Context(), options.omniClient.Management(), clusterName),
		)

		t.Run(
			"TalosUpgradeShouldSucceed",
			AssertTalosUpgradeFlow(t.Context(), options.omniClient.Omni().State(), clusterName, options.MachineOptions.TalosVersion),
		)

		t.Run(
			"ClusterBootstrapManifestSyncShouldBeSuccessful",
			KubernetesBootstrapManifestSync(t.Context(), options.omniClient.Management(), clusterName),
		)

		if !options.SkipExtensionsCheckOnCreate {
			t.Run(
				"HelloWorldServiceExtensionShouldBePresent",
				AssertExtensionIsPresent(t.Context(), options.omniClient, clusterName, HelloWorldServiceExtensionName),
			)
		}

		assertClusterAndAPIReady(t, clusterName, options, withTalosVersion(options.MachineOptions.TalosVersion), withKubernetesVersion(machineOptions.KubernetesVersion))

		t.Run(
			"FailedTalosUpgradeShouldBeRevertible",
			AssertTalosUpgradeIsRevertible(t.Context(), options.omniClient.Omni().State(), clusterName, options.MachineOptions.TalosVersion),
		)

		t.Run(
			"RunningTalosUpgradeShouldBeCancelable",
			AssertTalosUpgradeIsCancelable(t.Context(), options.omniClient.Omni().State(), clusterName, options.MachineOptions.TalosVersion, options.AnotherTalosVersion),
		)

		assertClusterAndAPIReady(t, clusterName, options, withKubernetesVersion(machineOptions.KubernetesVersion))

		t.Run(
			"MaintenanceTestConfigShouldStillBePresent",
			AssertMaintenanceTestConfigIsPresent(t.Context(), options.omniClient.Omni().State(), clusterName, 0), // check the maintenance config in the first machine
		)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testKubernetesUpgrades(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests upgrading Kubernetes version, including reverting a failed upgrade.`)

		t.Parallel()

		options.claimMachines(t, 4)

		clusterName := "integration-k8s-upgrade"

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 3,
				Workers:       1,

				MachineOptions: MachineOptions{
					TalosVersion:      options.MachineOptions.TalosVersion,
					KubernetesVersion: options.AnotherKubernetesVersion,
				},
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options, withKubernetesVersion(options.AnotherKubernetesVersion))

		t.Run(
			"KubernetesUpgradeShouldSucceed",
			AssertKubernetesUpgradeFlow(
				t.Context(), options.omniClient.Omni().State(), options.omniClient.Management(),
				clusterName,
				options.MachineOptions.KubernetesVersion,
			),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"FailedKubernetesUpgradeShouldBeRevertible",
			AssertKubernetesUpgradeIsRevertible(t.Context(), options.omniClient.Omni().State(), clusterName, options.MachineOptions.KubernetesVersion),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testEtcdBackupAndRestore(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests automatic & manual backup & restore for workload etcd.

Automatic backups are enabled, done, and then a manual backup is created.
Afterwards, a cluster's control plane is destroyed then recovered from the backup.

Finally, a completely new cluster is created using the same backup to test the "point-in-time recovery".`)

		t.Parallel()

		options.claimMachines(t, 6)

		clusterName := "integration-etcd-backup"

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 3,
				Workers:       1,

				EtcdBackup: &specs.EtcdBackupConf{
					Interval: durationpb.New(2 * time.Hour),
					Enabled:  true,
				},
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		runTests(t,
			AssertBlockKubernetesDeploymentCreateAndRunning(t.Context(), options.omniClient.Management(),
				clusterName,
				"default",
				"test",
			),
		)

		t.Run(
			"KubernetesSecretShouldBeCreated",
			AssertKubernetesSecretIsCreated(t.Context(), options.omniClient.Management(),
				clusterName, "default", "test", "backup-test-secret-val"),
		)

		t.Run(
			"EtcdAutomaticBackupShouldBeCreated",
			AssertEtcdAutomaticBackupIsCreated(t.Context(), options.omniClient.Omni().State(), clusterName),
		)

		t.Run(
			"EtcdManualBackupShouldBeCreated",
			AssertEtcdManualBackupIsCreated(t.Context(), options.omniClient.Omni().State(), clusterName),
		)

		secondClusterName := "integration-etcd-backup-new-cluster"

		runTests(
			t,
			AssertBlockCreateClusterFromEtcdBackup(t.Context(), options.omniClient, options.Options,
				clusterName,
				secondClusterName,
				"default",
				"test",
			),
		)

		t.Run(
			"EtcdSecretShouldBeSameAfterCreateFromBackup",
			AssertKubernetesSecretHasValue(t.Context(), options.omniClient.Management(), secondClusterName, "default", "test", "backup-test-secret-val"),
		)

		t.Run(
			"NewClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), secondClusterName, false, false),
		)

		runTests(
			t,
			AssertBlockRestoreEtcdFromLatestBackup(t.Context(), options.omniClient, options.Options,
				3,
				clusterName,
				"default",
				"test",
			),
		)

		t.Run(
			"RestoredClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testMaintenanceUpgrade(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test upgrading (downgrading) a machine in maintenance mode.

Create a cluster out of a single machine on version1, remove cluster (the machine will stay on version1, Talos is installed).
Create a cluster out of the same machine on version2, Omni should upgrade the machine to version2 while in maintenance.`)

		t.Parallel()

		options.claimMachines(t, 1)

		t.Run(
			"MachineShouldBeUpgradedInMaintenanceMode",
			AssertMachineShouldBeUpgradedInMaintenanceMode(
				t.Context(), options.omniClient,
				"integration-maintenance-upgrade",
				options.AnotherKubernetesVersion,
				options.MachineOptions.TalosVersion,
				options.AnotherTalosVersion,
			),
		)
	}
}

func testAuth(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test authorization on accessing Omni API, some tests run without a cluster, some only run with a context of a cluster.`)

		t.Parallel()

		options.claimMachines(t, 1)

		t.Run(
			"AnonymousRequestShouldBeDenied",
			AssertAnonymousAuthentication(t.Context(), options.omniClient),
		)

		t.Run(
			"UnauthenticatedRequestsShouldBeAllowedByLocalResourceServer",
			AssertUnauthenticatedLocalResourceServerAccess(t.Context()),
		)

		t.Run(
			"InvalidSignatureShouldBeDenied",
			AssertAPIInvalidSignature(t.Context(), options.omniClient),
		)

		t.Run(
			"PublicKeyWithoutLifetimeShouldNotBeRegistered",
			AssertPublicKeyWithoutLifetimeNotRegistered(t.Context(), options.omniClient),
		)

		t.Run(
			"PublicKeyWithLongLifetimeShouldNotBeRegistered",
			AssertPublicKeyWithLongLifetimeNotRegistered(t.Context(), options.omniClient),
		)

		t.Run(
			"OmniconfigShouldBeDownloadable",
			AssertOmniconfigDownload(t.Context(), options.omniClient),
		)

		t.Run(
			"PublicKeyWithUnknownEmailShouldNotBeRegistered",
			AssertRegisterPublicKeyWithUnknownEmail(t.Context(), options.omniClient),
		)

		t.Run(
			"ServiceAccountAPIShouldWork",
			AssertServiceAccountAPIFlow(t.Context(), options.omniClient),
		)

		t.Run(
			"ResourceAuthzShouldWork",
			AssertResourceAuthz(t.Context(), options.omniClient, options.clientConfig),
		)

		t.Run(
			"ResourceAuthzWithACLShouldWork",
			AssertResourceAuthzWithACL(t.Context(), options.omniClient, options.clientConfig),
		)

		clusterName := "integration-auth"

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       0,

				Features: &specs.ClusterSpec_Features{
					UseEmbeddedDiscoveryService: true,
				},

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"APIAuthorizationShouldBeTested",
			AssertAPIAuthz(t.Context(), options.omniClient, options.clientConfig, clusterName),
		)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}

func testClusterTemplate(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test flow of cluster creation and scaling using cluster templates.`)

		t.Parallel()

		options.claimMachines(t, 5)

		t.Run(
			"TestClusterTemplateFlow",
			AssertClusterTemplateFlow(t.Context(), options.omniClient.Omni().State(), options.MachineOptions),
		)
	}
}

func testWorkloadProxy(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test workload service proxying feature`)

		t.Parallel()

		options.claimMachines(t, 6)

		omniClient := options.omniClient
		cluster1 := "integration-workload-proxy-1"
		cluster2 := "integration-workload-proxy-2"

		t.Run("ClusterShouldBeCreated-"+cluster1, CreateCluster(t.Context(), omniClient, ClusterOptions{
			Name:          cluster1,
			ControlPlanes: 1,
			Workers:       1,

			Features: &specs.ClusterSpec_Features{
				EnableWorkloadProxy: true,
			},

			MachineOptions: options.MachineOptions,
			ScalingTimeout: options.ScalingTimeout,

			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,

			AllowSchedulingOnControlPlanes: true,
		}))
		t.Run("ClusterShouldBeCreated-"+cluster2, CreateCluster(t.Context(), omniClient, ClusterOptions{
			Name:          cluster2,
			ControlPlanes: 1,
			Workers:       2,

			Features: &specs.ClusterSpec_Features{
				EnableWorkloadProxy: true,
			},

			MachineOptions: options.MachineOptions,
			ScalingTimeout: options.ScalingTimeout,

			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,

			AllowSchedulingOnControlPlanes: true,
		}))

		runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(t.Context(), omniClient, cluster1, options.MachineOptions.TalosVersion,
			options.MachineOptions.KubernetesVersion))
		runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(t.Context(), omniClient, cluster2, options.MachineOptions.TalosVersion,
			options.MachineOptions.KubernetesVersion))

		parentCtx := t.Context()

		t.Run("WorkloadProxyShouldBeTested", func(t *testing.T) {
			workloadproxy.Test(parentCtx, t, omniClient, cluster1, cluster2)
		})

		t.Run("ClusterShouldBeDestroyed-"+cluster1, AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), cluster1, false, false))
		t.Run("ClusterShouldBeDestroyed-"+cluster2, AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), cluster2, false, false))
	}
}

func testStaticInfraProvider(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Tests common Omni operations on machines created by a static infrastructure provider:,
Note: this test expects all machines to be provisioned by the bare-metal infra provider as it doesn't filter them.

- create a 1+0 cluster - assert that cluster is healthy and ready
- scale it up to be 3+1 - assert that cluster is healthy and ready
- assert that machines are not ready to use (occupied)
- scale it down to be 1+0 - assert that cluster is healthy and ready
- destroy the cluster - assert that machines are wiped, then marked as ready to use
- create a new 3+1 cluster
- assert that cluster is healthy and ready
- remove links of the machines
`)
		t.Parallel()

		clusterName := "integration-static-infra-provider"

		t.Run(
			"ClusterShouldBeCreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 1,
				Workers:       0,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: true,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeScaledUp",
			ScaleClusterUp(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  2,
				Workers:        1,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ExtensionsShouldBeUpdated",
			UpdateExtensions(t.Context(), options.omniClient, clusterName, []string{"siderolabs/binfmt-misc", "siderolabs/glibc"}),
		)

		t.Run(
			"MachinesShouldBeAllocated",
			AssertInfraMachinesAreAllocated(t.Context(), options.omniClient.Omni().State(), clusterName,
				options.MachineOptions.TalosVersion, []string{"siderolabs/binfmt-misc", "siderolabs/glibc"}),
		)

		t.Run(
			"ClusterShouldBeScaledDown",
			ScaleClusterDown(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  -2,
				Workers:        -1,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, true),
		)

		t.Run(
			"ClusterShouldBeRecreated",
			CreateCluster(t.Context(), options.omniClient, ClusterOptions{
				Name:          clusterName,
				ControlPlanes: 3,
				Workers:       1,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,

				SkipExtensionCheckOnCreate: true,
			}),
		)

		assertClusterAndAPIReady(t, clusterName, options)

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, true),
		)
	}
}

func testOmniUpgradePrepare(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test Omni upgrades, the first half that runs on the previous Omni version

- create 3+1 cluster
- enable and verify workload proxying
- save cluster snapshot in the cluster resource for the future use`)

		t.Parallel()

		options.claimMachines(t, 4)

		omniClient := options.omniClient
		clusterName := "integration-omni-upgrades"

		t.Run("ClusterShouldBeCreated", CreateCluster(t.Context(), omniClient, ClusterOptions{
			Name:          clusterName,
			ControlPlanes: 3,
			Workers:       1,

			Features: &specs.ClusterSpec_Features{
				EnableWorkloadProxy: true,
			},

			MachineOptions: options.MachineOptions,
			ScalingTimeout: options.ScalingTimeout,

			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,

			AllowSchedulingOnControlPlanes: true,
		}))

		runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(t.Context(), omniClient, clusterName, options.MachineOptions.TalosVersion,
			options.MachineOptions.KubernetesVersion))

		parentCtx := t.Context()

		t.Run("WorkloadProxyShouldBeTested", func(t *testing.T) {
			workloadproxy.Test(parentCtx, t, omniClient, clusterName)
		})

		t.Run("SaveClusterSnapshot", SaveClusterSnapshot(t.Context(), omniClient, clusterName))
	}
}

func testOmniUpgradeVerify(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test Omni upgrades, the second half that runs on the current Omni version

- check that the cluster exists and is healthy
- verify that machines were not restarted
- check that machine configuration was not changed
- verify workload proxying still works
- scale up the cluster by one worker`)

		t.Parallel()

		options.claimMachines(t, 5)

		omniClient := options.omniClient
		clusterName := "integration-omni-upgrades"

		runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(t.Context(), omniClient, clusterName, options.MachineOptions.TalosVersion,
			options.MachineOptions.KubernetesVersion))

		parentCtx := t.Context()

		t.Run("AssertMachinesNotRebootedConfigUnchanged", AssertClusterSnapshot(t.Context(), omniClient, clusterName))

		t.Run("WorkloadProxyShouldBeTested", func(t *testing.T) {
			workloadproxy.Test(parentCtx, t, omniClient, clusterName)
		})

		t.Run(
			"OneWorkerShouldBeAdded",
			ScaleClusterUp(t.Context(), options.omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  0,
				Workers:        1,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		)

		runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(t.Context(), omniClient, clusterName, options.MachineOptions.TalosVersion,
			options.MachineOptions.KubernetesVersion))

		t.Run(
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false),
		)
	}
}
