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
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
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
	ptr               resource.Pointer
	state             state.State
	kubeRuntime       KubernetesClientGetter
	factory           informers.SharedInformerFactory
	ctx               context.Context // nolint:containedctx
	stopCh            chan struct{}
	logger            *zap.Logger
	ctxCancel         context.CancelFunc
	factoryStopCh     chan struct{}
	kubeClientCloseCh <-chan struct{}
	clusterName       string
	wg                sync.WaitGroup
	factoryWg         sync.WaitGroup
	mu                sync.Mutex
	synced            atomic.Bool
}

// KubernetesClientGetter gets Kubernetes clients for clusters.
type KubernetesClientGetter interface {
	GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
}

// NewKubernetesUsageFactory returns a ProducerFactory that starts virtual watch on the KubernetesUsage resource.
// The resource is computed from the multiple Kubernetes watches.
func NewKubernetesUsageFactory(kr KubernetesClientGetter) func(ctx context.Context, state state.State, ptr resource.Pointer, logger *zap.Logger) (Producer, error) {
	return func(ctx context.Context, state state.State, ptr resource.Pointer, logger *zap.Logger) (Producer, error) {
		p, ok := ptr.(*kubernetesUsagePointer)
		if !ok {
			return nil, errors.New("failed to get kubernetes client: cluster name can not be resolved")
		}

		kuberuntimeCtx, cancel := context.WithCancel(context.Background())
		kuberuntimeCtx = actor.MarkContextAsInternalActor(kuberuntimeCtx)

		ku := &KubernetesUsage{
			ctx:         kuberuntimeCtx,
			ctxCancel:   cancel,
			ptr:         ptr,
			clusterName: p.ClusterName(),
			kubeRuntime: kr,
			stopCh:      make(chan struct{}),
			logger:      logger.With(logging.Component("compute_kubernetes_usage")),
			state:       state,
		}

		if err := ku.createFactory(); err != nil {
			return nil, err
		}

		return ku, nil
	}
}

// createFactory creates a new kubernetes client and informer factory.
func (ku *KubernetesUsage) createFactory() error {
	client, err := ku.kubeRuntime.GetClient(ku.ctx, ku.clusterName)
	if err != nil {
		ku.mu.Lock()
		defer ku.mu.Unlock()

		ku.logger.Error("failed to get kubernetes client", zap.Error(err))
		ku.kubeClientCloseCh = make(chan struct{})
		ku.factoryStopCh = make(chan struct{})

		return err
	}

	ku.mu.Lock()
	defer ku.mu.Unlock()

	ku.factory = informers.NewSharedInformerFactory(client.Clientset(), 0)
	ku.kubeClientCloseCh = client.Wait()
	ku.factoryStopCh = make(chan struct{})

	return nil
}

func (ku *KubernetesUsage) getFactory() informers.SharedInformerFactory {
	ku.mu.Lock()
	defer ku.mu.Unlock()

	return ku.factory
}

func (ku *KubernetesUsage) stopFactory() {
	ku.mu.Lock()
	defer ku.mu.Unlock()

	close(ku.factoryStopCh)
}

// Start implements Producer interface.
func (ku *KubernetesUsage) Start() error {
	if err := ku.startFactory(); err != nil {
		return err
	}

	ku.wg.Add(1)

	panichandler.Go(func() {
		defer ku.wg.Done()

		for {
			select {
			case <-ku.stopCh:
				ku.ctxCancel()
				ku.stopFactoryAndWait()

				return
			case <-ku.kubeClientCloseCh:
				ku.logger.Info("kubernetes client closed, refreshing kubernetes client")
				ku.refresh()
			}
		}
	}, ku.logger)

	return nil
}

func (ku *KubernetesUsage) sync() {
	if !ku.synced.Load() {
		return
	}

	factory := ku.getFactory()

	var (
		nodes []*corev1.Node
		pods  []*corev1.Pod
		err   error
	)

	nodes, err = factory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err != nil {
		ku.logger.Error("failed to list nodes", zap.Error(err))

		return
	}

	pods, err = factory.Core().V1().Pods().Lister().List(labels.Everything())
	if err != nil {
		ku.logger.Error("failed to list pods", zap.Error(err))

		return
	}

	usage := virtual.NewKubernetesUsage(ku.ptr.ID())

	calculateUsage(pods, nodes, usage)

	_, err = safe.StateUpdateWithConflicts[*virtual.KubernetesUsage](ku.ctx, ku.state, usage.Metadata(), func(res *virtual.KubernetesUsage) error {
		res.TypedSpec().Value = usage.TypedSpec().Value

		return nil
	})
	if state.IsNotFoundError(err) {
		err = ku.state.Create(ku.ctx, usage)
	}

	if err != nil {
		ku.logger.Error("failed to update usage", zap.Error(err))
	}
}

// startFactory sets up informer handlers and starts the factory.
// Must be called with factory already created.
func (ku *KubernetesUsage) startFactory() error {
	ku.mu.Lock()
	defer ku.mu.Unlock()

	if _, err := ku.factory.Core().V1().Nodes().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { ku.sync() },
		UpdateFunc: func(_ any, _ any) { ku.sync() },
		DeleteFunc: func(_ any) { ku.sync() },
	}); err != nil {
		return err
	}

	if _, err := ku.factory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { ku.sync() },
		UpdateFunc: func(_ any, _ any) { ku.sync() },
		DeleteFunc: func(_ any) { ku.sync() },
	}); err != nil {
		return err
	}

	ku.factory.Start(ku.factoryStopCh)

	ku.factoryWg.Add(2)

	factoryStopCh := ku.factoryStopCh
	factory := ku.factory

	panichandler.Go(func() {
		defer ku.factoryWg.Done()

		factory.WaitForCacheSync(factoryStopCh)

		select {
		case <-factoryStopCh:
			return
		default:
		}

		ku.synced.Store(true)

		ku.sync()
	}, ku.logger)

	panichandler.Go(func() {
		defer ku.factoryWg.Done()

		<-factoryStopCh

		ku.synced.Store(false)

		factory.Shutdown()
	}, ku.logger)

	return nil
}

// stopFactoryAndWait stops the current factory and waits for its goroutines.
func (ku *KubernetesUsage) stopFactoryAndWait() {
	ku.stopFactory()
	ku.factoryWg.Wait()
}

// refresh stops the current factory, creates a new client and factory, and starts it.
func (ku *KubernetesUsage) refresh() {
	ku.logger.Info("refreshing kubernetes client")

	ku.stopFactoryAndWait()

	if err := ku.createFactory(); err != nil {
		ku.logger.Error("failed to create factory during refresh", zap.Error(err))

		return
	}

	if err := ku.startFactory(); err != nil {
		ku.logger.Error("failed to start factory during refresh", zap.Error(err))
	}
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
