// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package talosupgrade contains controllers related to Talos upgrade process.
package talosupgrade

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
)

// TalosUpgradeStatusController manages TalosUpgradeStatus performing a Talos upgrade.
//
// TalosUpgradeStatusController manages Talos OS (Talos) version upgrades in the cluster.
type TalosUpgradeStatusController struct {
	*qtransform.QController[*omni.Cluster, *omni.TalosUpgradeStatus]
}

// TalosUpgradeStatusControllerName is the name of TalosUpgradeStatusController.
const TalosUpgradeStatusControllerName = "TalosUpgradeStatusController"

type outdatedMachines struct {
	all       map[string]*omni.ClusterMachine
	upgrading []string
}

// NewStatusController initializes TalosUpgradeStatusController.
func NewStatusController() *TalosUpgradeStatusController {
	ctrl := &TalosUpgradeStatusController{}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.TalosUpgradeStatus]{
			Name: TalosUpgradeStatusControllerName,
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.TalosUpgradeStatus {
				return omni.NewTalosUpgradeStatus(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(upgradeStatus *omni.TalosUpgradeStatus) *omni.Cluster {
				return omni.NewCluster(upgradeStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, cluster *omni.Cluster, upgradeStatus *omni.TalosUpgradeStatus) error {
				clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
				if err != nil {
					return err
				}

				outdatedMachines, err := ctrl.filterOutdated(ctx, r, cluster, clusterMachines)
				if err != nil {
					return err
				}

				if err := ctrl.reconcileStatus(ctx, r, clusterMachines, outdatedMachines, cluster, upgradeStatus); err != nil {
					return err
				}

				if err := ctrl.updateRolloutState(ctx, r, outdatedMachines, cluster, upgradeStatus); err != nil {
					return err
				}

				if err := ctrl.reconcileTalosVersions(ctx, r, cluster, clusterMachines); err != nil {
					return err
				}

				return ctrl.reconcileUpgradeVersions(ctx, r, cluster, upgradeStatus)
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, cluster *omni.Cluster) error {
				clusterMachineTalosVersions, err := safe.ReaderListAll[*omni.ClusterMachineTalosVersion](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
				if err != nil {
					return err
				}

				ready, err := helpers.TeardownAndDestroyAll(ctx, r, clusterMachineTalosVersions.Pointers())
				if err != nil {
					return err
				}

				if !ready {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster machine Talos versions are still tearing down")
				}

				ready, err = helpers.TeardownAndDestroy(ctx, r, omni.NewUpgradeRollout(cluster.Metadata().ID()).Metadata())
				if err != nil {
					return err
				}

				if !ready {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("upgrade rollout %q tearing down", omni.NewUpgradeRollout(cluster.Metadata().ID()).Metadata().ID())
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.TalosVersion](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				// reconcile all cluster TalosUpgradeStatus on TalosVersion changes
				clusters, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
				if err != nil {
					return nil, err
				}

				return slices.Collect(clusters.Pointers()), nil
			},
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.SchematicConfiguration](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.MachinePendingUpdates](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfigStatus](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterConfigVersion](
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSet](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.ClusterMachineTalosVersionType,
				Kind: controller.OutputExclusive,
			},
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.UpgradeRolloutType,
				Kind: controller.OutputExclusive,
			},
		),
	)

	return ctrl
}

//nolint:gocyclo,cyclop,gocognit
func (ctrl *TalosUpgradeStatusController) reconcileStatus(ctx context.Context, r controller.ReaderWriter,
	clusterMachines safe.List[*omni.ClusterMachine], outdatedMachines *outdatedMachines,
	cluster *omni.Cluster, upgradeStatus *omni.TalosUpgradeStatus,
) error {
	machineSetNodes, err := safe.ReaderListAll[*omni.MachineSetNode](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
	if err != nil {
		return err
	}

	machineSetNodeMap := xslices.ToMap(slices.Collect(machineSetNodes.All()), func(r *omni.MachineSetNode) (string, *omni.MachineSetNode) {
		return r.Metadata().ID(), r
	})

	talosVersion := cluster.TypedSpec().Value.TalosVersion

	// if not reverting to the previous successful version, perform pre-checks on each step
	versionMismatch := talosVersion != upgradeStatus.TypedSpec().Value.LastUpgradeVersion
	schematicUpdates := false

	pendingUpdates, err := safe.ReaderListAll[*omni.MachinePendingUpdates](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
	))
	if err != nil {
		return err
	}

	var (
		pendingMachines       int
		lockedPendingMachines int
	)

	for pendingUpdate := range pendingUpdates.All() {
		if pendingUpdate.TypedSpec().Value.Upgrade == nil {
			continue
		}

		if pendingUpdate.TypedSpec().Value.Upgrade.FromSchematic == "" ||
			pendingUpdate.TypedSpec().Value.Upgrade.FromVersion == "" {
			continue
		}

		msn, ok := machineSetNodeMap[pendingUpdate.Metadata().ID()]
		if ok {
			_, locked := msn.Metadata().Annotations().Get(omni.MachineLocked)
			if locked {
				lockedPendingMachines++
			}
		}

		pendingMachines++

		if pendingUpdate.TypedSpec().Value.Upgrade.FromSchematic != pendingUpdate.TypedSpec().Value.Upgrade.ToSchematic {
			schematicUpdates = true

			break
		}
	}

	// exit early if there are no pending machines and no machines currently upgrading, setting the status to 'done'
	if pendingMachines == 0 && len(outdatedMachines.all) == 0 {
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Done
		upgradeStatus.TypedSpec().Value.Status = ""
		upgradeStatus.TypedSpec().Value.Step = ""
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = ""
		upgradeStatus.TypedSpec().Value.LastUpgradeVersion = talosVersion

		return nil
	}

	if versionMismatch {
		// set the status to 'upgrading' if there is a version mismatch
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Upgrading
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = talosVersion
	} else {
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = ""
	}

	totalMachines := clusterMachines.Len()

	currentMachine := min(totalMachines-pendingMachines+1, totalMachines)

	switch {
	case !versionMismatch && schematicUpdates:
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_UpdatingMachineSchematics

		fallthrough
	case versionMismatch || schematicUpdates:
		if pendingMachines == 0 {
			upgradeStatus.TypedSpec().Value.Status = "starting"
		} else {
			upgradeStatus.TypedSpec().Value.Status = fmt.Sprintf("updating machines %d/%d", currentMachine, totalMachines)
		}
	default:
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Reverting
		upgradeStatus.TypedSpec().Value.Status = fmt.Sprintf("reverting update %d/%d", currentMachine, totalMachines)
	}

	if len(outdatedMachines.upgrading) > 0 {
		upgradeStatus.TypedSpec().Value.Step = fmt.Sprintf("current %s: %s", pluralize.NewClient().Pluralize("machine", len(outdatedMachines.upgrading), false),
			strings.Join(outdatedMachines.upgrading, ", "),
		)

		return nil
	}

	if lockedPendingMachines == pendingMachines && lockedPendingMachines > 0 {
		upgradeStatus.TypedSpec().Value.Step = "all machines are locked"
		upgradeStatus.TypedSpec().Value.Status = "waiting for machines to be unlocked"

		return nil
	}

	upgradeStatus.TypedSpec().Value.Step = "no machines are currently upgrading"

	return nil
}

func (ctrl *TalosUpgradeStatusController) updateRolloutState(ctx context.Context, r controller.ReaderWriter,
	outdatedMachines *outdatedMachines, cluster *omni.Cluster, upgradeStatus *omni.TalosUpgradeStatus,
) error {
	clusterMachineStatuses, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
	))
	if err != nil {
		return err
	}

	var (
		controlPlanesUpgrading bool
		notReadyMachinesCounts int32
	)

	for clusterMachineStatus := range clusterMachineStatuses.All() {
		if clusterMachineStatus.TypedSpec().Value.Stage != specs.ClusterMachineStatusSpec_RUNNING ||
			!clusterMachineStatus.TypedSpec().Value.Ready {
			notReadyMachinesCounts++
		}
	}

	for _, clusterMachine := range outdatedMachines.all {
		if _, ok := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
			controlPlanesUpgrading = true

			break
		}
	}

	return safe.WriterModify(ctx, r, omni.NewUpgradeRollout(cluster.Metadata().ID()), func(res *omni.UpgradeRollout) error {
		machineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
		if err != nil {
			return err
		}

		upgradeQuotas := make(map[string]int32, machineSets.Len())

		for machineSet := range machineSets.All() {
			quota := int32(omni.GetParallelism(machineSet.TypedSpec().Value.UpgradeStrategy, machineSet.TypedSpec().Value.UpgradeStrategyConfig, 1))

			// if reverting, ignore not ready machines
			if upgradeStatus.TypedSpec().Value.Phase != specs.TalosUpgradeStatusSpec_Reverting {
				quota -= notReadyMachinesCounts
			}

			if quota <= 0 {
				continue
			}

			_, isControlPlane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole)
			if isControlPlane || !controlPlanesUpgrading {
				upgradeQuotas[machineSet.Metadata().ID()] = quota
			}
		}

		res.TypedSpec().Value.MachineSetsUpgradeQuota = upgradeQuotas

		if len(res.TypedSpec().Value.MachineSetsUpgradeQuota) == 0 && upgradeStatus.TypedSpec().Value.Phase != specs.TalosUpgradeStatusSpec_Done {
			upgradeStatus.TypedSpec().Value.Step = "waiting for the cluster to be ready"
		}

		return nil
	})
}

func (ctrl *TalosUpgradeStatusController) reconcileTalosVersions(ctx context.Context, r controller.ReaderWriter, cluster *omni.Cluster,
	clusterMachines safe.List[*omni.ClusterMachine],
) error {
	clusterMachineTalosVersions, err := safe.ReaderListAll[*omni.ClusterMachineTalosVersion](ctx, r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return err
	}

	visited := make(map[string]struct{}, clusterMachines.Len())

	for clusterMachine := range clusterMachines.All() {
		if clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		var schematicConfiguration *omni.SchematicConfiguration

		schematicConfiguration, err = safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, r, clusterMachine.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if schematicConfiguration.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		visited[schematicConfiguration.Metadata().ID()] = struct{}{}

		if schematicConfiguration.TypedSpec().Value.TalosVersion != cluster.TypedSpec().Value.TalosVersion {
			continue
		}

		if err = safe.WriterModify(ctx, r, omni.NewClusterMachineTalosVersion(schematicConfiguration.Metadata().ID()), func(res *omni.ClusterMachineTalosVersion) error {
			res.TypedSpec().Value.SchematicId = schematicConfiguration.TypedSpec().Value.SchematicId
			res.TypedSpec().Value.TalosVersion = schematicConfiguration.TypedSpec().Value.TalosVersion

			helpers.CopyAllLabels(schematicConfiguration, res)

			return nil
		}); err != nil {
			return err
		}
	}

	toDestroy := xiter.Filter(func(r resource.Pointer) bool {
		_, ok := visited[r.ID()]

		return !ok
	}, clusterMachineTalosVersions.Pointers())

	if _, err = helpers.TeardownAndDestroyAll(ctx, r, toDestroy); err != nil {
		return err
	}

	return nil
}

func (ctrl *TalosUpgradeStatusController) reconcileUpgradeVersions(ctx context.Context, r controller.ReaderWriter, cluster *omni.Cluster, upgradeStatus *omni.TalosUpgradeStatus) error {
	if upgradeStatus.TypedSpec().Value.Phase != specs.TalosUpgradeStatusSpec_Done {
		// upgrade is going, or failed, so clear the upgrade versions
		upgradeStatus.TypedSpec().Value.UpgradeVersions = nil

		return nil
	}

	clusterConfigVersion, err := safe.ReaderGetByID[*omni.ClusterConfigVersion](ctx, r, cluster.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	// upgrade is done, so calculate the upgrade versions
	upgradeStatus.TypedSpec().Value.UpgradeVersions, err = talos.CalculateUpgradeVersions(
		ctx, r, clusterConfigVersion.TypedSpec().Value.Version, upgradeStatus.TypedSpec().Value.LastUpgradeVersion, cluster.TypedSpec().Value.KubernetesVersion)

	return err
}

func (ctrl *TalosUpgradeStatusController) filterOutdated(ctx context.Context, r controller.ReaderWriter,
	cluster *omni.Cluster, clusterMachines safe.List[*omni.ClusterMachine],
) (*outdatedMachines, error) {
	res := &outdatedMachines{
		all: make(map[string]*omni.ClusterMachine, clusterMachines.Len()),
	}

	for clusterMachine := range clusterMachines.All() {
		schematicConfiguration, err := safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, r, clusterMachine.Metadata().ID())
		if err != nil {
			// this means the machine is not yet fully added to the cluster
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		clusterMachineConfigStatus, err := safe.ReaderGetByID[*omni.ClusterMachineConfigStatus](ctx, r, clusterMachine.Metadata().ID())
		if err != nil {
			// this means the machine is not yet fully added to the cluster
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		// ignore machines with missing TalosVersion or SchematicId, as they are not fully configured yet
		// so it's not outdated, but rather in the process of being installed
		if clusterMachineConfigStatus.TypedSpec().Value.TalosVersion == "" || clusterMachineConfigStatus.TypedSpec().Value.SchematicId == "" {
			continue
		}

		if clusterMachine.Metadata().Finalizers().Has(machineconfig.UpgradeFinalizer) {
			res.all[clusterMachine.Metadata().ID()] = clusterMachine
			res.upgrading = append(res.upgrading, clusterMachine.Metadata().ID())

			continue
		}

		// mark the machine as outdated if the schematic was generated for the outdated Talos version
		// that happens async in the controller, so we need to make sure it's in sync before proceeding
		if schematicConfiguration.TypedSpec().Value.TalosVersion != cluster.TypedSpec().Value.TalosVersion {
			res.all[clusterMachine.Metadata().ID()] = clusterMachine

			continue
		}

		if clusterMachineConfigStatus.TypedSpec().Value.TalosVersion != schematicConfiguration.TypedSpec().Value.TalosVersion ||
			clusterMachineConfigStatus.TypedSpec().Value.SchematicId != schematicConfiguration.TypedSpec().Value.SchematicId {
			res.all[clusterMachine.Metadata().ID()] = clusterMachine
		}
	}

	return res, nil
}
