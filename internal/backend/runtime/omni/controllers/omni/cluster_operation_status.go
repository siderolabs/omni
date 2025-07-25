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

func (helper clusterOperationStatusControllerHelper) transform(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, cluster *omni.Cluster, _ *omni.ClusterOperationStatus) error {
	cms, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return fmt.Errorf("failed to get cluster machines for cluster %q: %w", cluster.Metadata().ID(), err)
	}

	for cm := range cms.All() {
		if _, err = helper.processClusterMachine(ctx, r, cm); err != nil {
			return err
		}
	}

	return nil
}

func (helper clusterOperationStatusControllerHelper) finalizerRemoval(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, cluster *omni.Cluster) error {
	cms, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return fmt.Errorf("failed to get cluster machines for cluster %q: %w", cluster.Metadata().ID(), err)
	}

	allCMECsDestroyed := true

	for cm := range cms.All() {
		var destroyed bool

		if destroyed, err = helper.processClusterMachine(ctx, r, cm); err != nil {
			return err
		}

		if !destroyed {
			allCMECsDestroyed = false
		}
	}

	if !allCMECsDestroyed {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("not all %q are destroyed yet", omni.ClusterMachineExtendedConfigType)
	}

	return nil
}

func (helper clusterOperationStatusControllerHelper) processClusterMachine(ctx context.Context, r controller.ReaderWriter, cm *omni.ClusterMachine) (destroyed bool, err error) {
	cmc, err := safe.ReaderGetByID[*omni.ClusterMachineConfig](ctx, r, cm.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("extended machine config is not found")
		}

		return false, err
	}

	if cmc.Metadata().Phase() == resource.PhaseTearingDown {
		if destroyed, err = helper.processClusterMachineConfigTearingDown(ctx, cmc, r); err != nil {
			return false, fmt.Errorf("failed to process cluster machine config %q: %w", cmc.Metadata().ID(), err)
		}

		return destroyed, nil
	}

	if !cmc.Metadata().Finalizers().Has(helper.controllerName) {
		if err = r.AddFinalizer(ctx, cmc.Metadata(), helper.controllerName); err != nil {
			return false, err
		}
	}

	mcgo, err := safe.ReaderGetByID[*omni.MachineConfigGenOptions](ctx, r, cm.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return false, err
	}

	var installImage *specs.InstallImage
	if mcgo != nil {
		installImage = mcgo.TypedSpec().Value.InstallImage
	}

	return false, safe.WriterModify(ctx, r, omni.NewClusterMachineExtendedConfig(resources.DefaultNamespace, cmc.Metadata().ID()), func(res *omni.ClusterMachineExtendedConfig) error {
		res.TypedSpec().Value.ConfigSpec = cmc.TypedSpec().Value
		res.TypedSpec().Value.InstallImage = installImage
		helpers.CopyAllLabels(cmc, res)
		helpers.CopyAllAnnotations(cmc, res)

		return nil
	})
}

func (helper clusterOperationStatusControllerHelper) processClusterMachineConfigTearingDown(
	ctx context.Context,
	cmc *omni.ClusterMachineConfig,
	r controller.ReaderWriter,
) (destroyed bool, err error) {
	if destroyed, err = helpers.TeardownAndDestroy(ctx, r, omni.NewClusterMachineExtendedConfig(resources.DefaultNamespace, cmc.Metadata().ID()).Metadata()); err != nil {
		return false, err
	}

	if !destroyed {
		return false, nil
	}

	if cmc.Metadata().Finalizers().Has(helper.controllerName) {
		if err = r.RemoveFinalizer(ctx, cmc.Metadata(), helper.controllerName); err != nil {
			return false, err
		}
	}

	return true, nil
}
