// Copyright (c) 2026 Sidero Labs, Inc.
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
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/cluster"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/clustermachine"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/schematic"
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
		if cm.Metadata().Finalizers().Has(schematic.ConfigurationControllerName) {
			if _, err = safe.StateUpdateWithConflicts(ctx, s, cm.Metadata(), func(res *omni.ClusterMachine) error {
				res.Metadata().Finalizers().Remove(schematic.ConfigurationControllerName)

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
		if sc.Metadata().Finalizers().Has("TalosUpgradeStatusController") {
			if _, err = safe.StateUpdateWithConflicts(ctx, s, sc.Metadata(), func(res *omni.SchematicConfiguration) error {
				res.Metadata().Finalizers().Remove("TalosUpgradeStatusController")

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

// changeImageFactoryAuthOwner adopts the ImageFactoryAuth resources into ImageFactoryAuthController.
//
// They used to be written by the Omni startup path with no owner, and the controller that now
// maintains them — and that also issues the access tokens they carry — cannot modify a
// resource it does not own.
func changeImageFactoryAuthOwner(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	auths, err := safe.ReaderListAll[*omni.ImageFactoryAuth](ctx, st)
	if err != nil {
		return err
	}

	for auth := range auths.All() {
		if auth.Metadata().Owner() == imagefactory.AuthControllerName {
			continue
		}

		if err = changeOwner(ctx, st, auth, imagefactory.AuthControllerName); err != nil {
			return err
		}
	}

	return nil
}

func createIdentityLastActiveForExistingIdentities(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	identities, err := safe.ReaderListAll[*authres.Identity](ctx, st)
	if err != nil {
		return err
	}

	var created int

	for identity := range identities.All() {
		identityLastActive := authres.NewIdentityLastActive(identity.Metadata().ID())

		if err = st.Create(ctx, identityLastActive); err != nil {
			if state.IsConflictError(err) {
				continue
			}

			return fmt.Errorf("failed to create IdentityLastActive for %q: %w", identity.Metadata().ID(), err)
		}

		created++
	}

	logger.Info("created IdentityLastActive resources for existing identities", zap.Int("created", created))

	return nil
}

func moveSchematicCacheToEphemeral(_ context.Context, _ state.State, _ *zap.Logger, _ migrationContext) error {
	// No-op: superseded by dropSchematicResource, which removes the Schematic
	// resource type from the state entirely.
	return nil
}

func dropSchematicResource(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	const schematicType = resource.Type("Schematics.omni.sidero.dev")

	for _, ns := range []resource.Namespace{resources.DefaultNamespace, resources.EphemeralNamespace} {
		list, err := st.List(ctx, resource.NewMetadata(ns, schematicType, "", resource.VersionUndefined))
		if err != nil {
			return err
		}

		logger.Info("destroying schematic resources", zap.String("namespace", ns), zap.Int("count", len(list.Items)))

		for _, item := range list.Items {
			if !item.Metadata().Finalizers().Empty() {
				if _, err = st.UpdateWithConflicts(ctx, item.Metadata(), func(res resource.Resource) error {
					for _, f := range *res.Metadata().Finalizers() {
						res.Metadata().Finalizers().Remove(f)
					}

					return nil
				}, state.WithUpdateOwner(item.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
					return fmt.Errorf("failed to strip finalizers from Schematic %q: %w", item.Metadata().ID(), err)
				}
			}

			if err = st.Destroy(ctx, item.Metadata(), state.WithDestroyOwner(item.Metadata().Owner())); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("failed to destroy Schematic %q: %w", item.Metadata().ID(), err)
			}
		}
	}

	return nil
}

func dropTalosUpgradeStatusFinalizers(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	logger.Info("dropping TalosUpgradeStatusController finalizers from ClusterMachine resources")

	return dropFinalizers[*omni.ClusterMachine](ctx, st, "TalosUpgradeStatusController")
}

func dropRedactedClusterMachineConfigFinalizers(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	logger.Info("dropping RedactedClusterMachineConfigController finalizers from ClusterMachineConfig resources")

	return dropFinalizers[*omni.ClusterMachineConfig](ctx, st, "RedactedClusterMachineConfigController")
}

func removeStaleIdentityLastActiveResources(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	identityLastActives, err := safe.ReaderListAll[*authres.IdentityLastActive](ctx, st)
	if err != nil {
		return err
	}

	identities, err := safe.ReaderListAll[*authres.Identity](ctx, st)
	if err != nil {
		return err
	}

	identitySet := make(map[resource.ID]struct{}, identities.Len())

	for identity := range identities.All() {
		identitySet[identity.Metadata().ID()] = struct{}{}
	}

	var removed int

	for ila := range identityLastActives.All() {
		if _, identityOk := identitySet[ila.Metadata().ID()]; identityOk { // exists, continue
			continue
		}

		// Strip any finalizers defensively before destroying.
		if !ila.Metadata().Finalizers().Empty() {
			if _, err = safe.StateUpdateWithConflicts(ctx, st, ila.Metadata(), func(res *authres.IdentityLastActive) error {
				for _, f := range *res.Metadata().Finalizers() {
					res.Metadata().Finalizers().Remove(f)
				}

				return nil
			}, state.WithUpdateOwner(ila.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
				return fmt.Errorf("failed to strip finalizers from IdentityLastActive %q: %w", ila.Metadata().ID(), err)
			}
		}

		if err = st.TeardownAndDestroy(ctx, ila.Metadata(), state.WithTeardownAndDestroyOwner(ila.Metadata().Owner())); err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to destroy IdentityLastActive %q: %w", ila.Metadata().ID(), err)
		}

		removed++
	}

	logger.Info("removed stale IdentityLastActive resources", zap.Int("removed", removed))

	return nil
}

func dropWorkloadProxyConfigPatches(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	configPatches, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, st)
	if err != nil {
		return err
	}

	ctrl, err := cluster.NewWorkloadProxyController(true)
	if err != nil {
		return err
	}

	controllerName := ctrl.ControllerName

	for patch := range configPatches.All() {
		if patch.Metadata().Owner() != controllerName {
			continue
		}

		if _, err = safe.StateUpdateWithConflicts(ctx, st, patch.Metadata(), func(r *omni.ConfigPatch) error {
			for _, f := range *r.Metadata().Finalizers() {
				r.Metadata().Finalizers().Remove(f)
			}

			return nil
		}, state.WithUpdateOwner(controllerName)); err != nil {
			return err
		}

		if err = st.Destroy(ctx, patch.Metadata(), state.WithDestroyOwner(controllerName)); err != nil {
			return err
		}
	}

	return nil
}

func setInitialUserFlagForExistingInstances(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	authConfig, err := safe.ReaderGetByID[*authres.Config](ctx, st, authres.ConfigID)
	if state.IsNotFoundError(err) {
		return nil
	}

	if err != nil {
		return err
	}

	publicKeys, err := safe.ReaderListAll[*authres.PublicKey](ctx, st)
	if err != nil {
		return err
	}

	if _, err = safe.StateUpdateWithConflicts(ctx, st, authConfig.Metadata(), func(res *authres.Config) error {
		res.TypedSpec().Value.HasInitialUser = publicKeys.Len() > 0

		return nil
	}); err != nil {
		return err
	}

	logger.Info("back-filled HasInitialUser flag from existing public keys")

	return nil
}

func makeMachineRequestsOwnerEmpty(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	machineRequests, err := safe.ReaderListAll[*infra.MachineRequest](ctx, st)
	if err != nil {
		return err
	}

	for machineRequest := range machineRequests.All() {
		if machineRequest.Metadata().Owner() == "" {
			continue
		}

		if err = changeOwner(ctx, st, machineRequest, ""); err != nil {
			return err
		}
	}

	return nil
}

// legacyInstallDiskPatchWeight is the historical weight of the generated install disk selection
// patches. It is copied here, because the shared constant goes away with the patch mechanism.
const legacyInstallDiskPatchWeight = 0

// legacyInstallDiskFromPatch returns the disk path carried by a generated install disk selection
// patch. It returns an empty string when the patch is not exactly one.
//
// A patch is recognized by the exact generated shape: the well-known ID formed from the cluster
// machine label, and a body containing nothing but the single install disk value. A patch at the
// well-known ID that was modified to carry anything else is treated as hand-written.
//
// This is a private copy of the recognition logic the template export used to perform, with an
// owner check on top, so that controller-owned resources can never match. The export-side code is
// deleted together with the patch mechanism, and migrations must not depend on living helpers.
//
// The whole recognition deliberately lives here instead of list-time filters. The owner cannot be
// filtered at list time anyway, and a one-shot migration gains nothing from splitting the exact
// shape across two places.
func legacyInstallDiskFromPatch(configPatch *omni.ConfigPatch) string {
	// the generated selection patches are user-owned, so anything else is not one of them
	if configPatch.Metadata().Owner() != "" {
		return ""
	}

	clusterMachine, ok := configPatch.Metadata().Labels().Get(omni.LabelClusterMachine)
	if !ok {
		return ""
	}

	expectedID := fmt.Sprintf("%03d-cm-%s-install-disk", legacyInstallDiskPatchWeight, clusterMachine)
	if configPatch.Metadata().ID() != expectedID {
		return ""
	}

	buffer, err := configPatch.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return ""
	}

	defer buffer.Free()

	var data map[string]any

	if err = yaml.Unmarshal(buffer.Data(), &data); err != nil {
		return ""
	}

	machine, ok := data["machine"].(map[string]any)
	if !ok {
		return ""
	}

	install, ok := machine["install"].(map[string]any)
	if !ok {
		return ""
	}

	if len(data) != 1 || len(machine) != 1 || len(install) != 1 {
		return ""
	}

	disk, ok := install["disk"].(string)
	if !ok {
		return ""
	}

	return disk
}

// machineInstallDiskConfigsFromPatches translates the generated install disk selection patches
// into MachineInstallDiskConfig resources and deletes them. The patches were written by the
// cluster creation UI and by cluster templates before the install disk became a dedicated
// resource. The translation is required, because such patches no longer influence the install
// disk (the resolved disk overrides them at config generation time). Without it, the user's explicit choice
// would silently turn into automatic selection.
//
// The migration is idempotent. The resource create is skipped when it already exists, and the
// patch deletion is the completion marker, so a re-run after a partial failure completes the
// remaining work.
func machineInstallDiskConfigsFromPatches(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	configPatches, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, st)
	if err != nil {
		return err
	}

	for configPatch := range configPatches.All() {
		disk := legacyInstallDiskFromPatch(configPatch)
		if disk == "" {
			continue
		}

		machineID, _ := configPatch.Metadata().Labels().Get(omni.LabelClusterMachine)

		installDiskConfig := omni.NewMachineInstallDiskConfig(machineID)
		installDiskConfig.TypedSpec().Value.Disk = disk

		// A patch of a template-managed cluster was written by the template, so the resource it
		// translates to must stay template-managed. Without the annotation, removing the install
		// section from the template after the upgrade would preserve the selection as a user
		// choice instead of returning the machine to automatic selection.
		//
		// The template-managed marker has to come from the cluster: template syncs never put it
		// on the patches themselves, only on the cluster. A missing cluster (e.g., deleted halfway
		// when the old Omni stopped) leaves the marker unknowable, and the selection is preserved
		// as a user choice, which loses nothing.
		if clusterName, ok := configPatch.Metadata().Labels().Get(omni.LabelCluster); ok {
			cluster, getErr := safe.ReaderGetByID[*omni.Cluster](ctx, st, clusterName)
			if getErr != nil && !state.IsNotFoundError(getErr) {
				return fmt.Errorf("failed to get the cluster %q of the install disk patch %q: %w", clusterName, configPatch.Metadata().ID(), getErr)
			}

			if cluster != nil {
				if _, managedByTemplates := cluster.Metadata().Annotations().Get(omni.ResourceManagedByClusterTemplates); managedByTemplates {
					installDiskConfig.Metadata().Annotations().Set(omni.ResourceManagedByClusterTemplates, "")
				}
			}
		}

		// Create, not an upsert: a conflict means a previous interrupted run already created the
		// resource, and it must be kept as is, not rewritten.
		if err = st.Create(ctx, installDiskConfig); err != nil && !state.IsConflictError(err) {
			return fmt.Errorf("failed to create the install disk config for machine %q: %w", machineID, err)
		}

		if _, err = safe.StateUpdateWithConflicts(ctx, st, configPatch.Metadata(), func(res *omni.ConfigPatch) error {
			for _, finalizer := range *res.Metadata().Finalizers() {
				res.Metadata().Finalizers().Remove(finalizer)
			}

			return nil
		}, state.WithExpectedPhaseAny()); err != nil {
			return fmt.Errorf("failed to drop the finalizers of the install disk patch %q: %w", configPatch.Metadata().ID(), err)
		}

		if err = st.Destroy(ctx, configPatch.Metadata()); err != nil {
			return fmt.Errorf("failed to destroy the install disk patch %q: %w", configPatch.Metadata().ID(), err)
		}

		logger.Info("translated the install disk patch into a machine install disk config",
			zap.String("machine", machineID), zap.String("disk", disk))
	}

	return nil
}
