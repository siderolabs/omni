// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package tests provides the Omni tests.
package tests

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/cmd/integration-test/pkg/clientconfig"
)

// TestFunc is a testing function prototype.
type TestFunc func(t *testing.T)

// RestartAMachineFunc is a function to restart a machine by UUID.
type RestartAMachineFunc func(ctx context.Context, uuid string) error

// WipeAMachineFunc is a function to wipe a machine by UUID.
type WipeAMachineFunc func(ctx context.Context, uuid string) error

// FreezeAMachineFunc is a function to freeze a machine by UUID.
type FreezeAMachineFunc func(ctx context.Context, uuid string) error

// HTTPRequestSignerFunc is function to sign the HTTP request.
type HTTPRequestSignerFunc func(ctx context.Context, req *http.Request) error

// TalosAPIKeyPrepareFunc is a function to prepare a public key for Talos API auth.
type TalosAPIKeyPrepareFunc func(ctx context.Context, contextName string) error

// Options for the test runner.
//
//nolint:govet
type Options struct {
	RunTestPattern string

	CleanupLinks     bool
	RunStatsCheck    bool
	ExpectedMachines int

	RestartAMachineFunc RestartAMachineFunc
	WipeAMachineFunc    WipeAMachineFunc
	FreezeAMachineFunc  FreezeAMachineFunc

	MachineOptions MachineOptions

	HTTPEndpoint             string
	AnotherTalosVersion      string
	AnotherKubernetesVersion string
	OmnictlPath              string
}

// Run the integration tests.
//
//nolint:maintidx
func Run(ctx context.Context, clientConfig *clientconfig.ClientConfig, options Options) error {
	rootClient, err := clientConfig.GetClient()
	if err != nil {
		return err
	}

	talosAPIKeyPrepare := func(ctx context.Context, contextName string) error {
		return clientconfig.TalosAPIKeyPrepare(ctx, rootClient, contextName)
	}

	testList := []testGroup{
		{
			Name: "CleanState",
			Description: `
Bring the state of Omni to a clean state by removing all clusters, config patches, etc. which might have been left from previous runs.
Wait for all expected machines to join and be in maintenance mode.`,
			Parallel: false, // these tests should run first without other tests interfering
			Subtests: []subTest{
				{
					"DestroyAllClusterRelatedResources",
					DestroyAllClusterRelatedResources(ctx, rootClient.Omni().State()),
				},
				// machine discovery, all machines should be in maintenance mode
				{
					"LinkCountShouldMatchExpectedMachines",
					AssertNumberOfLinks(ctx, rootClient.Omni().State(), options.ExpectedMachines),
				},
				{
					"LinksShouldBeConnected",
					AssertLinksConnected(ctx, rootClient.Omni().State()),
				},
				{
					"LinksShouldMatchMachines",
					AssertMachinesMatchLinks(ctx, rootClient.Omni().State()),
				},
				{
					"MachinesShouldHaveLogs",
					AssertMachinesHaveLogs(ctx, rootClient.Omni().State(), rootClient.Management()),
				},
				{
					"MachinesShouldBeReachableInMaintenanceMode",
					AssertTalosMaintenanceAPIAccessViaOmni(ctx, rootClient, talosAPIKeyPrepare),
				},
				{
					"MachinesShouldBeInMaintenanceMode",
					AssertMachineStatus(ctx, rootClient.Omni().State(), true, "", map[string]string{
						omni.MachineStatusLabelConnected:       "",
						omni.MachineStatusLabelReportingEvents: "",
						omni.MachineStatusLabelAvailable:       "",
						// QEMU-specific labels which should always match, others are specific to the settings (number of cores, etc.)
						omni.MachineStatusLabelCPU:      "qemu",
						omni.MachineStatusLabelArch:     "amd64",
						omni.MachineStatusLabelPlatform: "metal",
					}, nil),
				},
			},
		},
		{
			Name: "TalosImageGeneration",
			Description: `
Generate various Talos images with Omni and try to download them.`,
			Parallel: true,
			Subtests: []subTest{
				{
					"TalosImagesShouldBeDownloadableUsingCLI",
					AssertDownloadUsingCLI(ctx, rootClient, options.OmnictlPath, options.HTTPEndpoint),
				},
				{
					"TalosImagesShouldBeDownloadable",
					AssertSomeImagesAreDownloadable(ctx, rootClient, func(ctx context.Context, req *http.Request) error {
						return clientconfig.SignHTTPRequest(ctx, rootClient, req)
					}, options.HTTPEndpoint),
				},
			},
		},
		{
			Name:         "KubernetesNodeAudit",
			Description:  "Test the auditing of the Kubernetes nodes, i.e. when a node is gone from the Omni perspective but still exists on the Kubernetes cluster.",
			Parallel:     true,
			MachineClaim: 2,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-k8s-node-audit",
						ControlPlanes: 1,
						Workers:       1,

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-k8s-node-audit",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"KubernetesNodeAuditShouldBePerformed",
					AssertKubernetesNodeAudit(ctx, rootClient.Omni().State(), "integration-k8s-node-audit", rootClient, options),
				},
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-k8s-node-audit"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-k8s-node-audit"),
		},
		{
			Name: "ForcedMachineRemoval",
			Description: `
Tests different scenarios for forced Machine removal (vs. graceful removing from a cluster):

- force remove a Machine which is not allocated (not part of any cluster)
- force remove a worker Machine which is part of the cluster
- force remove a control plane Machine which is part of the cluster, and replace with a new Machine.

These tests simulate a hardware failure of a Machine which requires a forced removal from Omni.

In the tests, we wipe and reboot the VMs to bring them back as available for the next test.`,
			Parallel:     true,
			MachineClaim: 4,
			Subtests: subTests(
				// this test will force-remove a machine, but it will bring it back, so pool of available will be still 4 machines
				subTest{
					"UnallocatedMachinesShouldBeDestroyable",
					AssertUnallocatedMachineDestroyFlow(ctx, rootClient.Omni().State(), options.RestartAMachineFunc),
				},
				// this test consumes all 4 available machines and creates a cluster
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-forced-removal",
						ControlPlanes: 3,
						Workers:       1,

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterShouldBeReady(ctx, rootClient, "integration-forced-removal", options.MachineOptions.TalosVersion, talosAPIKeyPrepare)...,
			).Append(
				// this test will force-remove a worker, so the cluster will be 3+0, and 1 available machine
				subTest{
					"WorkerNodesShouldBeForceRemovable",
					AssertForceRemoveWorkerNode(ctx, rootClient.Omni().State(), "integration-forced-removal", options.FreezeAMachineFunc, options.WipeAMachineFunc),
				},
			).Append(
				TestBlockClusterShouldBeReady(ctx, rootClient, "integration-forced-removal", options.MachineOptions.TalosVersion, talosAPIKeyPrepare)...,
			).Append(
				// this test will add an available machine as a fourth control plane node, but then remove a frozen one, so the cluster is 3+0, and 1 available machine
				subTest{
					"ControlPlaneNodeShouldBeForceReplaceable",
					AssertControlPlaneForceReplaceMachine(ctx, rootClient.Omni().State(), "integration-forced-removal", options),
				},
			).Append(
				TestBlockClusterShouldBeReady(ctx, rootClient, "integration-forced-removal", options.MachineOptions.TalosVersion, talosAPIKeyPrepare)...,
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-forced-removal"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-forced-removal"),
		},
		{
			Name: "ImmediateClusterDestruction",
			Description: `
Regression test: create a cluster and destroy it without waiting for the cluster to reach any state.`,
			Parallel:     true,
			MachineClaim: 3,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-immediate",
						ControlPlanes: 1,
						Workers:       2,

						MachineOptions: options.MachineOptions,
					}),
				},
				subTest{
					"ClusterShouldBeDestroyedImmediately",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-immediate"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-immediate"),
		},
		TestGroupClusterCreateAndReady(
			ctx,
			rootClient,
			talosAPIKeyPrepare,
			"default",
			`
Create a regular 3 + 2 cluster with HA controlplane, assert that the cluster is ready and accessible.
Don't do any changes to the cluster.`,
			ClusterOptions{
				ControlPlanes: 3,
				Workers:       2,

				MachineOptions: options.MachineOptions,
			},
		),
		TestGroupClusterCreateAndReady(
			ctx,
			rootClient,
			talosAPIKeyPrepare,
			"encrypted",
			`
Create a 1 + 1 cluster and enable disk encryption via Omni as a KMS.
Don't do any changes to the cluster.`,
			ClusterOptions{
				ControlPlanes: 1,
				Workers:       1,

				MachineOptions: options.MachineOptions,
				Features: &specs.ClusterSpec_Features{
					DiskEncryption: true,
				},
			},
		),
		TestGroupClusterCreateAndReady(
			ctx,
			rootClient,
			talosAPIKeyPrepare,
			"singlenode",
			`
Create a single node cluster.
Don't do any changes to the cluster.`,
			ClusterOptions{
				ControlPlanes: 1,
				Workers:       0,

				MachineOptions: options.MachineOptions,
			},
		),
		{
			Name: "ScaleUpAndDown",
			Description: `
Tests scaling up and down a cluster:

- create a 1+0 cluster
- scale up to 1+1
- scale up to 3+1
- scale down to 3+0
- scale down to 1+0

In between the scaling operations, assert that the cluster is ready and accessible.`,
			Parallel:     true,
			MachineClaim: 4,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-scaling",
						ControlPlanes: 1,
						Workers:       0,

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"OneWorkerShouldBeAdded",
					ScaleClusterUp(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:           "integration-scaling",
						ControlPlanes:  0,
						Workers:        1,
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"TwoControlPlanesShouldBeAdded",
					ScaleClusterUp(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:           "integration-scaling",
						ControlPlanes:  2,
						Workers:        0,
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"OneWorkerShouldBeRemoved",
					ScaleClusterDown(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:           "integration-scaling",
						ControlPlanes:  0,
						Workers:        -1,
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"TwoControlPlanesShouldBeRemoved",
					ScaleClusterDown(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:           "integration-scaling",
						ControlPlanes:  -2,
						Workers:        0,
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-scaling"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-scaling"),
		},
		{
			Name: "ScaleUpAndDownMachineClassBasedMachineSets",
			Description: `
Tests scaling up and down a cluster using machine classes:

- create a 1+0 cluster
- scale up to 1+1
- scale up to 3+1
- scale down to 3+0
- scale down to 1+0

In between the scaling operations, assert that the cluster is ready and accessible.`,
			Parallel:     true,
			MachineClaim: 4,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateClusterWithMachineClass(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:          "integration-scaling-machine-class-based-machine-sets",
						ControlPlanes: 1,
						Workers:       0,

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling-machine-class-based-machine-sets",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"OneWorkerShouldBeAdded",
					ScaleClusterMachineSets(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:           "integration-scaling-machine-class-based-machine-sets",
						ControlPlanes:  0,
						Workers:        1,
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling-machine-class-based-machine-sets",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"TwoControlPlanesShouldBeAdded",
					ScaleClusterMachineSets(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:           "integration-scaling-machine-class-based-machine-sets",
						ControlPlanes:  2,
						Workers:        0,
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling-machine-class-based-machine-sets",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"OneWorkerShouldBeRemoved",
					ScaleClusterMachineSets(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:           "integration-scaling-machine-class-based-machine-sets",
						ControlPlanes:  0,
						Workers:        -1,
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling-machine-class-based-machine-sets",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"TwoControlPlanesShouldBeRemoved",
					ScaleClusterMachineSets(ctx, rootClient.Omni().State(), ClusterOptions{
						Name:           "integration-scaling-machine-class-based-machine-sets",
						ControlPlanes:  -2,
						Workers:        0,
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-scaling-machine-class-based-machine-sets",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-scaling-machine-class-based-machine-sets"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-scaling-machine-class-based-machine-sets"),
		},
		{
			Name: "RollingUpdateParallelism",
			Description: `
Tests rolling update & scale down strategies for concurrency control for worker machine sets.

- create a 1+3 cluster
- update the worker configs with rolling strategy using maxParallelism of 2
- scale down the workers to 0 with rolling strategy using maxParallelism of 2
- assert that the maxParallelism of 2 was respected and used in both operations,`,
			Parallel:     true,
			MachineClaim: 4,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-rolling-update-parallelism",
						ControlPlanes: 1,
						Workers:       3,

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-rolling-update-parallelism",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"WorkersUpdateShouldBeRolledOutWithMaxParallelism",
					AssertWorkerNodesRollingConfigUpdate(ctx, rootClient, "integration-rolling-update-parallelism", 2),
				},
				subTest{
					"WorkersShouldScaleDownWithMaxParallelism",
					AssertWorkerNodesRollingScaleDown(ctx, rootClient, "integration-rolling-update-parallelism", 2),
				},
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-rolling-update-parallelism"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-rolling-update-parallelism"),
		},
		{
			Name: "ReplaceControlPlanes",
			Description: `
Tests replacing control plane nodes:

- create a 1+0 cluster
- scale up to 2+0, and immediately remove the first control plane node

In between the scaling operations, assert that the cluster is ready and accessible.`,
			Parallel:     true,
			MachineClaim: 2,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-replace-cp",
						ControlPlanes: 1,
						Workers:       0,

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-replace-cp",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"ControlPlanesShouldBeReplaced",
					ReplaceControlPlanes(ctx, rootClient.Omni().State(), ClusterOptions{
						Name: "integration-replace-cp",

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-replace-cp",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-replace-cp"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-replace-cp"),
		},
		{
			Name: "ConfigPatching",
			Description: `
Tests applying various config patching, including "broken" config patches which should not apply.`,
			Parallel:     true,
			MachineClaim: 4,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-config-patching",
						ControlPlanes: 3,
						Workers:       1,

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-config-patching",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"LargeImmediateConfigPatchShouldBeAppliedAndRemoved",
					AssertLargeImmediateConfigApplied(ctx, rootClient, "integration-config-patching", talosAPIKeyPrepare),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-config-patching",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"MachineSetConfigPatchShouldBeAppliedAndRemoved",
					AssertConfigPatchMachineSet(ctx, rootClient, "integration-config-patching"),
				},
				subTest{
					"SingleClusterMachineConfigPatchShouldBeAppliedAndRemoved",
					AssertConfigPatchSingleClusterMachine(ctx, rootClient, "integration-config-patching"),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-config-patching",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"ConfigPatchWithRebootShouldBeApplied",
					AssertConfigPatchWithReboot(ctx, rootClient, "integration-config-patching", talosAPIKeyPrepare),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-config-patching",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"InvalidConfigPatchShouldNotBeApplied",
					AssertConfigPatchWithInvalidConfig(ctx, rootClient, "integration-config-patching", talosAPIKeyPrepare),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-config-patching",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-config-patching"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-config-patching"),
		},
		{
			Name: "TalosUpgrades",
			Description: `
Tests upgrading Talos version, including reverting a failed upgrade.`,
			Parallel:     true,
			MachineClaim: 4,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-talos-upgrade",
						ControlPlanes: 3,
						Workers:       1,

						MachineOptions: MachineOptions{
							TalosVersion:      options.AnotherTalosVersion,
							KubernetesVersion: options.AnotherKubernetesVersion, // use older Kubernetes compatible with AnotherTalosVersion
						},
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-talos-upgrade",
					options.AnotherTalosVersion,
					options.AnotherKubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"HelloWorldServiceExtensionShouldBePresent",
					AssertExtensionIsPresent(ctx, rootClient, "integration-talos-upgrade", HelloWorldServiceExtensionName),
				},
				subTest{
					"TalosSchematicUpdateShouldSucceed",
					AssertTalosSchematicUpdateFlow(ctx, rootClient, "integration-talos-upgrade"),
				},
				subTest{
					"QemuGuestAgentExtensionShouldBePresent",
					AssertExtensionIsPresent(ctx, rootClient, "integration-talos-upgrade", QemuGuestAgentExtensionName),
				},
				subTest{
					"ClusterBootstrapManifestSyncShouldBeSuccessful",
					KubernetesBootstrapManifestSync(ctx, rootClient.Management(), "integration-talos-upgrade"),
				},
			).Append(
				subTest{
					"TalosUpgradeShouldSucceed",
					AssertTalosUpgradeFlow(ctx, rootClient.Omni().State(), "integration-talos-upgrade", options.MachineOptions.TalosVersion),
				},
				subTest{
					"ClusterBootstrapManifestSyncShouldBeSuccessful",
					KubernetesBootstrapManifestSync(ctx, rootClient.Management(), "integration-talos-upgrade"),
				},
				subTest{
					"HelloWorldServiceExtensionShouldBePresent",
					AssertExtensionIsPresent(ctx, rootClient, "integration-talos-upgrade", HelloWorldServiceExtensionName),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-talos-upgrade",
					options.MachineOptions.TalosVersion,
					options.AnotherKubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"FailedTalosUpgradeShouldBeRevertible",
					AssertTalosUpgradeIsRevertible(ctx, rootClient.Omni().State(), "integration-talos-upgrade", options.MachineOptions.TalosVersion),
				},
			).Append(
				subTest{
					"RunningTalosUpgradeShouldBeCancelable",
					AssertTalosUpgradeIsCancelable(ctx, rootClient.Omni().State(), "integration-talos-upgrade", options.MachineOptions.TalosVersion, options.AnotherTalosVersion),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-talos-upgrade",
					options.MachineOptions.TalosVersion,
					options.AnotherKubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"MaintenanceTestConfigShouldStillBePresent",
					AssertMaintenanceTestConfigIsPresent(ctx, rootClient.Omni().State(), "integration-talos-upgrade", 0), // check the maintenance config in the first machine
				},
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-talos-upgrade"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-talos-upgrade"),
		},
		{
			Name: "KubernetesUpgrades",
			Description: `
Tests upgrading Kubernetes version, including reverting a failed upgrade.`,
			Parallel:     true,
			MachineClaim: 4,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-k8s-upgrade",
						ControlPlanes: 3,
						Workers:       1,

						MachineOptions: MachineOptions{
							TalosVersion:      options.MachineOptions.TalosVersion,
							KubernetesVersion: options.AnotherKubernetesVersion,
						},
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-k8s-upgrade",
					options.MachineOptions.TalosVersion,
					options.AnotherKubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"KubernetesUpgradeShouldSucceed",
					AssertKubernetesUpgradeFlow(
						ctx, rootClient.Omni().State(), rootClient.Management(),
						"integration-k8s-upgrade",
						options.MachineOptions.KubernetesVersion,
					),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-k8s-upgrade",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"FailedKubernetesUpgradeShouldBeRevertible",
					AssertKubernetesUpgradeIsRevertible(ctx, rootClient.Omni().State(), "integration-k8s-upgrade", options.MachineOptions.KubernetesVersion),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-k8s-upgrade",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-k8s-upgrade"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-k8s-upgrade"),
		},
		{
			Name: "EtcdBackupAndRestore",
			Description: `
Tests automatic & manual backup & restore for workload etcd.

Automatic backups are enabled, done, and then a manual backup is created.
Afterwards, a cluster's control plane is destroyed then recovered from the backup.

Finally, a completely new cluster is created using the same backup to test the "point-in-time recovery".`,
			Parallel:     true,
			MachineClaim: 6,
			Subtests: subTests(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-etcd-backup",
						ControlPlanes: 3,
						Workers:       1,

						EtcdBackup: &specs.EtcdBackupConf{
							Interval: durationpb.New(2 * time.Hour),
							Enabled:  true,
						},
						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-etcd-backup",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				TestBlockKubernetesDeploymentCreateAndRunning(ctx, rootClient.Management(), "integration-etcd-backup",
					"default",
					"test",
				)...,
			).Append(
				subTest{
					"KubernetesSecretShouldBeCreated",
					AssertKubernetesSecretIsCreated(ctx, rootClient.Management(), "integration-etcd-backup", "default", "test", "backup-test-secret-val"),
				},
				subTest{
					"EtcdAutomaticBackupShouldBeCreated",
					AssertEtcdAutomaticBackupIsCreated(ctx, rootClient.Omni().State(), "integration-etcd-backup"),
				},
				subTest{
					"EtcdManualBackupShouldBeCreated",
					AssertEtcdManualBackupIsCreated(ctx, rootClient.Omni().State(), "integration-etcd-backup"),
				},
			).Append(
				TestBlockCreateClusterFromEtcdBackup(ctx, rootClient, talosAPIKeyPrepare, options,
					"integration-etcd-backup",
					"integration-etcd-backup-new-cluster",
					"default",
					"test",
				)...,
			).Append(
				subTest{
					"EtcdSecretShouldBeSameAfterCreateFromBackup",
					AssertKubernetesSecretHasValue(ctx, rootClient.Management(), "integration-etcd-backup-new-cluster", "default", "test", "backup-test-secret-val"),
				},
				subTest{
					"NewClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-etcd-backup-new-cluster"),
				},
			).Append(
				TestBlockRestoreEtcdFromLatestBackup(ctx, rootClient, talosAPIKeyPrepare, options, 3,
					"integration-etcd-backup",
					"default",
					"test",
				)...,
			).Append(
				subTest{
					"RestoredClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-etcd-backup"),
				},
			),
			Finalizer: func(t *testing.T) {
				DestroyCluster(ctx, rootClient.Omni().State(), "integration-etcd-backup")(t)
				DestroyCluster(ctx, rootClient.Omni().State(), "integration-etcd-backup-new-cluster")(t)
			},
		},
		{
			Name: "MaintenanceUpgrade",
			Description: `
		Test upgrading (downgrading) a machine in maintenance mode.

		Create a cluster out of a single machine on version1, remove cluster (the machine will stay on version1, Talos is installed).
		Create a cluster out of the same machine on version2, Omni should upgrade the machine to version2 while in maintenance.
		`,
			Parallel:     true,
			MachineClaim: 1,
			Subtests: subTests(
				subTest{
					"MachineShouldBeUpgradedInMaintenanceMode",
					AssertMachineShouldBeUpgradedInMaintenanceMode(
						ctx, rootClient,
						"integration-maintenance-upgrade",
						options.AnotherKubernetesVersion,
						options.MachineOptions.TalosVersion,
						options.AnotherTalosVersion,
						talosAPIKeyPrepare,
					),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-maintenance-upgrade"),
		},
		{
			Name: "Auth",
			Description: `
Test authorization on accessing Omni API, some tests run without a cluster, some only run with a context of a cluster.`,
			MachineClaim: 1,
			Parallel:     true,
			Subtests: subTests(
				subTest{
					"AnonymousRequestShouldBeDenied",
					AssertAnonymousAuthenication(ctx, rootClient),
				},
				subTest{
					"InvalidSignatureShouldBeDenied",
					AssertAPIInvalidSignature(ctx, rootClient),
				},
				subTest{
					"PublicKeyWithoutLifetimeShouldNotBeRegistered",
					AssertPublicKeyWithoutLifetimeNotRegistered(ctx, rootClient),
				},
				subTest{
					"PublicKeyWithLongLifetimeShouldNotBeRegistered",
					AssertPublicKeyWithLongLifetimeNotRegistered(ctx, rootClient),
				},
				subTest{
					"OmniconfigShouldBeDownloadable",
					AssertOmniconfigDownload(ctx, rootClient),
				},
				subTest{
					"PublicKeyWithUnknownEmailShouldNotBeRegistered",
					AssertRegisterPublicKeyWithUnknownEmail(ctx, rootClient),
				},
				subTest{
					"ServiceAccountAPIShouldWork",
					AssertServiceAccountAPIFlow(ctx, rootClient),
				},
				subTest{
					"ResourceAuthzShouldWork",
					AssertResourceAuthz(ctx, rootClient, clientConfig),
				},
				subTest{
					"ResourceAuthzWithACLShouldWork",
					AssertResourceAuthzWithACL(ctx, rootClient, clientConfig),
				},
			).Append(
				subTest{
					"ClusterShouldBeCreated",
					CreateCluster(ctx, rootClient, ClusterOptions{
						Name:          "integration-auth",
						ControlPlanes: 1,
						Workers:       0,

						Features: &specs.ClusterSpec_Features{
							UseEmbeddedDiscoveryService: true,
						},

						MachineOptions: options.MachineOptions,
					}),
				},
			).Append(
				TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
					ctx, rootClient,
					"integration-auth",
					options.MachineOptions.TalosVersion,
					options.MachineOptions.KubernetesVersion,
					talosAPIKeyPrepare,
				)...,
			).Append(
				subTest{
					"APIAuthorizationShouldBeTested",
					AssertAPIAuthz(ctx, rootClient, clientConfig, "integration-auth"),
				},
			).Append(
				subTest{
					"ClusterShouldBeDestroyed",
					AssertDestroyCluster(ctx, rootClient.Omni().State(), "integration-auth"),
				},
			),
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "integration-auth"),
		},
		{
			Name: "ClusterTemplate",
			Description: `
Test flow of cluster creation and scaling using cluster templates.`,
			Parallel:     true,
			MachineClaim: 5,
			Subtests: []subTest{
				{
					"TestClusterTemplateFlow",
					AssertClusterTemplateFlow(ctx, rootClient.Omni().State()),
				},
			},
			Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), "tmpl-cluster"),
		},
	}

	var re *regexp.Regexp

	if options.RunTestPattern != "" {
		var err error

		if re, err = regexp.Compile(options.RunTestPattern); err != nil {
			log.Printf("run test pattern parse error: %s", err)

			return err
		}
	}

	var testsToRun []testGroup

	for _, group := range testList {
		if re == nil || re.MatchString(group.Name) {
			testsToRun = append(testsToRun, group)

			continue
		}

		matchedGroup := group
		matchedGroup.Subtests = xslices.Filter(matchedGroup.Subtests, func(test subTest) bool {
			fullName := fmt.Sprintf("%s/%s", group.Name, test.Name)

			return re.MatchString(fullName)
		})

		if len(matchedGroup.Subtests) > 0 {
			testsToRun = append(testsToRun, matchedGroup)
		}
	}

	for _, group := range testsToRun {
		if group.MachineClaim > options.ExpectedMachines {
			return fmt.Errorf("test group %q requires %d machines, but only %d are expected", group.Name, group.MachineClaim, options.ExpectedMachines)
		}
	}

	machineSemaphore := semaphore.NewWeighted(int64(options.ExpectedMachines))

	exitCode := testing.MainStart(
		matchStringOnly(func(string, string) (bool, error) { return true, nil }),
		makeTests(ctx, testsToRun, machineSemaphore),
		nil,
		nil,
		nil,
	).Run()

	extraTests := []testing.InternalTest{}

	if options.RunStatsCheck {
		extraTests = append(extraTests, testing.InternalTest{
			Name: "AssertStatsLimits",
			F:    AssertStatsLimits(ctx),
		})
	}

	if len(extraTests) > 0 && exitCode == 0 {
		exitCode = testing.MainStart(
			matchStringOnly(func(string, string) (bool, error) { return true, nil }),
			extraTests,
			nil,
			nil,
			nil,
		).Run()
	}

	if options.CleanupLinks {
		if err := cleanupLinks(ctx, rootClient.Omni().State()); err != nil {
			return err
		}
	}

	if exitCode != 0 {
		return errors.New("test failed")
	}

	return nil
}

func cleanupLinks(ctx context.Context, st state.State) error {
	links, err := safe.ReaderListAll[*siderolink.Link](ctx, st)
	if err != nil {
		return err
	}

	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()

	return links.ForEachErr(func(r *siderolink.Link) error {
		_, err := st.Teardown(ctx, r.Metadata())
		if err != nil {
			return err
		}

		_, err = st.WatchFor(ctx, r.Metadata(), state.WithFinalizerEmpty())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		err = st.Destroy(ctx, r.Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		return nil
	})
}

func makeTests(ctx context.Context, testsToRun []testGroup, machineSemaphore *semaphore.Weighted, tests ...testing.InternalTest) []testing.InternalTest {
	groups := xslices.Map(testsToRun, func(group testGroup) testing.InternalTest {
		return testing.InternalTest{
			Name: group.Name,
			F: func(t *testing.T) {
				if group.Parallel {
					t.Parallel()
				}

				assert.NotEmpty(t, group.Name)

				t.Logf("[%s]:\n%s", group.Name, strings.TrimSpace(group.Description))

				if group.MachineClaim > 0 {
					t.Logf("attempting to acquire semaphore for %d machines", group.MachineClaim)

					if err := machineSemaphore.Acquire(ctx, int64(group.MachineClaim)); err != nil {
						t.Fatalf("failed to acquire machine semaphore: %s", err)
					}

					t.Logf("acquired semaphore for %d machines", group.MachineClaim)

					t.Cleanup(func() {
						t.Logf("releasing semaphore for %d machines", group.MachineClaim)

						machineSemaphore.Release(int64(group.MachineClaim))
					})
				}

				var testGroupFailed bool

				for _, elem := range group.Subtests {
					testGroupFailed = !t.Run(elem.Name, elem.F)
					if testGroupFailed {
						break
					}
				}

				if testGroupFailed && group.Finalizer != nil {
					t.Logf("running finalizer, as the test group failed")

					group.Finalizer(t)
				}
			},
		}
	})

	return append(groups, tests...)
}

//nolint:govet
type testGroup struct {
	Name         string
	Description  string
	Parallel     bool
	MachineClaim int
	Subtests     []subTest
	Finalizer    func(t *testing.T)
}

//nolint:govet
type subTest struct {
	Name string
	F    func(t *testing.T)
}

type subTestList []subTest

func subTests(items ...subTest) subTestList {
	return items
}

func (l subTestList) Append(items ...subTest) subTestList {
	return append(l, items...)
}
