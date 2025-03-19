// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/exposedservice"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/image"
)

// KubernetesStatusController manages KubernetesStatus resource lifecycle.
//
// KubernetesStatusController plays the role of machine discovery.
type KubernetesStatusController struct {
	watchers map[string]*kubernetesWatcher

	advertisedAPIURL       string
	workloadProxySubdomain string
}

// NewKubernetesStatusController creates a new KubernetesStatusController.
func NewKubernetesStatusController(advertisedAPIURL, workloadProxySubdomain string) *KubernetesStatusController {
	return &KubernetesStatusController{
		advertisedAPIURL:       advertisedAPIURL,
		workloadProxySubdomain: workloadProxySubdomain,
	}
}

// Name implements controller.Controller interface.
func (ctrl *KubernetesStatusController) Name() string {
	return "KubernetesStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubernetesStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterSecretsType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.LoadBalancerStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.KubeconfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ExposedServiceType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubernetesStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.KubernetesStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: omni.ExposedServiceType,
			Kind: controller.OutputExclusive,
		},
	}
}

type kubernetesWatcherNotify struct {
	status           *specs.KubernetesStatusSpec
	cluster          string
	servicesOptional optional.Optional[[]*corev1.Service]
}

// Run implements controller.Controller interface.
func (ctrl *KubernetesStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctrl.watchers = map[string]*kubernetesWatcher{}

	defer func() {
		for _, w := range ctrl.watchers {
			w.stop()
		}

		ctrl.watchers = nil
	}()

	notifyCh := make(chan kubernetesWatcherNotify)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcileRunners(ctx, r, logger, notifyCh); err != nil {
				return fmt.Errorf("error reconciling runners: %w", err)
			}

			r.ResetRestartBackoff()
		case ev := <-notifyCh:
			if _, running := ctrl.watchers[ev.cluster]; !running {
				// skip notifications for not running watchers (notification came late)
				continue
			}

			if services, servicesUpdated := ev.servicesOptional.Get(); servicesUpdated {
				if err := ctrl.updateExposedServices(ctx, r, ev.cluster, services, logger); err != nil {
					return fmt.Errorf("error updating exposed services: %w", err)
				}

				continue
			}

			// services are not updated, which means the update was a status update.

			if err := safe.WriterModify(ctx, r, omni.NewKubernetesStatus(resources.DefaultNamespace, ev.cluster),
				func(r *omni.KubernetesStatus) error {
					r.Metadata().Labels().Set(omni.LabelCluster, ev.cluster)

					if ev.status.Nodes != nil {
						r.TypedSpec().Value.Nodes = ev.status.Nodes
					}

					if ev.status.StaticPods != nil {
						r.TypedSpec().Value.StaticPods = ev.status.StaticPods
					}

					return nil
				}); err != nil {
				return fmt.Errorf("error updating kubernetes status: %w", err)
			}
		}
	}
}

func (ctrl *KubernetesStatusController) updateExposedServices(ctx context.Context, r controller.Runtime, cluster string, services []*corev1.Service, logger *zap.Logger) error {
	tracker := trackResource(r, resources.DefaultNamespace, omni.ExposedServiceType, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))

	reconciler, err := exposedservice.NewReconciler(ctx, r, logger, cluster, ctrl.workloadProxySubdomain, ctrl.advertisedAPIURL, services)
	if err != nil {
		return fmt.Errorf("error creating exposed service reconciler: %w", err)
	}

	servicesToKeep, err := reconciler.ReconcileServices(ctx)
	if err != nil {
		return fmt.Errorf("error reconciling services: %w", err)
	}

	for _, service := range servicesToKeep {
		tracker.keep(service)
	}

	return tracker.cleanup(ctx)
}

//nolint:gocyclo,cyclop
func (ctrl *KubernetesStatusController) reconcileRunners(ctx context.Context, r controller.Runtime, logger *zap.Logger, notifyCh chan<- kubernetesWatcherNotify) error {
	secrets, err := safe.ReaderListAll[*omni.ClusterSecrets](ctx, r)
	if err != nil {
		return err
	}

	lbStatus, err := safe.ReaderListAll[*omni.LoadBalancerStatus](ctx, r)
	if err != nil {
		return err
	}

	secretsPresent := map[string]struct{}{}

	for secret := range secrets.All() {
		secretsPresent[secret.Metadata().ID()] = struct{}{}
	}

	lbHealthy := map[string]struct{}{}
	lbStopped := map[string]struct{}{}

	for lbs := range lbStatus.All() {
		if lbs.TypedSpec().Value.Healthy {
			lbHealthy[lbs.Metadata().ID()] = struct{}{}
		}

		if lbs.TypedSpec().Value.Stopped {
			lbStopped[lbs.Metadata().ID()] = struct{}{}
		}
	}

	// stop watchers where secrets are gone (cluster deleted?) and remove the corresponding status resources
	for cluster, w := range ctrl.watchers {
		_, hasSecrets := secretsPresent[cluster]
		_, isStopped := lbStopped[cluster]

		reason := "cluster deleted"
		if isStopped {
			reason = "cluster is offline"
		}

		if !hasSecrets || isStopped {
			w.stop()

			delete(ctrl.watchers, cluster)

			logger.Info("stopped watcher for cluster", zap.String("cluster", cluster), zap.String("reason", reason))
		}
	}

	// start watchers (if not already running) for cluster which have secrets, lb healthy and not stopped
	for cluster := range secretsPresent {
		_, lbHealthy := lbHealthy[cluster]
		if !lbHealthy {
			continue
		}

		if _, isStopped := lbStopped[cluster]; isStopped {
			continue
		}

		_, alreadyRunning := ctrl.watchers[cluster]
		if alreadyRunning {
			continue
		}

		if err = ctrl.startWatcher(ctx, logger, cluster, notifyCh); err != nil {
			return fmt.Errorf("failed to start watcher for the cluster:%w", err)
		}

		logger.Info("started watcher for cluster", zap.String("cluster", cluster))
	}

	// cleanup kubernetes statuses for cluster which do not have secrets
	statuses, err := safe.ReaderListAll[*omni.KubernetesStatus](ctx, r)
	if err != nil {
		return err
	}

	for status := range statuses.All() {
		cluster := status.Metadata().ID()

		if _, ok := secretsPresent[cluster]; ok {
			continue
		}

		if err := r.Destroy(ctx, omni.NewKubernetesStatus(resources.DefaultNamespace, cluster).Metadata()); err != nil {
			return fmt.Errorf("error destroying kubernetes status: %w", err)
		}
	}

	// cleanup exposed services which are
	// - either destroy ready
	// - or clusters which do not have secrets
	if err := ctrl.cleanupExposedServices(ctx, r, secretsPresent); err != nil {
		return fmt.Errorf("error destroying exposed services: %w", err)
	}

	return nil
}

func (ctrl *KubernetesStatusController) cleanupExposedServices(ctx context.Context, r controller.Runtime, keepClustersSet map[string]struct{}) error {
	exposedServices, err := safe.ReaderListAll[*omni.ExposedService](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing exposed services: %w", err)
	}

	shouldDestroy := func(res *omni.ExposedService) bool {
		if res.Metadata().Phase() == resource.PhaseTearingDown {
			return res.Metadata().Finalizers().Empty()
		}

		exposedServiceCluster, ok := res.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return false
		}

		if _, ok = keepClustersSet[exposedServiceCluster]; ok {
			return false
		}

		return true
	}

	var multiErr error

	for exposedService := range exposedServices.All() {
		if !shouldDestroy(exposedService) {
			continue
		}

		destroyReady, teardownErr := r.Teardown(ctx, exposedService.Metadata())
		if teardownErr != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("error tearing down exposed service: %w", teardownErr))

			continue
		}

		if !destroyReady {
			continue
		}

		if destroyErr := r.Destroy(ctx, exposedService.Metadata()); destroyErr != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("error destroying exposed service: %w", destroyErr))
		}
	}

	return multiErr
}

var controlplanePodSelector = func() labels.Selector {
	sel, err := labels.Parse("tier=control-plane,k8s-app in (kube-apiserver,kube-controller-manager,kube-scheduler)")
	if err != nil {
		panic(err)
	}

	return sel
}()

func (ctrl *KubernetesStatusController) startWatcher(ctx context.Context, logger *zap.Logger, cluster string, notifyCh chan<- kubernetesWatcherNotify) error {
	r, err := runtime.LookupInterface[kubernetesRuntime](kubernetes.Name)
	if err != nil {
		return err
	}

	client, err := r.GetClient(ctx, cluster)
	if err != nil {
		return err
	}

	w := &kubernetesWatcher{
		logger:  logger,
		cluster: cluster,
		client:  client, // keep the reference to the client to prevent it from being garbage collected
	}

	w.nodeFactory = informers.NewSharedInformerFactory(w.client.Clientset(), 0)

	if _, err = w.nodeFactory.Core().V1().Nodes().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { w.nodesSync(ctx, notifyCh) },
		UpdateFunc: func(_ any, _ any) { w.nodesSync(ctx, notifyCh) },
		DeleteFunc: func(_ any) { w.nodesSync(ctx, notifyCh) },
	}); err != nil {
		return err
	}

	w.podFactory = informers.NewSharedInformerFactoryWithOptions(w.client.Clientset(), 0,
		informers.WithNamespace("kube-system"),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = controlplanePodSelector.String()
		}),
	)

	if _, err = w.podFactory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { w.podsSync(ctx, notifyCh) },
		UpdateFunc: func(_ any, _ any) { w.podsSync(ctx, notifyCh) },
		DeleteFunc: func(_ any) { w.podsSync(ctx, notifyCh) },
	}); err != nil {
		return err
	}

	ctx, w.ctxCancel = context.WithCancel(ctx)

	w.nodeFactory.Start(ctx.Done())
	w.podFactory.Start(ctx.Done())

	panichandler.Go(func() {
		w.nodeFactory.WaitForCacheSync(ctx.Done())
		w.nodesSynced.Store(true)

		w.nodesSync(ctx, notifyCh)
	}, logger)

	panichandler.Go(func() {
		w.podFactory.WaitForCacheSync(ctx.Done())
		w.podsSynced.Store(true)

		w.podsSync(ctx, notifyCh)
	}, logger)

	if config.Config.WorkloadProxying.Enabled {
		w.serviceFactory = informers.NewSharedInformerFactory(w.client.Clientset(), 0)

		if _, err = w.serviceFactory.Core().V1().Services().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) { filterExposedServiceEvents(obj, nil, logger, func() { w.servicesSync(ctx, notifyCh) }) },
			UpdateFunc: func(oldObj, newObj any) {
				filterExposedServiceEvents(newObj, oldObj, logger, func() { w.servicesSync(ctx, notifyCh) })
			},
			DeleteFunc: func(obj any) { filterExposedServiceEvents(obj, nil, logger, func() { w.servicesSync(ctx, notifyCh) }) },
		}); err != nil {
			return err
		}

		w.serviceFactory.Start(ctx.Done())

		panichandler.Go(func() {
			w.serviceFactory.WaitForCacheSync(ctx.Done())
			w.servicesSynced.Store(true)

			w.servicesSync(ctx, notifyCh)
		}, logger)
	}

	ctrl.watchers[cluster] = w

	return nil
}

type kubernetesWatcher struct {
	logger         *zap.Logger
	client         *kubernetes.Client
	nodeFactory    informers.SharedInformerFactory
	podFactory     informers.SharedInformerFactory
	serviceFactory informers.SharedInformerFactory
	ctxCancel      context.CancelFunc
	cluster        string
	nodesSynced    atomic.Bool
	podsSynced     atomic.Bool
	servicesSynced atomic.Bool
}

func (w *kubernetesWatcher) stop() {
	w.ctxCancel()
	w.nodeFactory.Shutdown()
	w.podFactory.Shutdown()
}

func (w *kubernetesWatcher) nodesSync(ctx context.Context, notifyCh chan<- kubernetesWatcherNotify) {
	if !w.nodesSynced.Load() {
		return
	}

	w.logger.Debug("nodes sync event", zap.String("cluster", w.cluster))

	nodes, err := w.nodeFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err != nil {
		w.logger.Error("nodes list failed", zap.String("cluster", w.cluster), zap.Error(err))
	}

	status := &specs.KubernetesStatusSpec{}
	status.Nodes = make([]*specs.KubernetesStatusSpec_NodeStatus, len(nodes))

	for i, node := range nodes {
		status.Nodes[i] = &specs.KubernetesStatusSpec_NodeStatus{
			Nodename:       node.Name,
			KubeletVersion: strings.TrimLeft(node.Status.NodeInfo.KubeletVersion, "v"),
		}

		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				status.Nodes[i].Ready = condition.Status == corev1.ConditionTrue
			}
		}
	}

	slices.SortFunc(status.Nodes, func(a, b *specs.KubernetesStatusSpec_NodeStatus) int {
		return cmp.Compare(a.Nodename, b.Nodename)
	})

	channel.SendWithContext(ctx, notifyCh,
		kubernetesWatcherNotify{
			cluster: w.cluster,
			status:  status,
		})
}

func (w *kubernetesWatcher) podsSync(ctx context.Context, notifyCh chan<- kubernetesWatcherNotify) {
	if !w.podsSynced.Load() {
		return
	}

	w.logger.Debug("pods sync event", zap.String("cluster", w.cluster))

	pods, err := w.podFactory.Core().V1().Pods().Lister().List(controlplanePodSelector)
	if err != nil {
		w.logger.Error("pods list failed", zap.String("cluster", w.cluster), zap.Error(err))
	}

	status := &specs.KubernetesStatusSpec{}

	podsByNode := map[string][]*corev1.Pod{}

	for _, pod := range pods {
		podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
	}

	nodes := maps.Keys(podsByNode)
	slices.Sort(nodes)

	status.StaticPods = make([]*specs.KubernetesStatusSpec_NodeStaticPods, 0, len(nodes))

	for _, node := range nodes {
		nodeStaticPods := &specs.KubernetesStatusSpec_NodeStaticPods{
			Nodename:   node,
			StaticPods: make([]*specs.KubernetesStatusSpec_StaticPodStatus, 0, len(podsByNode[node])),
		}

		for _, pod := range podsByNode[node] {
			staticPod := &specs.KubernetesStatusSpec_StaticPodStatus{
				App: pod.Labels["k8s-app"],
			}

			for _, container := range pod.Spec.Containers {
				if container.Name != staticPod.App {
					continue
				}

				tag, err := image.GetTag(container.Image)
				if err != nil {
					w.logger.Error("failed to get image tag", zap.String("cluster", w.cluster), zap.Error(err), zap.String("image", container.Image))

					continue
				}

				staticPod.Version = strings.TrimLeft(tag, "v")

				break
			}

			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady {
					staticPod.Ready = condition.Status == corev1.ConditionTrue
				}
			}

			nodeStaticPods.StaticPods = append(nodeStaticPods.StaticPods, staticPod)
		}

		slices.SortFunc(nodeStaticPods.StaticPods, func(a, b *specs.KubernetesStatusSpec_StaticPodStatus) int {
			return cmp.Compare(a.App, b.App)
		})

		status.StaticPods = append(status.StaticPods, nodeStaticPods)
	}

	channel.SendWithContext(ctx, notifyCh,
		kubernetesWatcherNotify{
			cluster: w.cluster,
			status:  status,
		})
}

func (w *kubernetesWatcher) servicesSync(ctx context.Context, notifyCh chan<- kubernetesWatcherNotify) {
	if !w.servicesSynced.Load() {
		return
	}

	w.logger.Debug("services sync event", zap.String("cluster", w.cluster))

	services, err := w.serviceFactory.Core().V1().Services().Lister().List(labels.Everything())
	if err != nil {
		w.logger.Error("services list failed", zap.String("cluster", w.cluster), zap.Error(err))
	}

	// filter out non-exposed services
	services = slices.DeleteFunc(services, func(service *corev1.Service) bool {
		return !exposedservice.IsExposedServiceEvent(service, nil, w.logger)
	})

	channel.SendWithContext(ctx, notifyCh,
		kubernetesWatcherNotify{
			cluster:          w.cluster,
			servicesOptional: optional.Some(services),
		})
}

func filterExposedServiceEvents(k8sObject, oldK8sObject any, logger *zap.Logger, syncFunc func()) {
	if exposedservice.IsExposedServiceEvent(k8sObject, oldK8sObject, logger) {
		syncFunc()
	}
}
