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
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests/event"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/cli-utils/pkg/object/validation"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omniconsts "github.com/siderolabs/omni/internal/pkg/constants"
)

// ClusterManifestsStatusController manages config version for each cluster.
type ClusterManifestsStatusController struct {
	generic.NamedController
}

// NewClusterManifestsStatusController initializes ClusterKubernetesManifestsStatusController.
func NewClusterManifestsStatusController() *ClusterManifestsStatusController {
	ctrl := &ClusterManifestsStatusController{
		NamedController: generic.NamedController{
			ControllerName: "ClusterKubernetesManifestsStatusController",
		},
	}

	return ctrl
}

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
				Type:      omni.KubernetesManifestType,
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
	case omni.KubernetesManifestType:
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

	oneTimeManifests := []*unstructured.Unstructured{}

	if clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests == nil {
		clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests = map[string]*specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{}
	}

	clusterKubernetesManifestsStatus.TypedSpec().Value.Errors = map[string]string{}

	visited := map[string]struct{}{}

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

		ctrl.setManifestsPending(
			clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests,
			manifests,
			res.TypedSpec().Value.Mode,
			visited,
		)

		if res.TypedSpec().Value.Mode == specs.KubernetesManifestSpec_ONE_TIME {
			oneTimeManifests = append(oneTimeManifests, manifests...)

			return nil
		}

		manifestsByInventory[inventory] = append(manifestsByInventory[inventory], manifests...)

		return nil
	})
	if err != nil {
		return err
	}

	// delete statuses which were pending and no longer exist in the new resources state
	for id, status := range clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests {
		shouldDeleteIfNotFound := status.Phase == specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_PENDING ||
			status.Mode == specs.KubernetesManifestSpec_ONE_TIME

		if _, ok := visited[id]; !ok && shouldDeleteIfNotFound {
			delete(clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests, id)
		}
	}

	var errs error

	// update the status early to display pending manifests, as SSA is a long running operation
	if err = ctrl.updateStatus(ctx, r, clusterKubernetesManifestsStatus); err != nil {
		return err
	}

	for inventory, objects := range manifestsByInventory {
		if err = ctrl.sync(ctx, logger, objects, inventory, client, clusterKubernetesManifestsStatus); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if len(oneTimeManifests) > 0 {
		if err = ctrl.syncOneTime(ctx, logger, oneTimeManifests, client, clusterKubernetesManifestsStatus); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	// update the status once again after the run is finished
	if err = ctrl.updateStatus(ctx, r, clusterKubernetesManifestsStatus); err != nil {
		return err
	}

	return errs
}

func (ctrl *ClusterManifestsStatusController) updateStatus(ctx context.Context, r controller.ReaderWriter, status *omni.ClusterKubernetesManifestsStatus) error {
	return safe.WriterModify(ctx, r, status, func(r *omni.ClusterKubernetesManifestsStatus) error {
		r.TypedSpec().Value = status.TypedSpec().Value

		r.Metadata().Annotations().Do(func(temp kvutils.TempKV) {
			for key, value := range status.Metadata().Annotations().Raw() {
				temp.Set(key, value)
			}
		})

		r.TypedSpec().Value.OutOfSync = 0

		for _, status := range r.TypedSpec().Value.Manifests {
			if status.Phase != specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED {
				r.TypedSpec().Value.OutOfSync++
			}
		}

		r.TypedSpec().Value.Total = int32(len(r.TypedSpec().Value.Manifests))

		return nil
	})
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

func (ctrl *ClusterManifestsStatusController) setManifestsPending(
	statuses map[string]*specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus,
	manifests []*unstructured.Unstructured,
	mode specs.KubernetesManifestSpec_Mode,
	visited map[string]struct{},
) {
	for _, manifest := range manifests {
		id := object.ObjMetadata{Namespace: manifest.GetNamespace(), Name: manifest.GetName(), GroupKind: manifest.GroupVersionKind().GroupKind()}.String()

		if _, ok := statuses[id]; ok {
			continue
		}

		visited[id] = struct{}{}

		statuses[id] = &specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
			Phase:     specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_PENDING,
			Mode:      mode,
			Kind:      manifest.GetKind(),
			Name:      manifest.GetName(),
			Namespace: manifest.GetNamespace(),
		}
	}
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
	inv string,
	client *kubernetes.Client,
	clusterKubernetesManifestsStatus *omni.ClusterKubernetesManifestsStatus,
) error {
	errCh := make(chan error, 1)
	resultCh := make(chan event.Event)

	ssaOpts := manifests.SSAOptions{
		FieldManagerName:   constants.KubernetesFieldManagerName,
		InventoryNamespace: constants.KubernetesInventoryNamespace,
		InventoryName:      inv,
		SSApplyBehaviorOptions: manifests.SSApplyBehaviorOptions{
			InventoryPolicy:  inventory.PolicyAdoptIfNoInventory,
			ReconcileTimeout: 10 * time.Second,
			PruneTimeout:     10 * time.Second,
		},
	}

	logger.Info("sync manifests full", zap.String("inventory", inv), zap.Int("count", len(objects)))

	panichandler.Go(func() {
		errCh <- manifests.SyncSSA(ctx, objects, client.Config, resultCh, ssaOpts)
	}, logger)

	var (
		errs   error
		pruned = map[string]struct{}{}
	)

	setManifestStatus := func(result event.Event, phase specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_Phase) {
		var errorMessage string

		if result.Error != nil {
			errorMessage = result.Error.Error()
		}

		clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests[result.ObjectID.String()] = &specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
			LastError: errorMessage,
			Phase:     phase,
			Mode:      specs.KubernetesManifestSpec_FULL,
			Kind:      result.ObjectID.GroupKind.Kind,
			Name:      result.ObjectID.Name,
			Namespace: result.ObjectID.Namespace,
		}
	}

	for {
		select {
		case result := <-resultCh:
			id := result.ObjectID.String()

			if result.Error != nil {
				if strings.Contains(result.Error.Error(), "reconcile timed out") {
					errs = errors.Join(errs, controller.NewRequeueError(result.Error, time.Second*30))

					continue
				}

				if isFatalError(result.Error) {
					setManifestStatus(
						result,
						specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_FAILED,
					)

					logger.Info("failed to sync manifest", zap.Error(result.Error))

					continue
				}

				errs = errors.Join(result.Error)

				continue
			}

			//nolint:exhaustive
			switch result.Type {
			case event.PruneType:
				setManifestStatus(
					result,
					specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_DELETING,
				)

				pruned[id] = struct{}{}
			case event.ApplyType:
				setManifestStatus(
					result,
					specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_PROGRESSING,
				)
			case event.RolloutType:
				if _, ok := pruned[id]; ok {
					delete(clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests, id)

					continue
				}

				setManifestStatus(
					result,
					specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED,
				)
			}
		case err := <-errCh:
			if err != nil {
				if isFatalError(err) {
					logger.Error("failed to sync manifest", zap.Error(err))
					clusterKubernetesManifestsStatus.TypedSpec().Value.Errors[inv] = err.Error()

					return errs
				}
			}

			errs = errors.Join(errs, err)

			return errs
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

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	for _, manifest := range objects {
		id := object.ObjMetadata{Namespace: manifest.GetNamespace(), Name: manifest.GetName(), GroupKind: manifest.GroupVersionKind().GroupKind()}.String()

		gvk := manifest.GroupVersionKind()

		setManifestStatus := func(phase specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_Phase, err string) {
			clusterKubernetesManifestsStatus.TypedSpec().Value.Manifests[id] = &specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
				Phase:     phase,
				Mode:      specs.KubernetesManifestSpec_ONE_TIME,
				Kind:      gvk.Kind,
				Name:      manifest.GetName(),
				Namespace: manifest.GetNamespace(),
				LastError: err,
			}
		}

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
			setManifestStatus(
				specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED,
				"",
			)

			continue
		}

		_, err = dr.Create(ctx, manifest, v1.CreateOptions{FieldManager: constants.KubernetesFieldManagerName})
		if err != nil {
			if apierrors.IsConflict(err) {
				continue
			}

			if isFatalError(err) {
				logger.Error("failed to sync manifest", zap.Error(err))

				setManifestStatus(
					specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_FAILED,
					err.Error(),
				)

				continue
			}

			errs = errors.Join(errs, fmt.Errorf("create failed: %w", err))

			continue
		}

		updated++

		setManifestStatus(
			specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED,
			"",
		)
	}

	logger.Info("completed one time manifests sync", zap.Int("count", len(objects)), zap.Int("updated", updated))

	return errs
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
		apierrors.IsRequestEntityTooLargeError(err) || webhookError(err) || validationError(err) || strings.Contains(err.Error(), "reconcile failed")
}
