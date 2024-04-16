// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
)

var errMachineLocked = errors.New("machine locked")

// TalosUpgradeStatusController manages TalosUpgradeStatus performing a Talos upgrade.
//
// TalosUpgradeStatusController upgrades Kubernetes component versions in the cluster.
type TalosUpgradeStatusController = qtransform.QController[*omni.Cluster, *omni.TalosUpgradeStatus]

// TalosUpgradeStatusControllerName is the name of TalosUpgradeStatusController.
const TalosUpgradeStatusControllerName = "TalosUpgradeStatusController"

// NewTalosUpgradeStatusController initializes TalosUpgradeStatusController.
func NewTalosUpgradeStatusController() *TalosUpgradeStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.TalosUpgradeStatus]{
			Name: TalosUpgradeStatusControllerName,
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.TalosUpgradeStatus {
				return omni.NewTalosUpgradeStatus(resources.DefaultNamespace, cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(upgradeStatus *omni.TalosUpgradeStatus) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, upgradeStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, cluster *omni.Cluster, upgradeStatus *omni.TalosUpgradeStatus) error {
				if err := updateSchematicsFinalizers(ctx, r, cluster); err != nil {
					return err
				}

				clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
				if err != nil {
					return err
				}

				if err = cleanupResources(ctx, r, clusterMachines); err != nil {
					return err
				}

				machineSetNodes, err := safe.ReaderListAll[*omni.MachineSetNode](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
				if err != nil {
					return err
				}

				if err = populateEmptySchematics(ctx, r, cluster); err != nil {
					return err
				}

				return reconcileTalosUpdateStatus(ctx, r, logger, upgradeStatus, cluster, clusterMachines, machineSetNodes)
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, cluster *omni.Cluster) error {
				if err := updateSchematicsFinalizers(ctx, r, cluster); err != nil {
					return err
				}

				clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
				if err != nil {
					return err
				}

				if err = cleanupResources(ctx, r, clusterMachines); err != nil {
					return err
				}

				if clusterMachines.Len() != 0 {
					// cluster still has machines, skipping
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster %q still has machines", cluster.Metadata().ID())
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ *omni.TalosVersion) ([]resource.Pointer, error) {
				// reconcile all clusters TalosUpgradeStatus on TalosVersion changes
				clusters, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
				if err != nil {
					return nil, err
				}

				return safe.Map(clusters, func(cluster *omni.Cluster) (resource.Pointer, error) {
					return cluster.Metadata(), nil
				})
			},
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterStatus, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.ClusterMachineIdentity, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.MachineSetNode, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.ClusterMachine, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.MachineStatus, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.SchematicConfiguration, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, cmcs *omni.ClusterMachineConfigStatus) ([]resource.Pointer, error) {
				cm, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, cmcs.Metadata().ID())
				if err != nil {
					if state.IsNotFoundError(err) {
						return nil, nil
					}

					return nil, err
				}

				clusterName, ok := cm.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					return nil, nil
				}

				return []resource.Pointer{omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata()}, nil
			},
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.ClusterMachineTalosVersionType,
				Kind: controller.OutputShared,
			},
		),
	)
}

func updateMachines(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	upgradeStatus *omni.TalosUpgradeStatus,
	machinesToUpdate []*omni.ClusterMachineTalosVersion,
	machineSetNodes safe.List[*omni.MachineSetNode],
) (string, error) {
	var machineToUpdate *omni.ClusterMachineTalosVersion

	slices.SortStableFunc(machinesToUpdate, func(a, b *omni.ClusterMachineTalosVersion) int {
		return a.Metadata().Created().Compare(b.Metadata().Created())
	})

	lockedMachines := map[resource.ID]struct{}{}

	machineSetNodes.ForEach(func(machine *omni.MachineSetNode) {
		if _, locked := machine.Metadata().Annotations().Get(omni.MachineLocked); locked {
			lockedMachines[machine.Metadata().ID()] = struct{}{}
		}
	})

	for _, machine := range machinesToUpdate {
		machineToUpdate = machine

		if _, locked := lockedMachines[machineToUpdate.Metadata().ID()]; !locked {
			break
		}
	}

	id, err := getMachineName(ctx, r, machineToUpdate.Metadata().ID())
	if err != nil {
		return "", err
	}

	if _, locked := lockedMachines[machineToUpdate.Metadata().ID()]; locked {
		upgradeStatus.TypedSpec().Value.Status = "upgrade paused"
		upgradeStatus.TypedSpec().Value.Step = fmt.Sprintf("waiting for the machine %s to be unlocked", id)

		return "", errMachineLocked
	}

	if err := updateMachine(ctx, r, logger, machineToUpdate); err != nil {
		return "", err
	}

	return id, nil
}

func getMachineName(ctx context.Context, r controller.ReaderWriter, id resource.ID) (string, error) {
	identity, err := safe.ReaderGet[*omni.ClusterMachineIdentity](ctx, r, omni.NewClusterMachineIdentity(resources.DefaultNamespace, id).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return id, nil
		}

		return "", err
	}

	if identity.TypedSpec().Value.Nodename == "" {
		return id, nil
	}

	return identity.TypedSpec().Value.Nodename, nil
}

//nolint:gocognit,cyclop,gocyclo,maintidx
func reconcileTalosUpdateStatus(ctx context.Context, r controller.ReaderWriter,
	logger *zap.Logger,
	upgradeStatus *omni.TalosUpgradeStatus, cluster *omni.Cluster,
	clusterMachines safe.List[*omni.ClusterMachine],
	machineSetNodes safe.List[*omni.MachineSetNode],
) error {
	talosVersion := cluster.TypedSpec().Value.TalosVersion

	// if not reverting to the previous successful version, perform pre-checks on each step
	versionMismatch := talosVersion != upgradeStatus.TypedSpec().Value.LastUpgradeVersion

	var schematicUpdates bool

	if versionMismatch {
		// set the status to 'upgrading' if there is a version mismatch
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Upgrading
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = talosVersion
	} else {
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = ""
	}

	var (
		machinesToUpdate []*omni.ClusterMachineTalosVersion
		configsReady     = true
		err              error
	)

	clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](ctx, r, omni.NewClusterStatus(resources.DefaultNamespace, cluster.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	for iter := clusterMachines.Iterator(); iter.Next(); {
		machine := iter.Value()

		if machine.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		var schematicID string

		schematicID, err = getDesiredSchematic(ctx, r, machine)
		if err != nil {
			return err
		}

		var clusterMachineTalosVersion *omni.ClusterMachineTalosVersion

		// if we end up creating the ClusterMachineTalosVersion here, schematic won't be outdated, as we are setting it to the desired schematic ID
		clusterMachineTalosVersion, err = getOrCreateResource(ctx, r, machine, talosVersion, schematicID)
		if err != nil {
			return err
		}

		if !machine.Metadata().Finalizers().Has(TalosUpgradeStatusControllerName) {
			if err = r.AddFinalizer(ctx, machine.Metadata(), TalosUpgradeStatusControllerName); err != nil {
				return err
			}
		}

		var configStatus *omni.ClusterMachineConfigStatus

		configStatus, err = safe.ReaderGet[*omni.ClusterMachineConfigStatus](ctx, r, omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, machine.Metadata().ID()).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		schematicOutdated := clusterMachineTalosVersion.TypedSpec().Value.SchematicId != schematicID

		resourceNeedsUpdate := clusterMachineTalosVersion.TypedSpec().Value.TalosVersion != talosVersion || schematicOutdated

		schematicUpdates = clusterMachineTalosVersion.TypedSpec().Value.SchematicId != schematicID ||
			(configStatus != nil && configStatus.TypedSpec().Value.SchematicId != schematicID) ||
			schematicUpdates

		if schematicOutdated {
			upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Upgrading
		}

		clusterMachineTalosVersion.TypedSpec().Value.TalosVersion = talosVersion
		clusterMachineTalosVersion.TypedSpec().Value.SchematicId = schematicID

		switch {
		// if the machine doesn't have the config status or config sha, it means we can update it's version immediately
		// as the machine is not configured yet
		case configStatus == nil || configStatus.TypedSpec().Value.ClusterMachineConfigSha256 == "":
			if err = updateMachine(ctx, r, logger, clusterMachineTalosVersion); err != nil {
				return err
			}

			fallthrough
		case configStatus.TypedSpec().Value.LastConfigError != "":
			configsReady = false
		}

		if !configsReady {
			continue
		}

		if resourceNeedsUpdate || configStatus.TypedSpec().Value.TalosVersion != talosVersion ||
			configStatus.TypedSpec().Value.SchematicId != schematicID {
			machinesToUpdate = append(machinesToUpdate, clusterMachineTalosVersion)
		}
	}

	upgradeStatus.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

	if len(machinesToUpdate) == 0 {
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Done
		upgradeStatus.TypedSpec().Value.Status = ""
		upgradeStatus.TypedSpec().Value.Step = ""
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = ""
		upgradeStatus.TypedSpec().Value.LastUpgradeVersion = talosVersion

		// upgrade is done, so calculate the upgrade versions
		upgradeStatus.TypedSpec().Value.UpgradeVersions, err = talos.CalculateUpgradeVersions(
			ctx, r, upgradeStatus.TypedSpec().Value.LastUpgradeVersion, cluster.TypedSpec().Value.KubernetesVersion)
		if err != nil {
			return err
		}

		return nil
	}

	// upgrade is going, or failed, so clear the upgrade versions
	upgradeStatus.TypedSpec().Value.UpgradeVersions = nil

	if clusterStatus.TypedSpec().Value.Phase != specs.ClusterStatusSpec_RUNNING || !clusterStatus.TypedSpec().Value.Ready || !configsReady {
		// if the cluster set is not ready, and doing "normal" upgrade, set the status to 'waiting for the cluster set to be ready'
		if versionMismatch {
			if upgradeStatus.TypedSpec().Value.Phase == specs.TalosUpgradeStatusSpec_Upgrading &&
				upgradeStatus.TypedSpec().Value.Status == "" {
				upgradeStatus.TypedSpec().Value.Step = "update paused"
				upgradeStatus.TypedSpec().Value.Status = "waiting for the cluster to be ready"
			}

			return nil
		}
	}

	controlPlanes := xslices.Filter(machinesToUpdate, func(res *omni.ClusterMachineTalosVersion) bool {
		_, matches := res.Metadata().Labels().Get(omni.LabelControlPlaneRole)

		return matches
	})

	workers := xslices.Filter(machinesToUpdate, func(res *omni.ClusterMachineTalosVersion) bool {
		_, matches := res.Metadata().Labels().Get(omni.LabelWorkerRole)

		return matches
	})

	totalMachines := clusterMachines.Len()
	pendingMachines := totalMachines - len(machinesToUpdate) + 1

	if versionMismatch || schematicUpdates {
		upgradeStatus.TypedSpec().Value.Status = fmt.Sprintf("updating machines %d/%d", pendingMachines, totalMachines)
	} else {
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Reverting
		upgradeStatus.TypedSpec().Value.Status = fmt.Sprintf("reverting update %d/%d", pendingMachines, totalMachines)
	}

	for _, machines := range [][]*omni.ClusterMachineTalosVersion{controlPlanes, workers} {
		if len(machines) > 0 {
			machine, err := updateMachines(ctx, r, logger, upgradeStatus, machines, machineSetNodes)
			if err != nil {
				if errors.Is(err, errMachineLocked) {
					return nil
				}

				return err
			}

			upgradeStatus.TypedSpec().Value.Step = fmt.Sprintf("current machine %s", machine)

			break
		}
	}

	return nil
}

func getOrCreateResource(ctx context.Context, r controller.ReaderWriter, machine *omni.ClusterMachine, talosVersion, schematicID string) (*omni.ClusterMachineTalosVersion, error) {
	res := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, machine.Metadata().ID())

	clusterMachineTalosVersion, err := safe.ReaderGet[*omni.ClusterMachineTalosVersion](ctx, r, res.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			if err = createInitialTalosVersion(ctx, r, machine, talosVersion, schematicID); err != nil {
				return nil, err
			}

			return safe.ReaderGet[*omni.ClusterMachineTalosVersion](ctx, r, res.Metadata())
		}

		return nil, err
	}

	return clusterMachineTalosVersion, nil
}

func updateMachine(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, version *omni.ClusterMachineTalosVersion) error {
	return safe.WriterModify(
		ctx,
		r,
		version,
		func(res *omni.ClusterMachineTalosVersion) error {
			logger.Info("update desired Talos version or schematic",
				zap.String("machine", version.Metadata().ID()),
				zap.String("from_schematic", res.TypedSpec().Value.SchematicId),
				zap.String("to_schematic", version.TypedSpec().Value.SchematicId),
				zap.String("from_version", res.TypedSpec().Value.TalosVersion),
				zap.String("to_version", version.TypedSpec().Value.TalosVersion),
			)

			res.TypedSpec().Value = version.TypedSpec().Value

			return nil
		},
	)
}

func createInitialTalosVersion(ctx context.Context, r controller.ReaderWriter, machine *omni.ClusterMachine, talosVersion, schematicID string) error {
	res := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, machine.Metadata().ID())

	helpers.CopyAllLabels(machine, res)

	res.TypedSpec().Value.TalosVersion = talosVersion
	res.TypedSpec().Value.SchematicId = schematicID

	return r.Create(ctx, res)
}

func cleanupResources(ctx context.Context, r controller.ReaderWriter, clusterMachines safe.List[*omni.ClusterMachine]) error {
	return clusterMachines.ForEachErr(func(clusterMachine *omni.ClusterMachine) error {
		if clusterMachine.Metadata().Phase() == resource.PhaseRunning {
			return nil
		}

		if err := r.Destroy(ctx, omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata()); err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if clusterMachine.Metadata().Finalizers().Has(TalosUpgradeStatusControllerName) {
			return r.RemoveFinalizer(ctx, clusterMachine.Metadata(), TalosUpgradeStatusControllerName)
		}

		return nil
	})
}

func getDesiredSchematic(ctx context.Context, r controller.ReaderWriter, machine *omni.ClusterMachine) (string, error) {
	schematic, err := safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return "", err
	}

	if schematic != nil {
		return schematic.TypedSpec().Value.SchematicId, nil
	}

	ms, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, machine.Metadata().ID())
	if err != nil {
		return "", err
	}

	if ms.TypedSpec().Value.Schematic == nil {
		return "", nil
	}

	if ms.TypedSpec().Value.Schematic.InitialSchematic != "" {
		return ms.TypedSpec().Value.Schematic.InitialSchematic, nil
	}

	return ms.TypedSpec().Value.Schematic.Id, nil
}

// populateEmptySchematics iterates all cluster machine talos versions for the cluster and if they have an empty schematic, populates
// it with the value stored in the machine status resource.
func populateEmptySchematics(ctx context.Context, r controller.ReaderWriter, cluster *omni.Cluster) error {
	clusterMachineTalosVersions, err := safe.ReaderListAll[*omni.ClusterMachineTalosVersion](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
	))
	if err != nil {
		return err
	}

	return clusterMachineTalosVersions.ForEachErr(func(clusterMachineTalosVersion *omni.ClusterMachineTalosVersion) error {
		if clusterMachineTalosVersion.TypedSpec().Value.SchematicId != "" {
			return nil
		}

		machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, clusterMachineTalosVersion.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		schematic := machineStatus.TypedSpec().Value.Schematic
		if schematic == nil {
			return nil
		}

		return safe.WriterModify(ctx, r, clusterMachineTalosVersion, func(res *omni.ClusterMachineTalosVersion) error {
			res.TypedSpec().Value.SchematicId = schematic.InitialSchematic

			return nil
		})
	})
}

func updateSchematicsFinalizers(ctx context.Context, r controller.ReaderWriter, cluster *omni.Cluster) error {
	schematicConfigurations, err := safe.ReaderListAll[*omni.SchematicConfiguration](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
	))
	if err != nil {
		return err
	}

	return schematicConfigurations.ForEachErr(func(res *omni.SchematicConfiguration) error {
		if res.Metadata().Phase() == resource.PhaseTearingDown {
			return r.RemoveFinalizer(ctx, res.Metadata(), TalosUpgradeStatusControllerName)
		}

		if !res.Metadata().Finalizers().Has(TalosUpgradeStatusControllerName) {
			return r.AddFinalizer(ctx, res.Metadata(), TalosUpgradeStatusControllerName)
		}

		return nil
	})
}
