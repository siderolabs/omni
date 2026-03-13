// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	fluxobject "github.com/fluxcd/cli-utils/pkg/object"
	fluxssa "github.com/fluxcd/pkg/ssa"
	"github.com/fluxcd/pkg/ssa/utils"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-kubernetes/kubernetes/ssa"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/cli-utils/pkg/object/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omniconsts "github.com/siderolabs/omni/internal/pkg/constants"
)

// KubernetesRuntime provides kubernetes cluster access capabilities.
type KubernetesRuntime interface {
	GetKubeconfig(ctx context.Context, cluster *common.Context) (*rest.Config, error)
	GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
}

// ClusterManifestsStatusController manages config version for each cluster.
type ClusterManifestsStatusController struct {
	kubernetesRuntime KubernetesRuntime
	generic.NamedController
}

// NewClusterManifestsStatusController initializes ClusterKubernetesManifestsStatusController.
func NewClusterManifestsStatusController(kubernetesRuntime KubernetesRuntime) *ClusterManifestsStatusController {
	ctrl := &ClusterManifestsStatusController{
		NamedController: generic.NamedController{
			ControllerName: "ClusterKubernetesManifestsStatusController",
		},
		kubernetesRuntime: kubernetesRuntime,
	}

	return ctrl
}

// ControllerOption configures ClusterManifestsStatusController.
type ControllerOption func(*ClusterManifestsStatusController)

// Settings implements controller.QController interface.
func (ctrl *ClusterManifestsStatusController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.KubernetesManifestGroupType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.LoadBalancerStatusType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterStatusType,
				Kind:      controller.InputQMapped,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputExclusive,
				Type: omni.ClusterKubernetesManifestsStatusType,
			},
		},
		Concurrency: optional.Some[uint](64),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *ClusterManifestsStatusController) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case omni.ClusterType,
		omni.LoadBalancerStatusType,
		omni.ClusterStatusType:
		return []resource.Pointer{
			omni.NewCluster(ptr.ID()).Metadata(),
		}, nil
	case omni.KubernetesManifestGroupType:
		clusterID, ok := ptr.Labels().Get(omni.LabelCluster)
		if !ok {
			return nil, nil
		}

		return []resource.Pointer{
			omni.NewCluster(clusterID).Metadata(),
		}, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *ClusterManifestsStatusController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	res, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if res.Metadata().Phase() == resource.PhaseTearingDown {
		return ctrl.reconcileTearingDown(ctx, r, res)
	}

	status, err := safe.ReaderGetByID[*omni.ClusterKubernetesManifestsStatus](ctx, r, res.Metadata().ID())
	if err != nil {
		if !state.IsNotFoundError(err) {
			return err
		}

		status = omni.NewClusterKubernetesManifestsStatus(res.Metadata().ID())
	}

	return ctrl.reconcileRunning(ctx, r, logger, res, status)
}

// nolint:gocyclo,cyclop,gocognit
func (ctrl *ClusterManifestsStatusController) reconcileRunning(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	cluster *omni.Cluster, clusterKubernetesManifestsStatus *omni.ClusterKubernetesManifestsStatus,
) error {
	if !cluster.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, cluster.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	if _, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked); locked {
		logger.Info("skip applying manifest: the cluster is locked")

		return nil
	}

	loadbalancerStatus, err := safe.ReaderGet[*omni.LoadBalancerStatus](ctx, r, omni.NewLoadBalancerStatus(cluster.Metadata().ID()).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get loadbalancer status: %w", err)
	}

	if loadbalancerStatus == nil || !loadbalancerStatus.TypedSpec().Value.Healthy {
		logger.Info("skip applying manifest: loadbalancer is not healthy")

		return nil
	}

	clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](ctx, r, omni.NewClusterStatus(cluster.Metadata().ID()).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get cluster status: %w", err)
	}

	if clusterStatus == nil {
		logger.Info("skip applying manifest: cluster status doesn't exist yet")

		return nil
	}

	if !clusterStatus.TypedSpec().Value.HasConnectedControlPlanes {
		logger.Info("skip applying manifest: no control planes are connected")

		return nil
	}

	manifestList, err := safe.ReaderListAll[*omni.KubernetesManifestGroup](ctx, r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return err
	}

	manifestsByInventory := map[string][]*unstructured.Unstructured{
		omniconsts.SSAOmniInternalInventory:    {},
		omniconsts.SSAOmniUserInventory:        {},
		omniconsts.SSAOmniOneTimeSyncInventory: {},
	}

	client, err := ctrl.kubernetesRuntime.GetClient(ctx, cluster.Metadata().ID())
	if err != nil {
		return err
	}

	manifestsGroups := map[string]string{}
	groups := make(map[string]*specs.KubernetesManifestGroupSpec, manifestList.Len())

	err = manifestList.ForEachErr(func(res *omni.KubernetesManifestGroup) error {
		groups[res.Metadata().ID()] = res.TypedSpec().Value

		inventory := omniconsts.SSAOmniUserInventory

		_, systemManifest := res.Metadata().Labels().Get(omni.LabelSystemManifest)
		if systemManifest {
			inventory = omniconsts.SSAOmniInternalInventory
		}

		if res.TypedSpec().Value.Mode == specs.KubernetesManifestGroupSpec_ONE_TIME {
			inventory = omniconsts.SSAOmniOneTimeSyncInventory
		}

		var manifests []*unstructured.Unstructured

		manifests, err = ctrl.readManifests(res, client)
		if err != nil {
			return err
		}

		if res.TypedSpec().Value.Mode == specs.KubernetesManifestGroupSpec_ONE_TIME {
			// skip adding manifests to the oneTimeManifests list as it's one time and was applied
			if clusterKubernetesManifestsStatus.TypedSpec().Value.IsGroupApplied(res.Metadata().ID()) {
				return nil
			}

			manifests = xslices.Filter(manifests, func(m *unstructured.Unstructured) bool {
				id := utils.FmtUnstructured(m)

				return clusterKubernetesManifestsStatus.TypedSpec().Value.GetManifestPhase(res.Metadata().ID(), id) != specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED
			})
		}

		manifestsByInventory[inventory] = append(manifestsByInventory[inventory], manifests...)

		for _, m := range manifests {
			id := utils.FmtUnstructured(m)
			manifestsGroups[id] = res.Metadata().ID()
		}

		return nil
	})
	if err != nil {
		return err
	}

	id := cluster.Metadata().ID()

	// update the status before applying manifests to set pending status for new manifests
	clusterKubernetesManifestsStatus, err = ctrl.updateStatus(ctx, r, client, id, groups, manifestsGroups, manifestsByInventory, nil)
	if err != nil {
		return err
	}

	var errs error

	for inventory, objects := range manifestsByInventory {
		if err = ctrl.sync(ctx, logger, objects, manifestsGroups, inventory, client); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	var lastError string

	if errs != nil && isFatalError(errs) {
		logger.Error("failed to apply manifests", zap.Error(errs))

		lastError = errs.Error()

		errs = nil
	}

	// update the status once again after the run is finished
	clusterKubernetesManifestsStatus, err = ctrl.updateStatus(ctx, r, client, id, groups, manifestsGroups, manifestsByInventory, &lastError)
	if err != nil {
		return err
	}

	if errs == nil && clusterKubernetesManifestsStatus.TypedSpec().Value.OutOfSync > 0 {
		return controller.NewRequeueInterval(time.Second * 30)
	}

	return errs
}

// nolint:gocyclo,cyclop,gocognit
func (ctrl *ClusterManifestsStatusController) updateStatus(
	ctx context.Context,
	r controller.ReaderWriter,
	client *kubernetes.Client,
	clusterName string,
	groups map[string]*specs.KubernetesManifestGroupSpec,
	manifestsGroups map[string]string,
	manifestsByInventory map[string][]*unstructured.Unstructured,
	lastError *string,
) (*omni.ClusterKubernetesManifestsStatus, error) {
	getResource := func(id string, obj object.ObjMetadata) (*unstructured.Unstructured, error) {
		mapping, err := client.Mapper.RESTMapping(obj.GroupKind)
		if err != nil {
			return nil, fmt.Errorf("failed to get REST mapping for object %v: %w", id, err)
		}

		var dr dynamic.ResourceInterface

		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			dr = client.Dynamic().Resource(mapping.Resource).Namespace(obj.Namespace)
		} else {
			dr = client.Dynamic().Resource(mapping.Resource)
		}

		return dr.Get(ctx, obj.Name, v1.GetOptions{})
	}

	return safe.WriterModifyWithResult(ctx, r, omni.NewClusterKubernetesManifestsStatus(clusterName),
		func(r *omni.ClusterKubernetesManifestsStatus) error {
			visited := make(map[string]struct{})

			// check status for all manifests that are applied (exists in inventories) and update the status accordingly
			for _, inventory := range []string{
				omniconsts.SSAOmniInternalInventory,
				omniconsts.SSAOmniUserInventory,
				omniconsts.SSAOmniOneTimeSyncInventory,
			} {
				inv, err := ssa.GetInventory(ctx, client.Clientset(), constants.KubernetesInventoryNamespace, inventory)
				if err != nil {
					return fmt.Errorf("failed to get inventory %q: %w", inventory, err)
				}

				for _, obj := range inv.Get() {
					id := utils.FmtObjMetadata(obj)

					objMetadata := object.ObjMetadata(obj)

					visited[id] = struct{}{}

					groupName, ok := manifestsGroups[id]
					if !ok {
						continue
					}

					_, err = getResource(id, objMetadata)
					if err != nil {
						if apierrors.IsNotFound(err) {
							continue
						}

						return err
					}

					r.TypedSpec().Value.SetManifestStatus(groupName, id, objMetadata, specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED)
				}
			}

			// add pending status for manifests that are not visited (e.g. new manifests that were not applied yet)
			for _, inventory := range []string{
				omniconsts.SSAOmniInternalInventory,
				omniconsts.SSAOmniUserInventory,
				omniconsts.SSAOmniOneTimeSyncInventory,
			} {
				for _, manifest := range manifestsByInventory[inventory] {
					obj := object.UnstructuredToObjMetadata(manifest)

					id := utils.FmtObjMetadata(fluxobject.ObjMetadata(obj))

					if _, ok := visited[id]; ok {
						continue
					}

					groupName, ok := manifestsGroups[id]
					if !ok {
						continue
					}

					r.TypedSpec().Value.SetManifestStatus(groupName, id, obj, specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_PENDING)
				}
			}

			// set deleting status for manifests that are not in manifestsGroups and exist in the status (e.g. manifests that were deleted)
			for groupName, group := range r.TypedSpec().Value.Groups {
				for id, manifest := range group.Manifests {
					if _, ok := manifestsGroups[id]; ok {
						continue
					}

					if group.Mode == specs.KubernetesManifestGroupSpec_ONE_TIME {
						continue
					}

					obj := object.ObjMetadata{
						Name:      manifest.Name,
						Namespace: manifest.Namespace,
						GroupKind: schema.GroupKind{
							Group: manifest.Group,
							Kind:  manifest.Kind,
						},
					}

					res, err := getResource(id, obj)
					if err != nil && !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
						return err
					}

					if res != nil {
						r.TypedSpec().Value.SetManifestStatus(groupName, id, obj, specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_DELETING)

						continue
					}

					r.TypedSpec().Value.DeleteManifestStatus(groupName, id)
				}
			}

			if lastError != nil {
				r.TypedSpec().Value.LastError = *lastError
			}

			r.TypedSpec().Value.UpdateGroups(groups)

			return nil
		},
	)
}

func (ctrl *ClusterManifestsStatusController) reconcileTearingDown(ctx context.Context, r controller.ReaderWriter, cluster *omni.Cluster) error {
	ready, err := helpers.TeardownAndDestroy(ctx, r, omni.NewClusterKubernetesManifestsStatus(cluster.Metadata().ID()).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if !ready {
		return nil
	}

	return r.RemoveFinalizer(ctx, cluster.Metadata(), ctrl.Name())
}

func (ctrl *ClusterManifestsStatusController) readManifests(res *omni.KubernetesManifestGroup, client *kubernetes.Client) ([]*unstructured.Unstructured, error) {
	manifests, err := res.TypedSpec().Value.GetManifests()
	if err != nil {
		return nil, err
	}

	for _, obj := range manifests {
		if res.TypedSpec().Value.Namespace != "" {
			gvk := obj.GroupVersionKind()

			mapping, err := client.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return nil, err
			}

			if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
				obj.SetNamespace(res.TypedSpec().Value.Namespace)
			}
		}

		annotations := obj.GetAnnotations()

		if annotations == nil {
			annotations = make(map[string]string)
		}

		annotations[omni.KubernetesManifestOwner] = res.Metadata().ID()

		obj.SetAnnotations(annotations)
	}

	return manifests, nil
}

func (ctrl *ClusterManifestsStatusController) sync(
	ctx context.Context,
	logger *zap.Logger,
	objects []*unstructured.Unstructured,
	groups map[string]string,
	inv string,
	client *kubernetes.Client,
) error {
	var (
		manager *ssa.Manager
		err     error
	)

	manager, err = ctrl.newSSAManager(client, inv)
	if err != nil {
		return err
	}

	defer manager.Close()

	applyOpts := ssa.ApplyOptions{
		InventoryPolicy: ssa.InventoryPolicyAdoptIfNoInventory,
		WaitTimeout:     10 * time.Second,
		WaitInterval:    5 * time.Second,
		NoPrune:         inv == omniconsts.SSAOmniOneTimeSyncInventory,
	}

	var requeue bool

	changes, err := manager.Apply(ctx, objects, applyOpts)
	if err != nil {
		return err
	}

	objectsMap := xslices.ToMap(objects, func(obj *unstructured.Unstructured) (string, *unstructured.Unstructured) {
		return utils.FmtUnstructured(obj), obj
	})

	for _, change := range changes {
		group := groups[change.Subject]

		obj := objectsMap[change.Subject]
		if obj == nil {
			continue
		}

		logger.Debug("manifest status",
			zap.String("name", obj.GetName()),
			zap.String("namespace", obj.GetNamespace()),
			zap.String("group", group),
			zap.String("action", string(change.Action)),
		)

		// nolint:exhaustive
		switch change.Action {
		case ssa.CreatedAction, ssa.ConfiguredAction:
			requeue = true
		}
	}

	if requeue {
		return controller.NewRequeueInterval(time.Second * 30)
	}

	return nil
}

func webhookError(err error) bool {
	msg := err.Error()

	return strings.Contains(msg, "failed calling webhook") || strings.Contains(msg, "failed to call webhook")
}

func validationError(err error) bool {
	var validationError *validation.Error

	return errors.As(err, &validationError)
}

func isFatalError(err error) bool {
	return apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) || apierrors.IsForbidden(err) ||
		apierrors.IsRequestEntityTooLargeError(err) || webhookError(err) || validationError(err)
}

func (ctrl *ClusterManifestsStatusController) newSSAManager(
	c *kubernetes.Client,
	inventoryName string,
) (*ssa.Manager, error) {
	httpClient, err := rest.HTTPClientFor(c.Config)
	if err != nil {
		return nil, err
	}

	dc, err := discovery.NewDiscoveryClientForConfigAndClient(c.Config, httpClient)
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

	kubeClient, err := client.New(c.Config, client.Options{
		HTTPClient: httpClient,
		Mapper:     mapper,
	})
	if err != nil {
		return nil, err
	}

	poller := polling.NewStatusPoller(kubeClient, mapper, polling.Options{})

	inventoryFactory := func(ctx context.Context) (ssa.Inventory, error) {
		return ssa.GetInventory(ctx, c.Clientset(), constants.KubernetesInventoryNamespace, inventoryName)
	}

	resourceManager := &resourceManagerWithGet{
		ResourceManager: *fluxssa.NewResourceManager(kubeClient, poller, fluxssa.Owner{
			Field: constants.KubernetesFieldManagerName,
		}),
		kubeClient: kubeClient,
		mapper:     mapper,
	}

	return ssa.NewCustomManager(resourceManager, inventoryFactory, httpClient, mapper), nil
}

type resourceManagerWithGet struct {
	kubeClient client.Client
	mapper     meta.RESTMapper

	fluxssa.ResourceManager
}

func (r *resourceManagerWithGet) Get(ctx context.Context, objMeta fluxobject.ObjMetadata) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}

	mapping, err := r.mapper.RESTMapping(objMeta.GroupKind)
	if err != nil {
		return nil, fmt.Errorf("failed to map kind %q to a supported version: %w", objMeta.GroupKind, err)
	}

	obj.SetGroupVersionKind(mapping.GroupVersionKind)

	err = r.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: objMeta.Namespace,
		Name:      objMeta.Name,
	}, obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}
