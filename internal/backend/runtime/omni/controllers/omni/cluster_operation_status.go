// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterOperationStatusController manages the ClusterOperationStatus and ClusterMachineExtendedConfig resources' lifecycle.
//
// ClusterOperationStatusController handles rolling updates for the cluster's machines.
type ClusterOperationStatusController = qtransform.QController[*omni.Cluster, *omni.ClusterOperationStatus]

// NewClusterOperationStatusController initializes ClusterOperationStatusController.
//
//nolint:gocognit,gocyclo,cyclop
func NewClusterOperationStatusController() *ClusterOperationStatusController {
	helper := clusterOperationStatusControllerHelper{
		controllerName: "ClusterOperationStatusController",
	}

	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterOperationStatus]{
			Name: helper.controllerName,
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterOperationStatus {
				return omni.NewClusterOperationStatus(resources.DefaultNamespace, cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterOperationStatus *omni.ClusterOperationStatus) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, clusterOperationStatus.Metadata().ID())
			},
			TransformExtraOutputFunc:        helper.transform,
			FinalizerRemovalExtraOutputFunc: helper.finalizerRemoval,
		},
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.ClusterMachineConfig, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.MachineConfigGenOptions, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedDestroyReadyInput(
			mappers.MapByClusterLabel[*omni.ClusterMachineExtendedConfig, *omni.Cluster](),
		),
		qtransform.WithExtraOutputs(controller.Output{
			Type: omni.ClusterMachineExtendedConfigType,
			Kind: controller.OutputExclusive,
		}),
	)
}

type clusterOperationStatusControllerHelper struct {
	controllerName string
}

func (helper clusterOperationStatusControllerHelper) transform(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, cluster *omni.Cluster, _ *omni.ClusterOperationStatus) error {
	cms, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return fmt.Errorf("failed to get cluster machines for cluster %q: %w", cluster.Metadata().ID(), err)
	}

	logger.Debug(fmt.Sprintf("%q cluster machines collected with cluster label", cms.Len()), zap.String(omni.ClusterType, cluster.Metadata().ID()))

	for cm := range cms.All() {
		if err = helper.transformForMachine(ctx, r, cm.Metadata().ID(), logger); err != nil {
			logger.Debug("failed to transform cluster machine", zap.String("cluster", cluster.Metadata().ID()), zap.Error(err))

			return err
		}
	}

	return nil
}

func (helper clusterOperationStatusControllerHelper) transformForMachine(ctx context.Context, r controller.ReaderWriter, id resource.ID, logger *zap.Logger) error {
	cmc, err := safe.ReaderGetByID[*omni.ClusterMachineConfig](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			logger.Debug("cluster machine config not found", zap.String(omni.ClusterMachineConfigType, id))

			return nil // nothing to do yet
		}

		return err
	}

	if cmc.Metadata().Phase() == resource.PhaseTearingDown {
		_, err = helper.handleConfigTearingDown(ctx, cmc, r, logger)

		return err
	}

	return helper.transformForMachineRunning(ctx, cmc, r, logger)
}

func (helper clusterOperationStatusControllerHelper) transformForMachineRunning(ctx context.Context, cmc *omni.ClusterMachineConfig, r controller.ReaderWriter, logger *zap.Logger) error {
	if !cmc.Metadata().Finalizers().Has(helper.controllerName) {
		logger.Debug("cluster machine config doesn't have finalizer", zap.String(omni.ClusterMachineConfigType, cmc.Metadata().ID()))

		if err := r.AddFinalizer(ctx, cmc.Metadata(), helper.controllerName); err != nil {
			logger.Error("failed to add finalizer to cluster machine config", zap.String(omni.ClusterMachineConfigType, cmc.Metadata().ID()), zap.Error(err))

			return err
		}
	}

	mcgo, err := safe.ReaderGetByID[*omni.MachineConfigGenOptions](ctx, r, cmc.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		logger.Error("failed to fetch machine config gen options", zap.String(omni.MachineConfigGenOptionsType, cmc.Metadata().ID()), zap.Error(err))

		return err
	}

	var installImage *specs.InstallImage
	if mcgo != nil {
		installImage = mcgo.TypedSpec().Value.InstallImage
	}

	return safe.WriterModify(ctx, r, omni.NewClusterMachineExtendedConfig(resources.DefaultNamespace, cmc.Metadata().ID()), func(res *omni.ClusterMachineExtendedConfig) error {
		res.TypedSpec().Value.ConfigSpec = cmc.TypedSpec().Value
		res.TypedSpec().Value.InstallImage = installImage
		helpers.CopyAllLabels(cmc, res)
		helpers.CopyAllAnnotations(cmc, res)

		return nil
	})
}

func (helper clusterOperationStatusControllerHelper) finalizerRemoval(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, cluster *omni.Cluster) error {
	cms, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return fmt.Errorf("failed to get cluster machines for cluster %q: %w", cluster.Metadata().ID(), err)
	}

	logger.Debug(fmt.Sprintf("%q cluster machines collected with cluster label", cms.Len()), zap.String(omni.ClusterType, cluster.Metadata().ID()))

	allCMECsDestroyed := true

	for cm := range cms.All() {
		var destroyed bool
		if destroyed, err = helper.finalizerRemovalForMachine(ctx, cm.Metadata().ID(), r, logger); err != nil {
			return fmt.Errorf("failed to teardown cluster machine extended config for cluster machine %q: %w", cm.Metadata().ID(), err)
		}

		if !destroyed {
			allCMECsDestroyed = false
		}
	}

	if !allCMECsDestroyed {
		logger.Debug(fmt.Sprintf("not all %q are destroyed. skipping reconciliation.", omni.ClusterMachineExtendedConfigType))

		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("not all %q are destroyed yet", omni.ClusterMachineExtendedConfigType)
	}

	return nil
}

func (helper clusterOperationStatusControllerHelper) finalizerRemovalForMachine(ctx context.Context, id resource.ID, r controller.ReaderWriter, logger *zap.Logger) (destroyed bool, err error) {
	cmc, err := safe.ReaderGetByID[*omni.ClusterMachineConfig](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) { // we already removed our finalizer and this cmc got removed, continue
			logger.Debug("cluster machine config and it's finalizer have already been removed. skipping reconciliation.", zap.String(omni.ClusterMachineConfigType, id), zap.Error(err))

			return true, nil
		}

		logger.Error("failed to fetch cluster machine config", zap.String(omni.ClusterMachineConfigType, cmc.Metadata().ID()), zap.Error(err))

		return false, err
	}

	if cmc.Metadata().Phase() != resource.PhaseTearingDown { // cmc is not yet in the tearing down phase, we skip this cm, as we will come here again
		logger.Debug("cluster machine config is not in teardown phase yet. skipping extended config teardown", zap.String(omni.ClusterMachineConfigType, cmc.Metadata().ID()))

		return false, nil
	}

	return helper.handleConfigTearingDown(ctx, cmc, r, logger)
}

func (helper clusterOperationStatusControllerHelper) handleConfigTearingDown(
	ctx context.Context,
	cmc *omni.ClusterMachineConfig,
	r controller.ReaderWriter,
	logger *zap.Logger,
) (destroyed bool, err error) {
	if destroyed, err = helpers.TeardownAndDestroy(ctx, r, omni.NewClusterMachineExtendedConfig(resources.DefaultNamespace, cmc.Metadata().ID()).Metadata()); err != nil {
		logger.Error("failed to teardown/destroy cluster machine extended config", zap.String(omni.ClusterMachineExtendedConfigType, cmc.Metadata().ID()), zap.Error(err))

		return false, err
	}

	if !destroyed {
		logger.Debug("cluster machine extended config hasn't been destroyed yet", zap.String(omni.ClusterMachineExtendedConfigType, cmc.Metadata().ID()))

		return false, nil
	}

	if cmc.Metadata().Finalizers().Has(helper.controllerName) {
		if err = r.RemoveFinalizer(ctx, cmc.Metadata(), helper.controllerName); err != nil {
			logger.Error("failed to remove finalizer from cluster machine config", zap.String(omni.ClusterMachineConfigType, cmc.Metadata().ID()), zap.Error(err))

			return false, err
		}
	}

	return true, nil
}
