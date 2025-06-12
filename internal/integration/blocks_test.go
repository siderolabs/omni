// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// AssertBlockClusterShouldBeReady is a reusable block of assertions that can be used to verify that a cluster is fully ready.
func AssertBlockClusterShouldBeReady(ctx context.Context, rootClient *client.Client, clusterName,
	expectedTalosVersion string,
) subTestList { //nolint:nolintlint,revive
	return subTestList{
		{
			"ClusterMachinesShouldBeRunning",
			AssertClusterMachinesStage(ctx, rootClient.Omni().State(), clusterName, specs.ClusterMachineStatusSpec_RUNNING),
		},
		{
			"ClusterMachinesShouldBeReady",
			AssertClusterMachinesReady(ctx, rootClient.Omni().State(), clusterName),
		},
		{
			"MachinesStatusShouldBeNotAvailable",
			AssertMachineStatus(ctx, rootClient.Omni().State(), false, clusterName, map[string]string{
				omni.MachineStatusLabelConnected:       "",
				omni.MachineStatusLabelReportingEvents: "",
			},
				[]string{omni.MachineStatusLabelAvailable},
			),
		},
		{
			"ClusterShouldHaveStatusReady",
			AssertClusterStatusReady(ctx, rootClient.Omni().State(), clusterName),
		},
		{
			"ClusterLoadBalancerShouldBeReady",
			AssertClusterLoadBalancerReady(ctx, rootClient.Omni().State(), clusterName),
		},
		{
			"EtcdMembersShouldMatchOmniResources",
			AssertEtcdMembershipMatchesOmniResources(ctx, rootClient, clusterName),
		},
		{
			"TalosMembersShouldMatchOmniResources",
			AssertTalosMembersMatchOmni(ctx, rootClient, clusterName),
		},
		{
			"TalosVersionShouldMatchExpected",
			AssertTalosVersion(ctx, rootClient, clusterName, expectedTalosVersion),
		},
	}
}

// AssertBlockProxyAPIAccessShouldWork is a reusable block of assertions that can be used to verify that Omni API proxies work.
func AssertBlockProxyAPIAccessShouldWork(ctx context.Context, rootClient *client.Client, clusterName string) []subTest { //nolint:nolintlint,revive
	return []subTest{
		{
			"ClusterKubernetesAPIShouldBeAccessibleViaOmni",
			AssertKubernetesAPIAccessViaOmni(ctx, rootClient, clusterName, true, 5*time.Minute),
		},
		{
			"ClusterTalosAPIShouldBeAccessibleViaOmni",
			AssertTalosAPIAccessViaOmni(ctx, rootClient, clusterName),
		},
	}
}

// AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady is a reusable block of assertions that can be used to verify
// that a cluster is fully ready and that Omni API proxies work, and Kubernetes version is correct, and Kubernetes usage
// metrics were collected.
//
// This block is a bit slower than TestsBlockClusterShouldBeReady, because it also verifies Kubernetes version.
func AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
	ctx context.Context, rootClient *client.Client,
	clusterName, expectedTalosVersion, expectedKubernetesVersion string,
) []subTest { //nolint:nolintlint,revive
	return AssertBlockClusterShouldBeReady(ctx, rootClient, clusterName, expectedTalosVersion).
		Append(AssertBlockProxyAPIAccessShouldWork(ctx, rootClient, clusterName)...).
		Append(
			subTest{
				"ClusterKubernetesVersionShouldBeCorrect",
				AssertClusterKubernetesVersion(ctx, rootClient.Omni().State(), clusterName, expectedKubernetesVersion),
			},
			subTest{
				"ClusterBootstrapManifestsShouldBeInSync",
				AssertClusterBootstrapManifestStatus(ctx, rootClient.Omni().State(), clusterName),
			},
		).
		Append(
			subTest{
				"ClusterKubernetesUsageShouldBeCorrect",
				AssertClusterKubernetesUsage(ctx, rootClient.Omni().State(), clusterName),
			},
		)
}

// AssertBlockRestoreEtcdFromLatestBackup is a reusable block of assertions that can be used to verify that a
// cluster's control plane can be broken, destroyed and then restored from an etcd backup.
func AssertBlockRestoreEtcdFromLatestBackup(ctx context.Context, rootClient *client.Client,
	options Options, controlPlaneNodeCount int, clusterName, assertDeploymentNS, assertDeploymentName string,
) subTestList { //nolint:nolintlint,revive
	return subTestList{
		subTest{
			"ControlPlaneShouldBeBrokenThenDestroyed",
			AssertBreakAndDestroyControlPlane(ctx, rootClient.Omni().State(), clusterName, options),
		},
		subTest{
			"ControlPlaneShouldBeRestoredFromBackup",
			AssertControlPlaneCanBeRestoredFromBackup(ctx, rootClient.Omni().State(), clusterName),
		},
		subTest{
			"ControlPlaneShouldBeScaledUp",
			ScaleClusterUp(ctx, rootClient.Omni().State(), ClusterOptions{
				Name:           clusterName,
				ControlPlanes:  controlPlaneNodeCount,
				MachineOptions: options.MachineOptions,
				ScalingTimeout: options.ScalingTimeout,
			}),
		},
	}.Append(
		subTest{
			"ClusterShouldHaveStatusReady",
			AssertClusterStatusReady(ctx, rootClient.Omni().State(), clusterName),
		},
		subTest{
			"ClusterLoadBalancerShouldBeReady",
			AssertClusterLoadBalancerReady(ctx, rootClient.Omni().State(), clusterName),
		},
		subTest{
			"EtcdMembersShouldMatchOmniResources",
			AssertEtcdMembershipMatchesOmniResources(ctx, rootClient, clusterName),
		},
	).Append(
		subTest{
			"KubernetesAPIShouldBeAccessible",
			AssertKubernetesAPIAccessViaOmni(ctx, rootClient, clusterName, false, 300*time.Second),
		},
		subTest{
			"ClusterNodesShouldBeInDesiredState",
			AssertKubernetesNodesState(ctx, rootClient, clusterName),
		},
		subTest{
			"KubeletShouldBeRestartedOnWorkers",
			AssertTalosServiceIsRestarted(ctx, rootClient, clusterName, "kubelet", resource.LabelExists(omni.LabelWorkerRole)),
		},
		subTest{
			"KubernetesDeploymentShouldHaveRunningPods",
			AssertKubernetesDeploymentHasRunningPods(ctx, rootClient.Management(), clusterName, assertDeploymentNS, assertDeploymentName),
		},
	).Append(
		AssertBlockKubernetesDeploymentCreateAndRunning(ctx, rootClient.Management(), clusterName, assertDeploymentNS, assertDeploymentName+"-after-restore")...,
	)
}

// AssertBlockCreateClusterFromEtcdBackup is a reusable block of assertions that can be used to verify that a
// new cluster can be created from another cluster's etcd backup.
func AssertBlockCreateClusterFromEtcdBackup(ctx context.Context, rootClient *client.Client, options Options,
	sourceClusterName, newClusterName, assertDeploymentNS, assertDeploymentName string,
) subTestList { //nolint:nolintlint,revive
	return subTestList{
		subTest{
			"ClusterShouldBeCreatedFromEtcdBackup",
			CreateCluster(ctx, rootClient, ClusterOptions{
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
			AssertClusterStatusReady(ctx, rootClient.Omni().State(), newClusterName),
		},
		subTest{
			"ClusterLoadBalancerShouldBeReady",
			AssertClusterLoadBalancerReady(ctx, rootClient.Omni().State(), newClusterName),
		},
		subTest{
			"EtcdMembersShouldMatchOmniResources",
			AssertEtcdMembershipMatchesOmniResources(ctx, rootClient, newClusterName),
		},
	).Append(
		subTest{
			"KubernetesAPIShouldBeAccessible",
			AssertKubernetesAPIAccessViaOmni(ctx, rootClient, newClusterName, false, 300*time.Second),
		},
		subTest{
			"ClusterNodesShouldBeInDesiredState",
			AssertKubernetesNodesState(ctx, rootClient, newClusterName),
		},
		subTest{
			"KubernetesDeploymentShouldHaveRunningPods",
			AssertKubernetesDeploymentHasRunningPods(ctx, rootClient.Management(), newClusterName, assertDeploymentNS, assertDeploymentName),
		},
	).Append(
		AssertBlockKubernetesDeploymentCreateAndRunning(ctx, rootClient.Management(), newClusterName, assertDeploymentNS, assertDeploymentName+"-after-restore")...,
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
func AssertClusterCreateAndReady(
	ctx context.Context,
	rootClient *client.Client,
	name string,
	options ClusterOptions,
	testOutputDir string,
	doSaveSupportBundle bool,
) []subTest { //nolint:nolintlint,revive
	clusterName := "integration-" + name
	options.Name = clusterName

	return subTests(
		subTest{
			"ClusterShouldBeCreated",
			CreateCluster(ctx, rootClient, options),
		},
	).Append(
		AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(ctx, rootClient, clusterName, options.MachineOptions.TalosVersion, options.MachineOptions.KubernetesVersion)...,
	).Append(
		subTest{
			"AssertSupportBundleContents",
			AssertSupportBundleContents(ctx, rootClient, clusterName),
		},
	).Append(
		subTest{
			"ClusterShouldBeDestroyed",
			AssertDestroyCluster(ctx, rootClient, clusterName, testOutputDir, options.InfraProvider != "", false, doSaveSupportBundle),
		},
	)
}
