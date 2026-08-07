// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-kubernetes/kubernetes/nodedrain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	integrationkube "github.com/siderolabs/omni/internal/integration/kubernetes"
)

const (
	// drainWorkloadNamespace, drainWorkloadName and drainWorkloadSelector identify the workload whose
	// shutdown the drain has to wait for.
	drainWorkloadNamespace = "omni-drain-test"
	drainWorkloadName      = "slow-shutdown"
	drainWorkloadSelector  = "app=" + drainWorkloadName

	// drainWorkloadGracePeriod is the workload's terminationGracePeriodSeconds. It sits far above the
	// shutdown below, so a pod killed at the grace period instead of exiting on its own is unmistakable.
	drainWorkloadGracePeriod = 900 * time.Second

	// drainWorkloadShutdown is how long the workload keeps running after SIGTERM. It is past
	// nodedrain.DefaultDrainTimeout with enough margin that a slow sample can't blur the two, and far short
	// of drainWorkloadGracePeriod.
	drainWorkloadShutdown = 360 * time.Second

	// drainFailureMarker is the fragment lifecycle.Manager.Run wraps a failed drain with, as it reaches
	// ClusterMachineConfigStatus.LastConfigError.
	drainFailureMarker = "cordon/drain before reboot failed"

	// drainSampleInterval paces the observation loop that reconstructs the drain timeline.
	drainSampleInterval = 2 * time.Second
)

// testDrainTerminationGracePeriod verifies how a Talos upgrade treats a pod that takes longer to shut down
// than the drain is willing to wait.
//
// The pod's own terminationGracePeriodSeconds is honored (it is neither overridden nor cut short), and the
// node is not rebooted while the pod is still terminating. The drain itself, though, is capped at
// nodedrain.DefaultDrainTimeout, so it fails first and only succeeds on a later retry.
//
// The test is slow by construction (a deliberate six-minute shutdown per drained worker), which is why it
// runs as its own suite rather than inside an existing one.
func testDrainTerminationGracePeriod(t *testing.T, options *TestOptions) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
	defer cancel()

	options.claimMachines(t, 3)

	clusterName := "integration-drain-grace-period"
	omniState := options.omniClient.Omni().State()
	newTalosVersion := options.MachineOptions.TalosVersion

	machineOptions := MachineOptions{
		TalosVersion:      options.AnotherTalosVersion,
		KubernetesVersion: options.AnotherKubernetesVersion,
	}

	t.Run(
		"ClusterShouldBeCreated",
		CreateCluster(ctx, options, ClusterOptions{
			Name:          clusterName,
			ControlPlanes: 1,
			Workers:       2,

			MachineOptions: machineOptions,
			ScalingTimeout: options.ScalingTimeout,

			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
		}),
	)

	assertClusterAndAPIReady(t, clusterName, options,
		withTalosVersion(machineOptions.TalosVersion), withKubernetesVersion(machineOptions.KubernetesVersion))

	kubeCtx := integrationkube.WrapContext(ctx, t)
	kubeClient := integrationkube.GetClient(kubeCtx, t, options.omniClient.Management(), clusterName)

	applyDrainWorkload(ctx, t, omniState, clusterName)
	waitForDrainWorkload(kubeCtx, t, kubeClient)

	machineIDByNode := machineIDByNodeName(ctx, t, omniState, clusterName)

	recorder := newDrainRecorder()
	samplerCtx, stopSampler := context.WithCancel(ctx)
	sampler := recorder.run(samplerCtx, kubeClient, omniState, clusterName)

	defer func() {
		stopSampler()
		<-sampler
	}()

	t.Logf("upgrading cluster %q from %q to %q", clusterName, machineOptions.TalosVersion, newTalosVersion)

	_, err := safe.StateUpdateWithConflicts(ctx, omniState, omni.NewCluster(clusterName).Metadata(), func(cluster *omni.Cluster) error {
		cluster.TypedSpec().Value.TalosVersion = newTalosVersion

		return nil
	})
	require.NoError(t, err)

	// The phase is already Done from the cluster creation, so wait for it to leave that state before
	// waiting for it to come back.
	t.Run("TalosUpgradeShouldStart", func(t *testing.T) {
		rtestutils.AssertResources(ctx, t, omniState, []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Phase, resourceDetails(r))
		})
	})

	t.Run("TalosUpgradeShouldComplete", func(t *testing.T) {
		rtestutils.AssertResources(ctx, t, omniState, []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Done, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.Equal(newTalosVersion, r.TypedSpec().Value.LastUpgradeVersion, resourceDetails(r))
		})
	})

	t.Run("MachinesShouldRunTheNewTalosVersion", AssertTalosVersion(ctx, options, clusterName, newTalosVersion))
	t.Run("ClusterMachinesShouldBeReady", AssertClusterMachinesReady(ctx, omniState, clusterName))

	stopSampler()
	<-sampler

	t.Run("DrainShouldHaveRespectedTerminationGracePeriod", func(t *testing.T) {
		assertDrainTimeline(t, recorder, machineIDByNode)
	})

	t.Run("ClusterShouldConvergeWithoutConfigErrors", func(t *testing.T) {
		machineIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))

		rtestutils.AssertResources(ctx, t, omniState, machineIDs, func(r *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
			assert.Empty(r.TypedSpec().Value.GetLastConfigError(), resourceDetails(r))
		})

		nodes, err := kubeClient.CoreV1().Nodes().List(kubeCtx, metav1.ListOptions{})
		require.NoError(t, err)

		for _, node := range nodes.Items {
			assert.False(t, node.Spec.Unschedulable, "node %q was left cordoned after the upgrade", node.Name)
		}
	})

	t.Run("ClusterShouldBeDestroyed", AssertDestroyCluster(ctx, omniState, clusterName, false, false))
}

// drainWorkloadManifest builds the workload the drain has to wait for: two replicas pinned one per worker,
// each ignoring SIGTERM for drainWorkloadShutdown before exiting on its own.
//
// The PodDisruptionBudget allows exactly one disruption while both replicas are healthy, so the eviction is
// permitted while still proving Omni goes through the eviction API rather than deleting the pod outright.
func drainWorkloadManifest() string {
	return fmt.Sprintf(
		`apiVersion: v1
kind: Namespace
metadata:
  name: %[1]s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[2]s
  namespace: %[1]s
spec:
  replicas: 2
  selector:
    matchLabels:
      app: %[2]s
  template:
    metadata:
      labels:
        app: %[2]s
    spec:
      terminationGracePeriodSeconds: %[3]d
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: %[2]s
              topologyKey: kubernetes.io/hostname
      containers:
        - name: sleeper
          image: alpine:3
          imagePullPolicy: IfNotPresent
          command:
            - /bin/sh
            - -c
            - 'trap "sleep %[4]d; exit 0" TERM; while true; do sleep 1; done'
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: %[2]s
  namespace: %[1]s
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: %[2]s
`,
		drainWorkloadNamespace,
		drainWorkloadName,
		int(drainWorkloadGracePeriod.Seconds()),
		int(drainWorkloadShutdown.Seconds()),
	)
}

// applyDrainWorkload hands the workload to Omni as a KubernetesManifestGroup, the way a user would, and
// removes it again once the test is done.
func applyDrainWorkload(ctx context.Context, t *testing.T, st state.State, clusterName string) {
	group := omni.NewKubernetesManifestGroup(drainWorkloadName)
	group.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	group.TypedSpec().Value.Mode = specs.KubernetesManifestGroupSpec_FULL

	require.NoError(t, group.TypedSpec().Value.SetUncompressedData([]byte(drainWorkloadManifest())))
	require.NoError(t, st.Create(ctx, group))

	t.Cleanup(func() { //nolint:contextcheck
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		if err := st.Destroy(cleanupCtx, group.Metadata()); err != nil && !state.IsNotFoundError(err) {
			t.Logf("failed to clean up the drain workload manifest group: %v", err)
		}
	})
}

// waitForDrainWorkload blocks until both replicas run, each on a worker of its own.
func waitForDrainWorkload(ctx context.Context, t *testing.T, kubeClient *kubernetes.Clientset) {
	t.Log("waiting for the drain workload to be running on both workers")

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		pods, err := kubeClient.CoreV1().Pods(drainWorkloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: drainWorkloadSelector})
		if !assert.NoError(collect, err) {
			return
		}

		nodes := map[string]struct{}{}

		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning || pod.Spec.NodeName == "" {
				continue
			}

			nodes[pod.Spec.NodeName] = struct{}{}
		}

		assert.Len(collect, nodes, 2, "expected one running replica on each of the two workers")
	}, 5*time.Minute, 2*time.Second)
}

// machineIDByNodeName maps each cluster machine's Kubernetes node name back to its Omni machine ID, so a
// drained node can be tied to the ClusterMachineConfigStatus that reports the drain's outcome.
func machineIDByNodeName(ctx context.Context, t *testing.T, st state.State, clusterName string) map[string]resource.ID {
	identities, err := safe.StateListAll[*omni.ClusterMachineIdentity](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
	require.NoError(t, err)

	result := map[string]resource.ID{}

	for identity := range identities.All() {
		if nodename := identity.TypedSpec().Value.Nodename; nodename != "" {
			result[nodename] = identity.Metadata().ID()
		}
	}

	require.NotEmpty(t, result)

	return result
}

// drainPodObservation is the timeline the sampler reconstructs for a single workload pod.
type drainPodObservation struct {
	// deletionSeenAt is when the pod was first observed with a deletion timestamp, i.e. once the eviction landed.
	deletionSeenAt time.Time
	// goneAt is when the pod was first observed to be absent, i.e. when it finished terminating.
	goneAt   time.Time
	nodeName string
	// gracePeriod is the grace period the API server actually applied to the deletion.
	gracePeriod time.Duration
}

// drainNodeObservation is the timeline the sampler reconstructs for a single node.
type drainNodeObservation struct {
	rebootedAt        time.Time
	initialBootID     string
	everUnschedulable bool
}

// drainRecorder samples the workload cluster and Omni state throughout the upgrade, so the assertions can
// run against a recorded timeline. The drain failure in particular is only visible for as long as it takes
// the controller to retry, which is too narrow a window to catch with a point-in-time poll.
type drainRecorder struct {
	pods          map[types.UID]*drainPodObservation
	nodes         map[string]*drainNodeObservation
	drainFailures map[resource.ID]string
	mu            sync.Mutex
}

func newDrainRecorder() *drainRecorder {
	return &drainRecorder{
		pods:          map[types.UID]*drainPodObservation{},
		nodes:         map[string]*drainNodeObservation{},
		drainFailures: map[resource.ID]string{},
	}
}

// run samples until the context is canceled, returning a channel closed once the last sample is in.
func (r *drainRecorder) run(ctx context.Context, kubeClient *kubernetes.Clientset, st state.State, clusterName string) <-chan struct{} {
	done := make(chan struct{})

	// Take the first reading synchronously, so the nodes' pre-upgrade boot IDs are on record before the
	// caller triggers the upgrade.
	r.sample(ctx, kubeClient, st, clusterName)

	go func() {
		defer close(done)

		ticker := time.NewTicker(drainSampleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.sample(ctx, kubeClient, st, clusterName)
			}
		}
	}()

	return done
}

// sample takes one reading of each source. Every source is allowed to fail on its own: the workload API
// server is unreachable while the single control plane reboots, and a missed reading only costs precision.
func (r *drainRecorder) sample(ctx context.Context, kubeClient *kubernetes.Clientset, st state.State, clusterName string) {
	now := time.Now()

	if pods, err := kubeClient.CoreV1().Pods(drainWorkloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: drainWorkloadSelector}); err == nil {
		r.recordPods(now, pods.Items)
	}

	if nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		r.recordNodes(now, nodes.Items)
	}

	if statuses, err := safe.StateListAll[*omni.ClusterMachineConfigStatus](
		ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
	); err == nil {
		r.recordConfigStatuses(statuses)
	}
}

func (r *drainRecorder) recordPods(now time.Time, pods []corev1.Pod) {
	r.mu.Lock()
	defer r.mu.Unlock()

	present := make(map[types.UID]struct{}, len(pods))

	for _, pod := range pods {
		present[pod.UID] = struct{}{}

		observation, ok := r.pods[pod.UID]
		if !ok {
			observation = &drainPodObservation{nodeName: pod.Spec.NodeName}
			r.pods[pod.UID] = observation
		}

		if observation.nodeName == "" {
			observation.nodeName = pod.Spec.NodeName
		}

		if pod.DeletionTimestamp != nil && observation.deletionSeenAt.IsZero() {
			observation.deletionSeenAt = now

			if pod.DeletionGracePeriodSeconds != nil {
				observation.gracePeriod = time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second
			}
		}
	}

	for uid, observation := range r.pods {
		if _, ok := present[uid]; !ok && observation.goneAt.IsZero() {
			observation.goneAt = now
		}
	}
}

func (r *drainRecorder) recordNodes(now time.Time, nodes []corev1.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, node := range nodes {
		observation, ok := r.nodes[node.Name]
		if !ok {
			observation = &drainNodeObservation{initialBootID: node.Status.NodeInfo.BootID}
			r.nodes[node.Name] = observation
		}

		observation.everUnschedulable = observation.everUnschedulable || node.Spec.Unschedulable

		if node.Status.NodeInfo.BootID != observation.initialBootID && observation.rebootedAt.IsZero() {
			observation.rebootedAt = now
		}
	}
}

func (r *drainRecorder) recordConfigStatuses(statuses safe.List[*omni.ClusterMachineConfigStatus]) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for status := range statuses.All() {
		configError := status.TypedSpec().Value.GetLastConfigError()

		if !strings.Contains(configError, drainFailureMarker) {
			continue
		}

		if _, ok := r.drainFailures[status.Metadata().ID()]; !ok {
			r.drainFailures[status.Metadata().ID()] = configError
		}
	}
}

// firstEvictedPod returns the pod the drain evicted first, which is the one on the first worker Omni
// upgraded.
func (r *drainRecorder) firstEvictedPod() *drainPodObservation {
	r.mu.Lock()
	defer r.mu.Unlock()

	var first *drainPodObservation

	for _, observation := range r.pods {
		if observation.deletionSeenAt.IsZero() {
			continue
		}

		if first == nil || observation.deletionSeenAt.Before(first.deletionSeenAt) {
			first = observation
		}
	}

	return first
}

func (r *drainRecorder) node(name string) *drainNodeObservation {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.nodes[name]
}

func (r *drainRecorder) drainFailure(machineID resource.ID) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	configError, ok := r.drainFailures[machineID]

	return configError, ok
}

// assertDrainTimeline checks the recorded timeline of the first worker Omni drained.
func assertDrainTimeline(t *testing.T, recorder *drainRecorder, machineIDByNode map[string]resource.ID) {
	pod := recorder.firstEvictedPod()
	require.NotNil(t, pod, "no workload pod was ever evicted, so the upgrade never drained a node hosting one")
	require.NotEmpty(t, pod.nodeName, "the evicted pod was never observed with a node assigned")
	require.False(t, pod.goneAt.IsZero(), "the evicted pod was never observed to finish terminating")

	terminating := pod.goneAt.Sub(pod.deletionSeenAt)

	t.Logf("pod on node %q terminated in %s (grace period %s)", pod.nodeName, terminating, pod.gracePeriod)

	// The eviction must carry the pod's own terminationGracePeriodSeconds. Anything else means the drain
	// overrode it instead of passing -1 through to the API server.
	assert.Equal(t, drainWorkloadGracePeriod, pod.gracePeriod,
		"the eviction did not apply the pod's own terminationGracePeriodSeconds")

	// The pod outlived the drain's own deadline and still exited on its own terms, rather than being killed
	// when the grace period ran out.
	assert.GreaterOrEqual(t, terminating, nodedrain.DefaultDrainTimeout,
		"the pod stopped terminating before the drain timeout, so this run never exercised the long shutdown")
	assert.Less(t, terminating, drainWorkloadGracePeriod,
		"the pod was killed at its grace period instead of exiting on its own")

	// nodedrain caps the whole drain at DefaultDrainTimeout regardless of how long the pods are entitled to take,
	// so the first drain attempt fails and Omni retries it.
	machineID, ok := machineIDByNode[pod.nodeName]
	require.True(t, ok, "no Omni machine ID for node %q", pod.nodeName)

	configError, ok := recorder.drainFailure(machineID)
	assert.True(t, ok, "expected the drain of node %q to time out and surface in LastConfigError", pod.nodeName)
	t.Logf("recorded drain failure for machine %q: %s", machineID, configError)

	node := recorder.node(pod.nodeName)
	require.NotNil(t, node, "node %q was never observed", pod.nodeName)

	assert.True(t, node.everUnschedulable, "node %q was never cordoned", pod.nodeName)

	require.False(t, node.rebootedAt.IsZero(), "node %q was never observed to reboot", pod.nodeName)

	// The whole point of draining before rebooting: the machine only goes down once the pod is gone.
	assert.True(t, node.rebootedAt.After(pod.goneAt),
		"node %q rebooted at %s, before the pod finished terminating at %s",
		pod.nodeName, node.rebootedAt, pod.goneAt)
}
