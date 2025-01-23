// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// TestBlockClusterShouldBeReady is a reusable block of assertions that can be used to verify that a cluster is fully ready.
func TestBlockClusterShouldBeReady(ctx context.Context, rootClient *client.Client, clusterName, expectedTalosVersion string, talosAPIKeyPrepare TalosAPIKeyPrepareFunc) subTestList { //nolint:revive
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
			AssertEtcdMembershipMatchesOmniResources(ctx, rootClient, clusterName, talosAPIKeyPrepare),
		},
		{
			"TalosMembersShouldMatchOmniResources",
			AssertTalosMembersMatchOmni(ctx, rootClient, clusterName, talosAPIKeyPrepare),
		},
		{
			"TalosVersionShouldMatchExpected",
			AssertTalosVersion(ctx, rootClient, clusterName, expectedTalosVersion, talosAPIKeyPrepare),
		},
	}
}

// TestBlockProxyAPIAccessShouldWork is a reusable block of assertions that can be used to verify that Omni API proxies work.
func TestBlockProxyAPIAccessShouldWork(ctx context.Context, rootClient *client.Client, clusterName string, talosAPIKeyPrepare TalosAPIKeyPrepareFunc) []subTest { //nolint:revive
	return []subTest{
		{
			"ClusterKubernetesAPIShouldBeAccessibleViaOmni",
			AssertKubernetesAPIAccessViaOmni(ctx, rootClient, clusterName, true, 5*time.Minute),
		},
		{
			"ClusterTalosAPIShouldBeAccessibleViaOmni",
			AssertTalosAPIAccessViaOmni(ctx, rootClient, clusterName, talosAPIKeyPrepare),
		},
	}
}

// TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady is a reusable block of assertions that can be used to verify
// that a cluster is fully ready and that Omni API proxies work, and Kubernetes version is correct, and Kubernetes usage
// metrics were collected.
//
// This block is a bit slower than TestBlockClusterShouldBeReady, because it also verifies Kubernetes version.
func TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(
	ctx context.Context, rootClient *client.Client,
	clusterName, expectedTalosVersion, expectedKubernetesVersion string,
	talosAPIKeyPrepare TalosAPIKeyPrepareFunc,
) []subTest { //nolint:revive
	return TestBlockClusterShouldBeReady(ctx, rootClient, clusterName, expectedTalosVersion, talosAPIKeyPrepare).
		Append(TestBlockProxyAPIAccessShouldWork(ctx, rootClient, clusterName, talosAPIKeyPrepare)...).
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

// TestBlockRestoreEtcdFromLatestBackup is a reusable block of assertions that can be used to verify that a
// cluster's control plane can be broken, destroyed and then restored from an etcd backup.
func TestBlockRestoreEtcdFromLatestBackup(ctx context.Context, rootClient *client.Client, talosAPIKeyPrepare TalosAPIKeyPrepareFunc,
	options Options, controlPlaneNodeCount int, clusterName, assertDeploymentNS, assertDeploymentName string,
) subTestList { //nolint:revive
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
			AssertEtcdMembershipMatchesOmniResources(ctx, rootClient, clusterName, talosAPIKeyPrepare),
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
			AssertTalosServiceIsRestarted(ctx, rootClient, clusterName, talosAPIKeyPrepare, "kubelet", resource.LabelExists(omni.LabelWorkerRole)),
		},
		subTest{
			"KubernetesDeploymentShouldHaveRunningPods",
			AssertKubernetesDeploymentHasRunningPods(ctx, rootClient.Management(), clusterName, assertDeploymentNS, assertDeploymentName),
		},
	).Append(
		TestBlockKubernetesDeploymentCreateAndRunning(ctx, rootClient.Management(), clusterName, assertDeploymentNS, assertDeploymentName+"-after-restore")...,
	)
}

// TestBlockCreateClusterFromEtcdBackup is a reusable block of assertions that can be used to verify that a
// new cluster can be created from another cluster's etcd backup.
func TestBlockCreateClusterFromEtcdBackup(ctx context.Context, rootClient *client.Client, talosAPIKeyPrepare TalosAPIKeyPrepareFunc, options Options,
	sourceClusterName, newClusterName, assertDeploymentNS, assertDeploymentName string,
) subTestList { //nolint:revive
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
			AssertEtcdMembershipMatchesOmniResources(ctx, rootClient, newClusterName, talosAPIKeyPrepare),
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
		TestBlockKubernetesDeploymentCreateAndRunning(ctx, rootClient.Management(), newClusterName, assertDeploymentNS, assertDeploymentName+"-after-restore")...,
	)
}

// TestBlockKubernetesDeploymentCreateAndRunning is a reusable block of assertions that can be used to verify that a
// Kubernetes deployment is created and has running pods.
func TestBlockKubernetesDeploymentCreateAndRunning(ctx context.Context, managementClient *management.Client, clusterName, ns, name string) []subTest { //nolint:revive
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

// TestGroupClusterCreateAndReady is a reusable group of tests that can be used to verify that a cluster is created and ready.
func TestGroupClusterCreateAndReady(
	ctx context.Context,
	rootClient *client.Client,
	talosAPIKeyPrepare TalosAPIKeyPrepareFunc,
	name, description string,
	options ClusterOptions,
) testGroup { //nolint:revive
	clusterName := "integration-" + name
	options.Name = clusterName

	return testGroup{
		Name:         strings.ToUpper(name[0:1]) + name[1:] + "Cluster",
		Description:  description,
		Parallel:     true,
		MachineClaim: options.ControlPlanes + options.Workers,
		Subtests: subTests(
			subTest{
				"ClusterShouldBeCreated",
				CreateCluster(ctx, rootClient, options),
			},
		).Append(
			TestBlockClusterAndTalosAPIAndKubernetesShouldBeReady(ctx, rootClient, clusterName, options.MachineOptions.TalosVersion, options.MachineOptions.KubernetesVersion, talosAPIKeyPrepare)...,
		).Append(
			subTest{
				"AssertSupportBundleContents",
				AssertSupportBundleContents(ctx, rootClient, clusterName),
			},
		).Append(
			subTest{
				"ClusterShouldBeDestroyed",
				AssertDestroyCluster(ctx, rootClient.Omni().State(), clusterName, options.InfraProvider != "", false),
			},
		),
		Finalizer: DestroyCluster(ctx, rootClient.Omni().State(), clusterName),
	}
}
