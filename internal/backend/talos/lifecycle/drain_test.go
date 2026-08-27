// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

// fixedClientsetProvider returns the same clientset for any cluster.
type fixedClientsetProvider struct {
	clientset k8s.Interface
}

func (p fixedClientsetProvider) GetKubernetesClientset(context.Context, string) (k8s.Interface, error) {
	return p.clientset, nil
}

func newDrainTestManager(t *testing.T, clientset k8s.Interface) *lifecycle.Manager {
	t.Helper()

	c, err := imagefactory.NewClient("factory.talos.dev", "", "")
	require.NoError(t, err)

	return lifecycle.NewManager(zapNop(t), imagefactory.NewClients(
		state.WrapCore(namespaced.NewState(inmem.Build)),
		c,
	), "ghcr.io/siderolabs/installer", fixedClientsetProvider{clientset: clientset}, nil, nil)
}

// TestCordonAndDrain verifies the two cordon/drain modes: the normal mode cordons and drains with
// failures aborting the operation as always, while the best-effort mode (a recovery of an unhealthy
// machine) only cordons, tolerates every failure, and never blocks the reboot.
func TestCordonAndDrain(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	const nodeName = "node-1"

	progressCollector := func(messages *[]string) func(string) {
		return func(msg string) { *messages = append(*messages, msg) }
	}

	joined := func(messages []string) string { return strings.Join(messages, "\n") }

	newNodeClientset := func() *fake.Clientset {
		return fake.NewSimpleClientset([]k8sruntime.Object{&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}}...)
	}

	t.Run("normalModeDrains", func(t *testing.T) {
		t.Parallel()

		clientset := newNodeClientset()

		var messages []string

		err := newDrainTestManager(t, clientset).RunCordonAndDrainForTest(ctx, "cluster-1", nodeName, false, progressCollector(&messages))
		require.NoError(t, err)

		assert.Contains(t, joined(messages), "drained")

		node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.True(t, node.Spec.Unschedulable, "the node must be cordoned")
	})

	t.Run("normalModeAbortsOnCordonFailure", func(t *testing.T) {
		t.Parallel()

		clientset := newNodeClientset()
		clientset.PrependReactor("patch", "nodes", func(ktesting.Action) (bool, k8sruntime.Object, error) {
			return true, nil, errors.New("api server unreachable")
		})

		var messages []string

		err := newDrainTestManager(t, clientset).RunCordonAndDrainForTest(ctx, "cluster-1", nodeName, false, progressCollector(&messages))
		require.Error(t, err, "a cordon failure must abort a normal operation, the machine is healthy and can wait")
	})

	t.Run("bestEffortDrains", func(t *testing.T) {
		t.Parallel()

		clientset := newNodeClientset()

		var messages []string

		err := newDrainTestManager(t, clientset).RunCordonAndDrainForTest(ctx, "cluster-1", nodeName, true, progressCollector(&messages))
		require.NoError(t, err)

		assert.Contains(t, joined(messages), "drained", "an evictable node must still be drained on a recovery")

		node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.True(t, node.Spec.Unschedulable, "the node must be cordoned")
	})

	t.Run("bestEffortToleratesDrainFailure", func(t *testing.T) {
		t.Parallel()

		clientset := newNodeClientset()
		clientset.PrependReactor("list", "pods", func(ktesting.Action) (bool, k8sruntime.Object, error) {
			return true, nil, errors.New("pod listing broken")
		})

		var messages []string

		err := newDrainTestManager(t, clientset).RunCordonAndDrainForTest(ctx, "cluster-1", nodeName, true, progressCollector(&messages))
		require.NoError(t, err, "a failing drain must not abort a recovery operation")

		assert.Contains(t, joined(messages), "failed to drain")
	})

	t.Run("bestEffortToleratesUnreachableAPI", func(t *testing.T) {
		t.Parallel()

		// Every node read and write fails, as when the machine being recovered hosts the only
		// control plane and its API server is down along with it.
		clientset := fake.NewSimpleClientset()
		clientset.PrependReactor("*", "nodes", func(ktesting.Action) (bool, k8sruntime.Object, error) {
			return true, nil, errors.New("api server unreachable")
		})

		var messages []string

		err := newDrainTestManager(t, clientset).RunCordonAndDrainForTest(ctx, "cluster-1", nodeName, true, progressCollector(&messages))
		require.NoError(t, err, "nothing may abort a recovery operation")

		assert.Contains(t, joined(messages), "failed to cordon")
	})
}
