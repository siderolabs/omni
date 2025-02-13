// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

// WorkloadProxyReconciler reconciles the workload proxies for a cluster.
type WorkloadProxyReconciler interface {
	// Reconcile reconciles the workload proxies for a cluster.
	Reconcile(cluster resource.ID, rd *workloadproxy.ReconcileData) error
	DropAlias(alias string) bool
}

// ClusterWorkloadProxyStatusControllerName is the name of the controller.
const ClusterWorkloadProxyStatusControllerName = "ClusterWorkloadProxyStatusController"

// ClusterWorkloadProxyStatusController creates the system config patch that contains the maintenance config.
type ClusterWorkloadProxyStatusController = qtransform.QController[*omni.Cluster, *omni.ClusterWorkloadProxyStatus]

// NewClusterWorkloadProxyStatusController initializes ClusterWorkloadProxyStatusController.
func NewClusterWorkloadProxyStatusController(workloadProxyReconciler WorkloadProxyReconciler) *ClusterWorkloadProxyStatusController {
	helper := &clusterWorkloadProxyStatusControllerHelper{
		workloadProxyReconciler: workloadProxyReconciler,
	}

	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterWorkloadProxyStatus]{
			Name: ClusterWorkloadProxyStatusControllerName,
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterWorkloadProxyStatus {
				return omni.NewClusterWorkloadProxyStatus(resources.DefaultNamespace, cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(ClusterWorkloadProxyStatus *omni.ClusterWorkloadProxyStatus) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, ClusterWorkloadProxyStatus.Metadata().ID())
			},
			TransformExtraOutputFunc:        helper.transform,
			FinalizerRemovalExtraOutputFunc: helper.teardown,
		},
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.ExposedService, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.ClusterMachineStatus, *omni.Cluster](),
		),

		qtransform.WithConcurrency(2),
	)
}

type clusterWorkloadProxyStatusControllerHelper struct {
	workloadProxyReconciler WorkloadProxyReconciler
}

//nolint:gocognit
func (helper *clusterWorkloadProxyStatusControllerHelper) transform(ctx context.Context, r controller.ReaderWriter,
	logger *zap.Logger, cluster *omni.Cluster, status *omni.ClusterWorkloadProxyStatus,
) error {
	cmsList, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
	if err != nil {
		return fmt.Errorf("failed to list cluster machine statuses: %w", err)
	}

	for cms := range cmsList.All() {
		if cms.Metadata().Phase() == resource.PhaseTearingDown {
			if err = r.RemoveFinalizer(ctx, cms.Metadata(), ClusterWorkloadProxyStatusControllerName); err != nil {
				return fmt.Errorf("failed to remove finalizer: %w", err)
			}

			continue
		}

		if err = r.AddFinalizer(ctx, cms.Metadata(), ClusterWorkloadProxyStatusControllerName); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if !cluster.TypedSpec().Value.GetFeatures().GetEnableWorkloadProxy() {
		if err = helper.workloadProxyReconciler.Reconcile(cluster.Metadata().ID(), nil); err != nil {
			return fmt.Errorf("failed to reconcile load balancers (feature disabled): %w", err)
		}

		status.TypedSpec().Value.NumExposedServices = 0

		return nil
	}

	healthyTargetHosts := make([]string, 0, cmsList.Len())

	for cms := range cmsList.All() {
		if cms.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		if !cms.TypedSpec().Value.GetReady() {
			continue
		}

		if cms.TypedSpec().Value.ManagementAddress == "" {
			continue
		}

		healthyTargetHosts = append(healthyTargetHosts, cms.TypedSpec().Value.ManagementAddress)
	}

	slices.Sort(healthyTargetHosts)

	svcList, err := safe.ReaderListAll[*omni.ExposedService](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
	if err != nil {
		return fmt.Errorf("failed to list exposed services: %w", err)
	}

	rd := &workloadproxy.ReconcileData{
		AliasPort: make(map[string]string, svcList.Len()),
		Hosts:     healthyTargetHosts,
	}

	for svc := range svcList.All() {
		if svc.Metadata().Phase() == resource.PhaseTearingDown {
			alias, ok := svc.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
			if ok {
				helper.workloadProxyReconciler.DropAlias(alias)
			}

			if err = r.RemoveFinalizer(ctx, svc.Metadata(), ClusterWorkloadProxyStatusControllerName); err != nil {
				return fmt.Errorf("failed to remove finalizer: %w", err)
			}

			continue
		}

		if err = r.AddFinalizer(ctx, svc.Metadata(), ClusterWorkloadProxyStatusControllerName); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}

		alias, ok := svc.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
		if !ok {
			logger.Warn("missing label", zap.String("label", omni.LabelExposedServiceAlias), zap.String("service", svc.Metadata().ID()))

			continue
		}

		rd.AliasPort[alias] = strconv.Itoa(int(svc.TypedSpec().Value.Port))
	}

	if err = helper.workloadProxyReconciler.Reconcile(cluster.Metadata().ID(), rd); err != nil {
		return fmt.Errorf("failed to reconcile load balancers: %w", err)
	}

	status.TypedSpec().Value.NumExposedServices = uint32(len(rd.AliasPort))

	return nil
}

func (helper *clusterWorkloadProxyStatusControllerHelper) teardown(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, cluster *omni.Cluster) error {
	cmsList, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
	if err != nil {
		return fmt.Errorf("failed to list cluster machine statuses: %w", err)
	}

	for cms := range cmsList.All() {
		if err = r.RemoveFinalizer(ctx, cms.Metadata(), ClusterWorkloadProxyStatusControllerName); err != nil {
			return fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	svcList, err := safe.ReaderListAll[*omni.ExposedService](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
	if err != nil {
		return fmt.Errorf("failed to list exposed services: %w", err)
	}

	for svc := range svcList.All() {
		alias, ok := svc.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
		if !ok {
			logger.Warn("missing label", zap.String("label", omni.LabelExposedServiceAlias), zap.String("service", svc.Metadata().ID()))
		} else {
			helper.workloadProxyReconciler.DropAlias(alias)
		}

		if err = r.RemoveFinalizer(ctx, svc.Metadata(), ClusterWorkloadProxyStatusControllerName); err != nil {
			return fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	if err = helper.workloadProxyReconciler.Reconcile(cluster.Metadata().ID(), nil); err != nil {
		logger.Error("failed to reconcile load balancers", zap.Error(err))
	}

	return nil
}
