// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package producers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	resourcehelper "github.com/siderolabs/omni/internal/pkg/kubernetes/resource"
)

type kubernetesUsagePointer struct {
	resource.Pointer

	id string
}

func (c *kubernetesUsagePointer) ID() resource.ID {
	return c.id
}

func (c *kubernetesUsagePointer) ClusterName() string {
	return c.Pointer.ID()
}

// KubernetesUsageResourceTransformer adds cluster uuid to the resource ID to guarantee unique watch id for each cluster generation.
func KubernetesUsageResourceTransformer(state state.State) func(context.Context, resource.Pointer) (resource.Pointer, error) {
	return func(ctx context.Context, ptr resource.Pointer) (resource.Pointer, error) {
		uuid, err := safe.ReaderGetByID[*omni.ClusterUUID](ctx, state, ptr.ID())
		if err != nil {
			return nil, err
		}

		return &kubernetesUsagePointer{
			Pointer: ptr,
			id:      ptr.ID() + "/" + uuid.TypedSpec().Value.Uuid,
		}, nil
	}
}

// KubernetesUsage producer.
type KubernetesUsage struct {
	logger  *zap.Logger
	ptr     resource.Pointer
	state   state.State
	factory informers.SharedInformerFactory
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewKubernetesUsage starts virtual watch on the KubernetesUsage resource.
// The resource is computed from the multiple Kubernetes watches.
func NewKubernetesUsage(ctx context.Context, state state.State, ptr resource.Pointer, logger *zap.Logger) (Producer, error) {
	type kubernetesClient interface {
		GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
	}

	r, err := runtime.LookupInterface[kubernetesClient](kubernetes.Name)
	if err != nil {
		return nil, err
	}

	p, ok := ptr.(*kubernetesUsagePointer)
	if !ok {
		return nil, errors.New("failed to get kubernetes client: cluster name can not be resolved")
	}

	client, err := r.GetClient(ctx, p.ClusterName())
	if err != nil {
		return nil, err
	}

	factory := informers.NewSharedInformerFactory(client.Clientset(), 0)

	return &KubernetesUsage{
		ptr:     ptr,
		factory: factory,
		stopCh:  make(chan struct{}),
		logger:  logger.With(logging.Component("compute_kubernetes_usage")),
		state:   state,
	}, nil
}

// Start implements Producer interface.
func (ku *KubernetesUsage) Start() error {
	var synced atomic.Bool

	sync := func() {
		if !synced.Load() {
			return
		}

		var (
			nodes []*corev1.Node
			pods  []*corev1.Pod
			err   error
		)

		nodes, err = ku.factory.Core().V1().Nodes().Lister().List(labels.Everything())
		if err != nil {
			ku.logger.Error("failed to list nodes", zap.Error(err))

			return
		}

		pods, err = ku.factory.Core().V1().Pods().Lister().List(labels.Everything())
		if err != nil {
			ku.logger.Error("failed to list pods", zap.Error(err))

			return
		}

		usage := virtual.NewKubernetesUsage(ku.ptr.ID())

		calculateUsage(pods, nodes, usage)

		_, err = safe.StateUpdateWithConflicts[*virtual.KubernetesUsage](context.Background(), ku.state, usage.Metadata(), func(res *virtual.KubernetesUsage) error {
			res.TypedSpec().Value = usage.TypedSpec().Value

			return nil
		})
		if state.IsNotFoundError(err) {
			err = ku.state.Create(context.Background(), usage)
		}

		if err != nil {
			ku.logger.Error("failed to update usage", zap.Error(err))
		}
	}

	if _, err := ku.factory.Core().V1().Nodes().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { sync() },
		UpdateFunc: func(_ any, _ any) { sync() },
		DeleteFunc: func(_ any) { sync() },
	}); err != nil {
		return err
	}

	if _, err := ku.factory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { sync() },
		UpdateFunc: func(_ any, _ any) { sync() },
		DeleteFunc: func(_ any) { sync() },
	}); err != nil {
		return err
	}

	ku.factory.Start(ku.stopCh)

	ku.wg.Add(2)

	panichandler.Go(func() {
		defer ku.wg.Done()

		ku.factory.WaitForCacheSync(ku.stopCh)

		select {
		case <-ku.stopCh:
			// stopped
			return
		default:
		}

		synced.Store(true)

		sync()
	}, ku.logger)

	panichandler.Go(func() {
		defer ku.wg.Done()

		<-ku.stopCh

		synced.Store(false)

		ku.factory.Shutdown()
	}, ku.logger)

	return nil
}

// Stop implements Producer interface.
func (ku *KubernetesUsage) Stop() {
	close(ku.stopCh)
}

// Cleanup implements Producer interface.
func (ku *KubernetesUsage) Cleanup() {
	ku.wg.Wait()
}

// calculateUsage computes pod usage for the cluster.
func calculateUsage(pods []*corev1.Pod, nodes []*corev1.Node, usage *virtual.KubernetesUsage) {
	spec := usage.TypedSpec().Value

	if spec.Cpu == nil {
		spec.Cpu = &specs.KubernetesUsageSpec_Quantity{}
	}

	if spec.Mem == nil {
		spec.Mem = &specs.KubernetesUsageSpec_Quantity{}
	}

	if spec.Storage == nil {
		spec.Storage = &specs.KubernetesUsageSpec_Quantity{}
	}

	if spec.Pods == nil {
		spec.Pods = &specs.KubernetesUsageSpec_Pod{}
	}

	spec.Pods.Count = int32(len(pods))

	spec.Cpu.Limits = 0
	spec.Mem.Limits = 0
	spec.Storage.Limits = 0

	spec.Cpu.Requests = 0
	spec.Mem.Requests = 0
	spec.Storage.Requests = 0

	for _, pod := range pods {
		requests, limits := resourcehelper.PodRequestsAndLimits(pod)

		spec.Cpu.Limits += limits.Cpu().AsApproximateFloat64()
		spec.Mem.Limits += limits.Memory().AsApproximateFloat64()
		spec.Storage.Limits += limits.StorageEphemeral().AsApproximateFloat64()

		spec.Cpu.Requests += requests.Cpu().AsApproximateFloat64()
		spec.Mem.Requests += requests.Memory().AsApproximateFloat64()
		spec.Storage.Requests += requests.StorageEphemeral().AsApproximateFloat64()
	}

	spec.Cpu.Capacity = 0
	spec.Mem.Capacity = 0
	spec.Storage.Capacity = 0
	spec.Pods.Capacity = 0

	for _, node := range nodes {
		spec.Cpu.Capacity += node.Status.Capacity.Cpu().AsApproximateFloat64()
		spec.Mem.Capacity += node.Status.Capacity.Memory().AsApproximateFloat64()
		spec.Storage.Capacity += node.Status.Allocatable.StorageEphemeral().AsApproximateFloat64()
		spec.Pods.Capacity += int32(node.Status.Capacity.Pods().Value())
	}
}
