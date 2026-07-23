// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testutils

import (
	"context"
	"errors"
	"sync"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

const (
	testImageFactoryHost = "factory-test.talos.dev"
	testTalosRegistry    = "ghcr.io/siderolabs/installer"
)

// FakeKubernetesProvider implements lifecycle.KubernetesClientProvider, returning a fixed clientset for any cluster.
type FakeKubernetesProvider struct {
	clientset k8s.Interface
	mu        sync.Mutex
}

// SetClientset sets the clientset returned to cordon/drain/uncordon. Seed it with the cluster's nodes.
func (p *FakeKubernetesProvider) SetClientset(clientset k8s.Interface) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.clientset = clientset
}

// GetKubernetesClientset returns the fake clientset.
func (p *FakeKubernetesProvider) GetKubernetesClientset(context.Context, string) (k8s.Interface, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.clientset == nil {
		return nil, errors.New("fake kubernetes provider has no clientset configured, call SetClientset first")
	}

	return p.clientset, nil
}

// NewFakeKubernetesClientset builds a fake clientset seeded with schedulable nodes of the given names and
// the eviction subresource advertised, so nodedrain's cordon/drain/uncordon paths work against it.
func NewFakeKubernetesClientset(nodeNames ...string) *fake.Clientset {
	objects := make([]k8sruntime.Object, 0, len(nodeNames))
	for _, name := range nodeNames {
		objects = append(objects, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}})
	}

	clientset := fake.NewSimpleClientset(objects...)
	clientset.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "pods/eviction", Namespaced: true, Kind: "Eviction", Group: "policy", Version: "v1"},
			},
		},
	}

	return clientset
}

// socketTalosClientFactory hands the Manager a real Talos client that reaches the machine mock over its
// unix socket. The production ClientFactory's maintenance path dials a SideroLink endpoint rather than a
// socket, so tests use this socket-aware factory instead; the clients it returns are real and do real RPCs.
type socketTalosClientFactory struct {
	st state.State
}

func (f socketTalosClientFactory) GetForMachine(ctx context.Context, machineID string) (*talos.Client, error) {
	machineStatus, err := safe.StateGetByID[*omni.MachineStatus](ctx, f.st, machineID)
	if err != nil {
		return nil, err
	}

	c, err := talos.NewMaintenanceClient(ctx, machineStatus.TypedSpec().Value.ManagementAddress)
	if err != nil {
		return nil, err
	}

	return talos.NewClient(c, "", machineID), nil
}

// NewLifecycleManager builds a real lifecycle.Manager wired to a Talos client factory that reaches the
// machine mocks over their sockets and to the given Kubernetes client provider.
func NewLifecycleManager(st state.State, k8sProvider lifecycle.KubernetesClientProvider) *lifecycle.Manager {
	return lifecycle.NewManager(
		zap.NewNop(),
		testImageFactoryHost,
		testTalosRegistry,
		k8sProvider,
		socketTalosClientFactory{st: st},
		nil,
	)
}
