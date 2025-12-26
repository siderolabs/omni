// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/clustermachine"
)

func moveClusterTaintFromResourceToLabel(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	clusterTaints, err := safe.ReaderListAll[*omni.ClusterTaint](ctx, st)
	if err != nil {
		return err
	}

	for taint := range clusterTaints.All() {
		_, err = safe.StateUpdateWithConflicts(ctx, st, omni.NewClusterStatus(taint.Metadata().ID()).Metadata(), func(res *omni.ClusterStatus) error {
			res.Metadata().Labels().Set(omni.LabelClusterTaintedByBreakGlass, "")

			return nil
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(omnictrl.ClusterStatusControllerName))
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}
		// cluster status does not exist, taint is dangling, just remove it

		if err = st.TeardownAndDestroy(ctx, taint.Metadata()); err != nil {
			return err
		}
	}

	return err
}

func dropExtraInputFinalizers(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	logger.Info("dropping extra finalizers from MachineSetStatus resources")

	if err := dropFinalizers[*omni.MachineSetStatus](ctx, st, "ClusterDestroyStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineSetStatus resources: %w", err)
	}

	logger.Info("dropping extra finalizers from ClusterMachineStatus resources")

	if err := dropFinalizers[*omni.ClusterMachineStatus](ctx, st, "ClusterDestroyStatusController", "MachineSetDestroyStatusController", "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from ClusterMachineStatus resources: %w", err)
	}

	logger.Info("dropping extra finalizers from Link resources")

	if err := dropFinalizers[*siderolink.Link](ctx, st, "BMCConfigController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from Link resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineSetNode resources")

	if err := dropFinalizers[*omni.MachineSetNode](ctx, st, "ClusterMachineStatusController", "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineSetNode resources: %w", err)
	}

	logger.Info("dropping extra finalizers from ClusterMachine resources")

	if err := dropFinalizers[*omni.ClusterMachine](ctx, st, "MachineStatusSnapshotController", "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from ClusterMachine resources: %w", err)
	}

	logger.Info("dropping extra finalizers from InfraMachineConfig resources")

	if err := dropFinalizers[*omni.InfraMachineConfig](ctx, st, "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from InfraMachineConfig resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineExtensions resources")

	if err := dropFinalizers[*omni.MachineExtensions](ctx, st, "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineExtensions resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineStatus resources")

	if err := dropFinalizers[*omni.MachineStatus](ctx, st, "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineStatus resources: %w", err)
	}

	logger.Info("dropping extra finalizers from NodeUniqueToken resources")

	if err := dropFinalizers[*siderolink.NodeUniqueToken](ctx, st, "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from NodeUniqueToken resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineStatusSnapshot resources")

	if err := dropFinalizers[*omni.MachineStatusSnapshot](ctx, st, "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineStatusSnapshot resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineLabels resources")

	if err := dropFinalizers[*omni.MachineLabels](ctx, st, "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineLabels resources: %w", err)
	}

	logger.Info("dropping extra finalizers from infra.MachineStatus resources")

	if err := dropFinalizers[*infra.MachineStatus](ctx, st, "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from infra.MachineStatus resources: %w", err)
	}

	return nil
}

func moveInfraProviderAnnotationsToLabels(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	for _, resType := range []resource.Type{siderolink.LinkType, omni.MachineType} {
		kind := resource.NewMetadata(resources.DefaultNamespace, resType, "", resource.VersionUndefined)

		list, err := st.List(ctx, kind)
		if err != nil {
			return err
		}

		for _, item := range list.Items {
			id, ok := item.Metadata().Annotations().Get(omni.LabelInfraProviderID)
			if !ok {
				continue
			}

			if _, err = st.UpdateWithConflicts(ctx, item.Metadata(), func(res resource.Resource) error {
				res.Metadata().Labels().Set(omni.LabelInfraProviderID, id)
				res.Metadata().Annotations().Delete(omni.LabelInfraProviderID)

				return nil
			}, state.WithUpdateOwner(item.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
				return err
			}
		}
	}

	return nil
}

func dropSchematicConfigFinalizerFromClusterMachines(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, s)
	if err != nil {
		return err
	}

	for cm := range list.All() {
		if cm.Metadata().Finalizers().Has(omnictrl.SchematicConfigurationControllerName) {
			if _, err = safe.StateUpdateWithConflicts(ctx, s, cm.Metadata(), func(res *omni.ClusterMachine) error {
				res.Metadata().Finalizers().Remove(omnictrl.SchematicConfigurationControllerName)

				return nil
			}, state.WithUpdateOwner(cm.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
				return err
			}
		}
	}

	return nil
}

func dropTalosUpgradeStatusFinalizersFromSchematicConfigs(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.ReaderListAll[*omni.SchematicConfiguration](ctx, s)
	if err != nil {
		return err
	}

	for sc := range list.All() {
		if sc.Metadata().Finalizers().Has(omnictrl.TalosUpgradeStatusControllerName) {
			if _, err = safe.StateUpdateWithConflicts(ctx, s, sc.Metadata(), func(res *omni.SchematicConfiguration) error {
				res.Metadata().Finalizers().Remove(omnictrl.TalosUpgradeStatusControllerName)

				return nil
			}, state.WithUpdateOwner(sc.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
				return err
			}
		}
	}

	return nil
}

func makeMachineSetNodesOwnerEmpty(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	machineSetNodes, err := safe.ReaderListAll[*omni.MachineSetNode](ctx, st)
	if err != nil {
		return err
	}

	for machineSetNode := range machineSetNodes.All() {
		if machineSetNode.Metadata().Owner() != omnictrl.NewMachineSetNodeController().ControllerName {
			continue
		}

		machineSetNode.Metadata().Labels().Set(omni.LabelManagedByMachineSetNodeController, "")

		if err = changeOwner(ctx, st, machineSetNode, ""); err != nil {
			return err
		}
	}

	return nil
}

func changeClusterMachineConfigPatchesOwner(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	clusterMachineConfigPatches, err := safe.ReaderListAll[*omni.ClusterMachineConfigPatches](ctx, st)
	if err != nil {
		return err
	}

	controllerName := clustermachine.NewConfigPatchesController().ControllerName

	for cmcp := range clusterMachineConfigPatches.All() {
		if err = changeOwner(ctx, st, cmcp, controllerName); err != nil {
			return err
		}
	}

	return nil
}
