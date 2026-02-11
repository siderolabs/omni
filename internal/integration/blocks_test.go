// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// AssertBlockClusterShouldBeReady is a reusable block of assertions that can be used to verify that a cluster is fully ready.
func AssertBlockClusterShouldBeReady(ctx context.Context, options *TestOptions, clusterName,
	expectedTalosVersion string,
) subTestList { //nolint:nolintlint,revive
	omniClient := options.omniClient

	return subTestList{
		{
			"ClusterMachinesShouldBeRunning",
			AssertClusterMachinesStage(ctx, omniClient.Omni().State(), clusterName, specs.ClusterMachineStatusSpec_RUNNING),
		},
		{
			"ClusterMachinesShouldBeReady",
			AssertClusterMachinesReady(ctx, omniClient.Omni().State(), clusterName),
		},
		{
			"MachinesStatusShouldBeNotAvailable",
			AssertMachineStatus(ctx, omniClient.Omni().State(), false, clusterName, map[string]string{
				omni.MachineStatusLabelConnected:       "",
				omni.MachineStatusLabelReportingEvents: "",
			},
				[]string{omni.MachineStatusLabelAvailable},
			),
		},
		{
			"ClusterShouldHaveStatusReady",
			AssertClusterStatusReady(ctx, omniClient.Omni().State(), clusterName),
		},
		{
			"ClusterLoadBalancerShouldBeReady",
			AssertClusterLoadBalancerReady(ctx, omniClient.Omni().State(), clusterName),
		},
		{
			"EtcdMembersShouldMatchOmniResources",
			AssertEtcdMembershipMatchesOmniResources(ctx, options, clusterName),
		},
		{
			"TalosMembersShouldMatchOmniResources",
			AssertTalosMembersMatchOmni(ctx, options, clusterName),
		},
		{
			"TalosVersionShouldMatchExpected",
			AssertTalosVersion(ctx, options, clusterName, expectedTalosVersion),
		},
	}
}

// AssertBlockProxyAPIAccessShouldWork is a reusable block of assertions that can be used to verify that Omni API proxies work.
func AssertBlockProxyAPIAccessShouldWork(ctx context.Context, options *TestOptions, clusterName string) []subTest { //nolint:nolintlint,revive
	return []subTest{
		{
			"ClusterKubernetesAPIShouldBeAccessibleViaOmni",
			AssertKubernetesAPIAccessViaOmni(ctx, options.omniClient, clusterName, true, 5*time.Minute),
		},
		{
			"ClusterTalosAPIShouldBeAccessibleViaOmni",
			AssertTalosAPIAccessViaOmni(ctx, options, clusterName),
		},
	}
}

// AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady is a reusable block of assertions that can be used to verify
// that a cluster is fully ready and that Omni API proxies work, and Kubernetes version is correct, and Kubernetes usage
// metrics were collected.
//
// This block is a bit slower than TestsBlockClusterShouldBeReady, because it also verifies Kubernetes version.
func AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
	ctx context.Context, options *TestOptions,
	clusterName, expectedTalosVersion, expectedKubernetesVersion string,
) []subTest { //nolint:nolintlint,revive
	omniState := options.omniClient.Omni().State()
	return AssertBlockClusterShouldBeReady(ctx, options, clusterName, expectedTalosVersion).
		Append(AssertBlockProxyAPIAccessShouldWork(ctx, options, clusterName)...).
		Append(
			subTest{
				"ClusterKubernetesVersionShouldBeCorrect",
				AssertClusterKubernetesVersion(ctx, omniState, clusterName, expectedKubernetesVersion),
			},
			subTest{
				"ClusterBootstrapManifestsShouldBeInSync",
				AssertClusterBootstrapManifestStatus(ctx, omniState, clusterName),
			},
		).
		Append(
			subTest{
				"ClusterKubernetesUsageShouldBeCorrect",
				AssertClusterKubernetesUsage(ctx, omniState, clusterName),
			},
		)
}

// AssertBlockRestoreEtcdFromLatestBackup is a reusable block of assertions that can be used to verify that a
// cluster's control plane can be broken, destroyed and then restored from an etcd backup.
func AssertBlockRestoreEtcdFromLatestBackup(ctx context.Context, testOptions *TestOptions,
	options Options, controlPlaneNodeCount int, clusterName, assertDeploymentNS, assertDeploymentName string,
) subTestList { //nolint:nolintlint,revive
	omniClient := testOptions.omniClient

	return subTestList{
		subTest{
			"ControlPlaneShouldBeBrokenThenDestroyed",
			AssertBreakAndDestroyControlPlane(ctx, omniClient.Omni().State(), clusterName, options),
		},
		subTest{
			"ControlPlaneShouldBeRestoredFromBackup",
			AssertControlPlaneCanBeRestoredFromBackup(ctx, omniClient.Omni().State(), clusterName),
		},
		subTest{
			"ControlPlaneShouldBeScaledUp",
			ScaleClusterUp(ctx, omniClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  controlPlaneNodeCount,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		},
	}.Append(
		subTest{
			"ClusterShouldHaveStatusReady",
			AssertClusterStatusReady(ctx, omniClient.Omni().State(), clusterName),
		},
		subTest{
			"ClusterLoadBalancerShouldBeReady",
			AssertClusterLoadBalancerReady(ctx, omniClient.Omni().State(), clusterName),
		},
		subTest{
			"EtcdMembersShouldMatchOmniResources",
			AssertEtcdMembershipMatchesOmniResources(ctx, testOptions, clusterName),
		},
	).Append(
		subTest{
			"KubernetesAPIShouldBeAccessible",
			AssertKubernetesAPIAccessViaOmni(ctx, omniClient, clusterName, false, 300*time.Second),
		},
		subTest{
			"ClusterNodesShouldBeInDesiredState",
			AssertKubernetesNodesState(ctx, omniClient, clusterName),
		},
		subTest{
			"KubeletShouldBeRestartedOnWorkers",
			AssertTalosServiceIsRestarted(ctx, testOptions, clusterName, "kubelet", resource.LabelExists(omni.LabelWorkerRole)),
		},
		subTest{
			"KubernetesDeploymentShouldHaveRunningPods",
			AssertKubernetesDeploymentHasRunningPods(ctx, omniClient.Management(), clusterName, assertDeploymentNS, assertDeploymentName),
		},
	).Append(
		AssertBlockKubernetesDeploymentCreateAndRunning(ctx, omniClient.Management(), clusterName, assertDeploymentNS, assertDeploymentName+"-after-restore")...,
	)
}

// AssertBlockCreateClusterFromEtcdBackup is a reusable block of assertions that can be used to verify that a
// new cluster can be created from another cluster's etcd backup.
func AssertBlockCreateClusterFromEtcdBackup(ctx context.Context, testOptions *TestOptions, options Options,
	sourceClusterName, newClusterName, assertDeploymentNS, assertDeploymentName string,
) subTestList { //nolint:nolintlint,revive
	omniClient := testOptions.omniClient

	return subTestList{
		subTest{
			"ClusterShouldBeCreatedFromEtcdBackup",
			CreateCluster(ctx, testOptions, ClusterOptions{
				Name:          newClusterName,
				ControlPlanes: 1,
				Workers:       1,

				RestoreFromEtcdBackupClusterID: sourceClusterName,

				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		},
	}.Append(
		subTest{
			"ClusterShouldHaveStatusReady",
			AssertClusterStatusReady(ctx, omniClient.Omni().State(), newClusterName),
		},
		subTest{
			"ClusterLoadBalancerShouldBeReady",
			AssertClusterLoadBalancerReady(ctx, omniClient.Omni().State(), newClusterName),
		},
		subTest{
			"EtcdMembersShouldMatchOmniResources",
			AssertEtcdMembershipMatchesOmniResources(ctx, testOptions, newClusterName),
		},
	).Append(
		subTest{
			"KubernetesAPIShouldBeAccessible",
			AssertKubernetesAPIAccessViaOmni(ctx, omniClient, newClusterName, false, 300*time.Second),
		},
		subTest{
			"ClusterNodesShouldBeInDesiredState",
			AssertKubernetesNodesState(ctx, omniClient, newClusterName),
		},
		subTest{
			"KubernetesDeploymentShouldHaveRunningPods",
			AssertKubernetesDeploymentHasRunningPods(ctx, omniClient.Management(), newClusterName, assertDeploymentNS, assertDeploymentName),
		},
	).Append(
		AssertBlockKubernetesDeploymentCreateAndRunning(ctx, omniClient.Management(), newClusterName, assertDeploymentNS, assertDeploymentName+"-after-restore")...,
	)
}

// AssertBlockKubernetesDeploymentCreateAndRunning is a reusable block of assertions that can be used to verify that a
// Kubernetes deployment is created and has running pods.
func AssertBlockKubernetesDeploymentCreateAndRunning(ctx context.Context, managementClient *management.Client, clusterName, ns, name string) []subTest { //nolint:nolintlint,revive
	return []subTest{
		{
			"KubernetesDeploymentShouldBeCreated",
			AssertKubernetesDeploymentIsCreated(ctx, managementClient, clusterName, ns, name),
		},
		{
			"KubernetesDeploymentShouldHaveRunningPods",
			AssertKubernetesDeploymentHasRunningPods(ctx, managementClient, clusterName, ns, name),
		},
	}
}

// AssertClusterCreateAndReady is a reusable group of tests that can be used to verify that a cluster is created and ready.
func AssertClusterCreateAndReady(ctx context.Context, testOptions *TestOptions, options ClusterOptions) []subTest { //nolint:nolintlint,revive
	omniClient := testOptions.omniClient

	return subTests(
		subTest{
			"ClusterShouldBeCreated",
			CreateCluster(ctx, testOptions, options),
		},
	).Append(
		AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(ctx, testOptions, options.Name, options.MachineOptions.TalosVersion, options.MachineOptions.KubernetesVersion)...,
	).Append(
		subTest{
			"AssertSupportBundleContents",
			AssertSupportBundleContents(ctx, omniClient, options.Name),
		},
	).Append(
		subTest{
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(ctx, omniClient.Omni().State(), options.Name, options.InfraProvider != "", false),
		},
	)
}
