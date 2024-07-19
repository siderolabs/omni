// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	managementpb "github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// AssertKubernetesAPIAccessViaOmni verifies that cluster kubeconfig works.
//
//nolint:gocognit
func AssertKubernetesAPIAccessViaOmni(testCtx context.Context, rootClient *client.Client, clusterName string, assertAllNodesReady bool, timeout time.Duration) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, timeout)
		defer cancel()

		var (
			k8sClient = getKubernetesClient(ctx, t, rootClient.Management(), clusterName)
			k8sNodes  *corev1.NodeList
			err       error
		)

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			require.NoError(collect, ctx.Err())

			k8sNodes, err = k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if !assert.NoError(collect, err) {
				return
			}

			nodeNamesInK8s := make([]string, 0, len(k8sNodes.Items))

			for _, k8sNode := range k8sNodes.Items {
				nodeNamesInK8s = append(nodeNamesInK8s, k8sNode.Name)
			}

			var identityList safe.List[*omni.ClusterMachineIdentity]

			identityList, err = safe.StateListAll[*omni.ClusterMachineIdentity](ctx, rootClient.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
			require.NoError(collect, err)

			nodeNamesInIdentities := make([]string, 0, identityList.Len())

			identityList.ForEach(func(identity *omni.ClusterMachineIdentity) {
				nodeNamesInIdentities = append(nodeNamesInIdentities, identity.TypedSpec().Value.GetNodename())
			})

			assert.ElementsMatch(collect, nodeNamesInK8s, nodeNamesInIdentities, "Node names in Kubernetes (list A) and Omni machine identities (list B) do not match")
		}, 3*time.Minute, 5*time.Second)

		isNodeReady := func(node corev1.Node) bool {
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady {
					return true
				}
			}

			return false
		}

		for _, k8sNode := range k8sNodes.Items {
			ready := isNodeReady(k8sNode)

			if assertAllNodesReady {
				assert.True(t, ready, "node %q is not ready", k8sNode.Name)
			}

			if ready {
				var (
					label string
					ok    bool
				)

				assert.NoError(t, retry.Constant(time.Second*30).RetryWithContext(ctx, func(ctx context.Context) error {
					label, ok = k8sNode.Labels[nodeLabel]
					if !ok {
						var n *corev1.Node

						n, err = k8sClient.CoreV1().Nodes().Get(ctx, k8sNode.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						k8sNode = *n

						return retry.ExpectedErrorf("the label %s is not set", nodeLabel)
					}

					return nil
				}))

				assert.True(t, ok)
				assert.NotEmpty(t, label)
			}
		}
	}
}

// AssertKubernetesUpgradeFlow verifies Kubernetes upgrade flow.
//
// TODO: machine set locking should be in a separate test.
func AssertKubernetesUpgradeFlow(testCtx context.Context, st state.State, managementClient *management.Client, clusterName string, kubernetesVersion string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 15*time.Minute)
		defer cancel()

		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.KubernetesUpgradeStatus, assert *assert.Assertions) {
			// there is an upgrade path
			assert.Contains(r.TypedSpec().Value.UpgradeVersions, kubernetesVersion, resourceDetails(r))
		})

		t.Logf("running pre-checks for upgrade of %q to %q", clusterName, kubernetesVersion)

		require.NoError(t, managementClient.WithCluster(clusterName).KubernetesUpgradePreChecks(ctx, kubernetesVersion))

		t.Logf("upgrading cluster %q to %q", clusterName, kubernetesVersion)

		machineSetNodes, err := safe.StateListAll[*omni.MachineSetNode](ctx, st, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelMachineSet, omni.WorkersResourceID(clusterName)),
		))
		require.NoError(t, err)
		require.True(t, machineSetNodes.Len() > 0)

		// lock a machine in the machine set
		_, err = safe.StateUpdateWithConflicts(ctx, st, machineSetNodes.Get(0).Metadata(), func(res *omni.MachineSetNode) error {
			res.Metadata().Annotations().Set(omni.MachineLocked, "")

			return nil
		})
		require.NoError(t, err)

		// trigger an upgrade
		_, err = safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.TypedSpec().Value.KubernetesVersion = kubernetesVersion

			return nil
		})
		require.NoError(t, err)

		// upgrade should start
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.KubernetesUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.KubernetesUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.NotEmpty(specs.KubernetesUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Step, resourceDetails(r))
			assert.NotEmpty(specs.KubernetesUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Status, resourceDetails(r))
		})

		t.Log("waiting until upgrade reaches the locked machine")

		// upgrade should start
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.KubernetesUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.KubernetesUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.NotEmpty(specs.KubernetesUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Step, resourceDetails(r))
			assert.Contains(r.TypedSpec().Value.Status, "locked", resourceDetails(r))
		})

		// lock a machine in the machine set
		_, err = safe.StateUpdateWithConflicts(ctx, st, machineSetNodes.Get(0).Metadata(), func(res *omni.MachineSetNode) error {
			res.Metadata().Annotations().Delete(omni.MachineLocked)

			return nil
		})
		require.NoError(t, err)

		t.Log("upgrade is going")

		// upgrade should finish successfully
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.KubernetesUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.KubernetesUpgradeStatusSpec_Done, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.Equal(kubernetesVersion, r.TypedSpec().Value.LastUpgradeVersion, resourceDetails(r))
			assert.Empty(r.TypedSpec().Value.Step, resourceDetails(r))
		})

		// validate that all components are on the expected version
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.KubernetesStatus, assert *assert.Assertions) {
			for _, node := range r.TypedSpec().Value.Nodes {
				assert.Equal(kubernetesVersion, node.KubeletVersion, resourceDetails(r))
				assert.True(node.Ready, resourceDetails(r))
			}

			for _, nodePods := range r.TypedSpec().Value.StaticPods {
				for _, pod := range nodePods.StaticPods {
					assert.Equal(kubernetesVersion, pod.Version, resourceDetails(r))
					assert.True(pod.Ready, resourceDetails(r))
				}
			}
		})

		KubernetesBootstrapManifestSync(ctx, managementClient, clusterName)(t)
	}
}

// KubernetesBootstrapManifestSync syncs kubernetes bootstrap manifests.
func KubernetesBootstrapManifestSync(testCtx context.Context, managementClient *management.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 15*time.Minute)
		defer cancel()

		t.Logf("running bootstrap manifest sync for %q", clusterName)

		syncHandler := func(result *managementpb.KubernetesSyncManifestResponse) error {
			switch result.ResponseType { //nolint:exhaustive
			case managementpb.KubernetesSyncManifestResponse_MANIFEST:
				if result.Skipped {
					return nil
				}

				t.Logf("syncing manifest %q:\n%s\n", result.Path, result.Diff)
			case managementpb.KubernetesSyncManifestResponse_ROLLOUT:
				t.Logf("waiting for rolling out of %q", result.Path)
			}

			return nil
		}

		require.NoError(t, managementClient.WithCluster(clusterName).KubernetesSyncManifests(ctx, false, syncHandler))
	}
}

// AssertKubernetesUpgradeIsRevertible verifies reverting a failed Kubernetes upgrade.
func AssertKubernetesUpgradeIsRevertible(testCtx context.Context, st state.State, clusterName string, currentKubernetesVersion string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 5*time.Minute)
		defer cancel()

		badKubernetesVersion := currentKubernetesVersion + "-bad"

		t.Logf("attempting an upgrade of cluster %q to %q", clusterName, badKubernetesVersion)

		// trigger an upgrade to a bad version
		_, err := safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.Metadata().Annotations().Set(constants.DisableValidation, "")

			cluster.TypedSpec().Value.KubernetesVersion = badKubernetesVersion

			return nil
		})
		require.NoError(t, err)

		// upgrade should start
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.KubernetesUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.KubernetesUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.NotEmpty(specs.KubernetesUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Step, resourceDetails(r))
			assert.NotEmpty(specs.KubernetesUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Status, resourceDetails(r))
			assert.Equal(currentKubernetesVersion, r.TypedSpec().Value.LastUpgradeVersion, resourceDetails(r))
		})

		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.ImagePullStatus, assert *assert.Assertions) {
			assert.Contains(r.TypedSpec().Value.LastProcessedError, "-bad")
		})

		t.Log("revert an upgrade")

		_, err = safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.TypedSpec().Value.KubernetesVersion = currentKubernetesVersion

			return nil
		})
		require.NoError(t, err)

		// upgrade should be reverted
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.KubernetesUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.KubernetesUpgradeStatusSpec_Done, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.Equal(currentKubernetesVersion, r.TypedSpec().Value.LastUpgradeVersion, resourceDetails(r))
			assert.Empty(r.TypedSpec().Value.Step, resourceDetails(r))
		})

		// validate that all components are on the expected version
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.KubernetesStatus, assert *assert.Assertions) {
			for _, node := range r.TypedSpec().Value.Nodes {
				assert.Equal(currentKubernetesVersion, node.KubeletVersion, resourceDetails(r))
				assert.True(node.Ready, resourceDetails(r))
			}

			for _, nodePods := range r.TypedSpec().Value.StaticPods {
				for _, pod := range nodePods.StaticPods {
					assert.Equal(currentKubernetesVersion, pod.Version, resourceDetails(r))
					assert.True(pod.Ready, resourceDetails(r))
				}
			}
		})
	}
}

// AssertKubernetesDeploymentIsCreated verifies that a test deployment either already exists or otherwise gets created.
func AssertKubernetesDeploymentIsCreated(testCtx context.Context, managementClient *management.Client, clusterName, ns, name string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
		defer cancel()

		kubeClient := getKubernetesClient(ctx, t, managementClient, clusterName)

		deployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": name,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": name,
						},
					},
					Spec: corev1.PodSpec{
						TerminationGracePeriodSeconds: pointer.To(int64(0)),
						Containers: []corev1.Container{{
							Name:  name,
							Image: "busybox:1",
							// sleep forever
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'hello'; sleep 1; done",
							},
						}},
					},
				},
			},
		}

		_, err := kubeClient.AppsV1().Deployments(ns).Create(ctx, &deployment, metav1.CreateOptions{})
		if !kubeerrors.IsAlreadyExists(err) {
			require.NoError(t, err)
		}
	}
}

// AssertKubernetesSecretIsCreated verifies that a test secret either already exists or otherwise gets created.
func AssertKubernetesSecretIsCreated(testCtx context.Context, managementClient *management.Client, clusterName, ns, name, testValue string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
		defer cancel()

		kubeClient := getKubernetesClient(ctx, t, managementClient, clusterName)

		valBase64 := base64.StdEncoding.EncodeToString([]byte(testValue))

		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Data: map[string][]byte{
				"test-key": []byte(valBase64),
			},
		}

		_, err := kubeClient.CoreV1().Secrets(ns).Create(ctx, &secret, metav1.CreateOptions{})
		require.NoError(t, err, "failed to create secret")
	}
}

// AssertKubernetesSecretHasValue verifies that a test secret has a specific value.
func AssertKubernetesSecretHasValue(testCtx context.Context, managementClient *management.Client, clusterName, ns, name, expectedValue string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
		defer cancel()

		kubeClient := getKubernetesClient(ctx, t, managementClient, clusterName)

		secret, err := kubeClient.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		require.NoError(t, err, "failed to get secret")

		actualValBase64, ok := secret.Data["test-key"]
		require.True(t, ok, "secret does not have test-key")

		expectedValBase64 := base64.StdEncoding.EncodeToString([]byte(expectedValue))

		assert.Equal(t, expectedValBase64, string(actualValBase64))
	}
}

// AssertKubernetesNodesState asserts two things for the given new cluster name:
// 1. Omni cluster machines match exactly the Kubernetes nodes that are in Ready state
// 2. All the extra (stale) Kubernetes nodes are in NotReady state
//
// This assertion is useful to assert the expected nodes state when a cluster is created from an etcd backup.
func AssertKubernetesNodesState(ctx context.Context, rootClient *client.Client, newClusterName string) func(t *testing.T) {
	return func(t *testing.T) {
		identityList, err := safe.StateListAll[*omni.ClusterMachineIdentity](ctx, rootClient.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, newClusterName)))
		require.NoError(t, err)

		expectedReadyNodeNames, err := safe.Map(identityList, func(cm *omni.ClusterMachineIdentity) (string, error) {
			return cm.TypedSpec().Value.GetNodename(), nil
		})
		require.NoError(t, err)

		expectedReadyNodeNameSet := xslices.ToSet(expectedReadyNodeNames)

		nodeIsReady := func(node corev1.Node) bool {
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					return true
				}
			}

			return false
		}

		kubeClient := getKubernetesClient(ctx, t, rootClient.Management(), newClusterName)

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			kubernetesNodes, listErr := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			require.NoError(collect, listErr)

			validNotReadyNodes := make([]string, 0, len(expectedReadyNodeNames))
			extraReadyNodes := make([]string, 0, len(expectedReadyNodeNames))

			for _, kubernetesNode := range kubernetesNodes.Items {
				ready := nodeIsReady(kubernetesNode)
				_, valid := expectedReadyNodeNameSet[kubernetesNode.Name]

				if !valid && ready {
					extraReadyNodes = append(extraReadyNodes, kubernetesNode.Name)
				} else if valid && !ready {
					validNotReadyNodes = append(validNotReadyNodes, kubernetesNode.Name)
				}
			}

			if !assert.Empty(collect, extraReadyNodes, "Kubernetes has extra Ready nodes") {
				t.Logf("extra ready nodes: %q", extraReadyNodes)
			}

			if !assert.Empty(collect, validNotReadyNodes, "Kubernetes has valid NotReady nodes") {
				t.Logf("valid but not ready nodes: %q", validNotReadyNodes)
			}
		}, 2*time.Minute, 1*time.Second, "Kubernetes nodes should be in desired state")
	}
}

// AssertKubernetesDeploymentHasRunningPods verifies that a deployment has running pods.
func AssertKubernetesDeploymentHasRunningPods(ctx context.Context, managementClient *management.Client, clusterName, ns, name string) TestFunc {
	return func(t *testing.T) {
		deps := getKubernetesClient(ctx, t, managementClient, clusterName).AppsV1().Deployments(ns)

		// restart the deployment, in case the pod is scheduled on a NotReady node (a node that is no longer valid, which was restored from an etcd backup)
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			deployment, err := deps.Get(ctx, name, metav1.GetOptions{})
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				require.NoError(collect, err)
			}

			if !assert.NoError(collect, err) {
				t.Logf("failed to get deployment %q/%q: %q", ns, name, err)

				return
			}

			if deployment.Spec.Template.ObjectMeta.Annotations == nil {
				deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
			}

			deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

			if _, err = deps.Update(ctx, deployment, metav1.UpdateOptions{}); !assert.NoError(collect, err) {
				t.Logf("failed to update deployment %q/%q: %q", ns, name, err)
			}
		}, 2*time.Minute, 1*time.Second)

		// assert that deployment has a running (Ready) pod
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			deployment, err := deps.Get(ctx, name, metav1.GetOptions{})
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				require.NoError(collect, err)
			}

			if !assert.NoError(collect, err) {
				t.Logf("failed to get deployment %q/%q: %q", ns, name, err)

				return
			}

			if !assert.Greater(collect, deployment.Status.ReadyReplicas, int32(0)) {
				t.Logf("deployment %q/%q has no ready replicas", ns, name)
			}
		}, 2*time.Minute, 1*time.Second)
	}
}

func getKubernetesClient(ctx context.Context, t *testing.T, managementClient *management.Client, clusterName string) *kubernetes.Clientset {
	// use service account kubeconfig to bypass oidc flow
	kubeconfigBytes, err := managementClient.WithCluster(clusterName).Kubeconfig(ctx,
		management.WithServiceAccount(24*time.Hour, "integration-test", constants.DefaultAccessGroup),
	)
	require.NoError(t, err)

	kubeconfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(kubeconfigBytes)
	})
	require.NoError(t, err)

	cli, err := kubernetes.NewForConfig(kubeconfig)
	require.NoError(t, err)

	return cli
}
