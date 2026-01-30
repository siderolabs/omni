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

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests/event"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object/validation"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	omniconsts "github.com/siderolabs/omni/internal/pkg/constants"
)

// ClusterManifestsStatusController manages config version for each cluster.
type ClusterManifestsStatusController struct {
	controller.QController
}

// NewClusterManifestsStatusController initializes ClusterKubernetesManifestsStatusController.
func NewClusterManifestsStatusController() *ClusterManifestsStatusController {
	ctrl := &ClusterManifestsStatusController{}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterKubernetesManifestsStatus]{
			Name: "ClusterKubernetesManifestsStatusController",
			MapMetadataFunc: func(manifest *omni.Cluster) *omni.ClusterKubernetesManifestsStatus {
				return omni.NewClusterKubernetesManifestsStatus(manifest.Metadata().ID())
			},
			UnmapMetadataFunc: func(manifestStatus *omni.ClusterKubernetesManifestsStatus) *omni.Cluster {
				return omni.NewCluster(manifestStatus.Metadata().ID())
			},
			TransformFunc: ctrl.transform,
		},
		qtransform.WithExtraMappedInput[*omni.KubernetesManifest](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, in controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				clusterID, ok := in.Labels().Get(omni.LabelCluster)
				if !ok {
					return nil, nil
				}

				return []resource.Pointer{
					omni.NewCluster(clusterID).Metadata(),
				}, nil
			},
		),
		qtransform.WithExtraMappedInput[*omni.LoadBalancerStatus](
			mappers.MapClusterResourceToLabeledResources[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterStatus](
			mappers.MapClusterResourceToLabeledResources[*omni.Cluster](),
		),
	)

	return ctrl
}

// nolint:gocyclo,cyclop
func (ctrl *ClusterManifestsStatusController) transform(ctx context.Context, r controller.Reader, logger *zap.Logger,
	cluster *omni.Cluster, kubernetesManifestStatus *omni.ClusterKubernetesManifestsStatus,
) error {
	if _, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked); locked {
		logger.Info("skip applying manifest: the cluster is locked")

		return nil
	}

	loadbalancerStatus, err := safe.ReaderGet[*omni.LoadBalancerStatus](ctx, r, omni.NewLoadBalancerStatus(cluster.Metadata().ID()).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get loadbalancer status: %w", err)
	}

	if loadbalancerStatus == nil || !loadbalancerStatus.TypedSpec().Value.Healthy {
		return nil
	}

	clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](ctx, r, omni.NewClusterStatus(cluster.Metadata().ID()).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get cluster status: %w", err)
	}

	if clusterStatus == nil {
		return nil
	}

	if !clusterStatus.TypedSpec().Value.HasConnectedControlPlanes {
		return nil
	}

	manifestList, err := safe.ReaderListAll[*omni.KubernetesManifest](ctx, r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return err
	}

	manifestsByInventory := map[string][]manifests.Manifest{
		omniconsts.SSAOmniInternalInventory: {},
		omniconsts.SSAOmniUserInventory:     {},
	}

	type kubernetesConfigurator interface {
		GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
	}

	kubernetesRuntime, err := runtime.LookupInterface[kubernetesConfigurator](kubernetes.Name)
	if err != nil {
		return err
	}

	client, err := kubernetesRuntime.GetClient(ctx, cluster.Metadata().ID())
	if err != nil {
		return err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(client.Config)
	if err != nil {
		return fmt.Errorf("error building discovery client: %w", err)
	}

	dc := memory.NewMemCacheClient(discoveryClient)

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

	inventories := make(map[string]inventory.Inventory, 2)

	for _, id := range []string{
		omniconsts.SSAOmniInternalInventory,
		omniconsts.SSAOmniUserInventory,
	} {
		var inventory inventory.Inventory

		inventory, err = getInventory(ctx, client.Config, mapper, dc, id)
		if err != nil {
			return err
		}

		inventories[id] = inventory
	}

	oneTimeManifests := []*unstructured.Unstructured{}

	err = manifestList.ForEachErr(func(res *omni.KubernetesManifest) error {
		inventory := omniconsts.SSAOmniUserInventory

		_, systemManifest := res.Metadata().Labels().Get(omni.LabelSystemManifest)
		if systemManifest {
			inventory = omniconsts.SSAOmniInternalInventory
		}

		var manifests []*unstructured.Unstructured

		manifests, err = ctrl.readManifests(res, client)
		if err != nil {
			return err
		}

		if res.TypedSpec().Value.Mode == specs.KubernetesManifestSpec_ONE_TIME {
			oneTimeManifests = append(oneTimeManifests, manifests...)

			return nil
		}

		manifestsByInventory[inventory] = manifests

		return nil
	})
	if err != nil {
		return err
	}

	kubernetesManifestStatus.TypedSpec().Value.Manifests = map[string]*specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{}
	kubernetesManifestStatus.TypedSpec().Value.Errors = map[string]string{}

	var errs error

	for inventory, objects := range manifestsByInventory {
		if err = ctrl.sync(ctx, logger, objects, inventory, client, kubernetesManifestStatus); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if len(oneTimeManifests) > 0 {
		if err = ctrl.syncOneTime(ctx, logger, oneTimeManifests, client, kubernetesManifestStatus); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (ctrl *ClusterManifestsStatusController) readManifests(res *omni.KubernetesManifest, client *kubernetes.Client) ([]*unstructured.Unstructured, error) {
	manifests, err := res.TypedSpec().Value.GetManifests()
	if err != nil {
		return nil, err
	}

	if res.TypedSpec().Value.Namespace != "" {
		for _, obj := range manifests {
			gvk := obj.GroupVersionKind()

			mapping, err := client.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return nil, err
			}

			if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
				obj.SetNamespace(res.TypedSpec().Value.Namespace)
			}
		}
	}

	return manifests, nil
}

func (ctrl *ClusterManifestsStatusController) sync(
	ctx context.Context,
	logger *zap.Logger,
	objects []*unstructured.Unstructured,
	inventory string,
	client *kubernetes.Client,
	clusterKubernetesManifestsStatus *omni.ClusterKubernetesManifestsStatus,
) error {
	errCh := make(chan error, 1)
	resultCh := make(chan event.Event)

	ssaOpts := manifests.SSAOptions{
		FieldManagerName:       constants.KubernetesFieldManagerName,
		InventoryNamespace:     constants.KubernetesInventoryNamespace,
		InventoryName:          inventory,
		SSApplyBehaviorOptions: manifests.DefaultSSApplyBehaviorOptions(),
	}

	logger.Info("sync manifests full", zap.String("inventory", inventory), zap.Int("count", len(objects)))

	panichandler.Go(func() {
		errCh <- manifests.SyncSSA(ctx, objects, client.Config, resultCh, ssaOpts)
	}, logger)

	for {
		select {
		case result := <-resultCh:
			id := result.ObjectID.Namespace + "/" + result.ObjectID.Name

			if result.Error != nil {
				clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests[id] = &specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
					LastError: result.Error.Error(),
					Phase:     specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_FAILED,
					Mode:      specs.KubernetesManifestSpec_FULL,
				}

				continue
			}

			if result.Type == event.PruneType || result.Type == event.RolloutType {
				continue
			}

			clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests[id] = &specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
				Phase: specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED,
				Mode:  specs.KubernetesManifestSpec_FULL,
			}
		case err := <-errCh:
			if err != nil {
				if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) || apierrors.IsForbidden(err) || apierrors.IsRequestEntityTooLargeError(err) || webhookError(err) || validationError(err) {
					logger.Error("failed to sync manifest", zap.Error(err))
					clusterKubernetesManifestsStatus.TypedSpec().Value.Errors[inventory] = err.Error()

					return nil
				}
			}

			return err
		}
	}
}

func (ctrl *ClusterManifestsStatusController) syncOneTime(
	ctx context.Context,
	logger *zap.Logger,
	objects []*unstructured.Unstructured,
	client *kubernetes.Client,
	clusterKubernetesManifestsStatus *omni.ClusterKubernetesManifestsStatus,
) error {
	var (
		errs    error
		updated int
	)

	for _, manifest := range objects {
		id := "one-time/" + manifest.GetNamespace() + "/" + manifest.GetName()

		gvk := manifest.GroupVersionKind()

		mapping, err := client.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}

		var dr dynamic.ResourceInterface

		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// default the namespace if it's not set in the manifest
			if manifest.GetNamespace() == "" {
				manifest.SetNamespace(v1.NamespaceDefault)
			}

			// namespaced resources should specify the namespace
			dr = client.Dynamic().Resource(mapping.Resource).Namespace(manifest.GetNamespace())
		} else {
			// for cluster-wide resources
			dr = client.Dynamic().Resource(mapping.Resource)
		}

		o, err := dr.Get(ctx, manifest.GetName(), v1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		if o != nil {
			clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests[id] = &specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
				Phase: specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED,
				Mode:  specs.KubernetesManifestSpec_ONE_TIME,
			}

			continue
		}

		_, err = dr.Create(ctx, manifest, v1.CreateOptions{FieldManager: constants.KubernetesFieldManagerName})
		if err != nil {
			if apierrors.IsConflict(err) {
				continue
			}

			if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) || apierrors.IsForbidden(err) || apierrors.IsRequestEntityTooLargeError(err) || webhookError(err) || validationError(err) {
				logger.Error("failed to sync manifest", zap.Error(err))
				clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests[id] = &specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
					Phase:     specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_FAILED,
					Mode:      specs.KubernetesManifestSpec_ONE_TIME,
					LastError: err.Error(),
				}

				continue
			}

			errs = errors.Join(errs, fmt.Errorf("create failed: %w", err))

			continue
		}

		updated++

		clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests[id] = &specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
			Phase: specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED,
			Mode:  specs.KubernetesManifestSpec_ONE_TIME,
		}
	}

	logger.Info("sync manifests one time", zap.Int("count", len(objects)), zap.Int("updated", updated))

	return errs
}

func getInventory(
	ctx context.Context,
	kubeconfig *rest.Config,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	dc discovery.CachedDiscoveryInterface,
	id string,
) (inventory.Inventory, error) {
	clientGetter := manifests.K8sRESTClientGetter{
		RestConfig:      kubeconfig,
		Mapper:          mapper,
		DiscoveryClient: dc,
	}

	factory := util.NewFactory(clientGetter)

	inventoryClient, err := inventory.ConfigMapClientFactory{StatusEnabled: true}.NewClient(factory)
	if err != nil {
		return nil, err
	}

	inventoryInfo := inventory.NewSingleObjectInfo(inventory.ID(
		id),
		types.NamespacedName{
			Namespace: constants.KubernetesInventoryNamespace,
			Name:      id,
		})

	inv, err := inventoryClient.Get(ctx, inventoryInfo, inventory.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &inventory.SingleObjectInventory{}, nil
		}

		return nil, err
	}

	return inv, err
}

func webhookError(err error) bool {
	msg := err.Error()

	return strings.Contains(msg, "failed calling webhook") || strings.Contains(msg, "failed to call webhook")
}

func validationError(err error) bool {
	var validationError *validation.Error

	return errors.As(err, &validationError)
}
