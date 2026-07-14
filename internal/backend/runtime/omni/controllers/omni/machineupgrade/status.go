// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machineupgrade implements the Talos upgrade controller for Omni runtime.
package machineupgrade

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

// LifecycleManager runs Talos's LifecycleService install/upgrade for a single machine, and hands out a cached client for the legacy fallback path.
type LifecycleManager interface {
	GetForMachine(ctx context.Context, machineID string) (*talos.Client, error)
	Run(ctx context.Context, op lifecycle.Operation, opts ...lifecycle.Option) error
	ImageFactoryHost() string
	TalosRegistry() string
}

type StatusController struct {
	*qtransform.QController[*omni.MachineStatus, *omni.MachineUpgradeStatus]
	lifecycleManager LifecycleManager
}

func NewStatusController(lifecycleManager LifecycleManager) *StatusController {
	ctrl := &StatusController{
		lifecycleManager: lifecycleManager,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.MachineUpgradeStatus]{
			Name: "MachineUpgradeStatusController",
			MapMetadataFunc: func(ms *omni.MachineStatus) *omni.MachineUpgradeStatus {
				return omni.NewMachineUpgradeStatus(ms.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MachineUpgradeStatus) *omni.MachineStatus {
				return omni.NewMachineStatus(status.Metadata().ID())
			},
			TransformFunc: ctrl.transform,
		},
		qtransform.WithExtraMappedInput[*omni.KernelArgs](qtransform.MapperSameID[*omni.MachineStatus]()),
		qtransform.WithExtraMappedInput[*omni.SchematicConfiguration](qtransform.MapperSameID[*omni.MachineStatus]()),
	)

	return ctrl
}

//nolint:gocyclo,cyclop
func (ctrl *StatusController) transform(ctx context.Context, r controller.Reader, logger *zap.Logger, ms *omni.MachineStatus, status *omni.MachineUpgradeStatus) error {
	helpers.SyncLabels(ms, status, omni.LabelCluster, omni.LabelMachineSet)

	status.TypedSpec().Value.IsMaintenance = ms.TypedSpec().Value.Maintenance

	if !ms.TypedSpec().Value.SchematicReady() {
		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = "schematic info is not available"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	schematicSpec := ms.TypedSpec().Value.Schematic

	status.TypedSpec().Value.CurrentSchematicId = schematicSpec.FullId

	if schematicSpec.Raw == "" {
		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = "the raw schematic is not available"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	machineSchematic, err := schematic.Unmarshal([]byte(schematicSpec.Raw))
	if err != nil {
		return fmt.Errorf("failed to unmarshal schematic: %w", err)
	}

	computedID, err := machineSchematic.ID()
	if err != nil {
		return fmt.Errorf("failed to calculate schematic ID: %w", err)
	}

	if computedID != schematicSpec.FullId {
		// when we calculate the schematic ID, we get a different result than machine reports. The image factory library version might be outdated.
		// Do not take any action to prevent undesired schematic changes/upgrades.
		logger.Error("schematic ID mismatch, skip upgrade", zap.String("reported", schematicSpec.FullId), zap.String("calculated", computedID))

		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = ""
		status.TypedSpec().Value.Error = "schematic ID mismatch"

		return nil
	}

	schematicConfiguration, err := safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, r, ms.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting schematic configuration: %w", err)
	}

	if schematicConfiguration == nil {
		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = "schematic configuration is not available"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	desiredSchematicID := schematicConfiguration.TypedSpec().Value.SchematicId
	status.TypedSpec().Value.SchematicId = desiredSchematicID

	talosVersion := ms.TypedSpec().Value.TalosVersion
	if talosVersion == "" {
		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = "Talos version is not available"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	// TODO: - We could centralize the maintenance mode upgrades in this controller and drop it from the management API.
	//       - Here, we would also manage the Talos version updates, and even extensions management.
	//       - For now, we hardcode "talosVersionEqual" to true, as we atm only handle schematic updates.
	//
	talosVersionEqual := true
	status.TypedSpec().Value.TalosVersion = talosVersion
	status.TypedSpec().Value.CurrentTalosVersion = talosVersion

	// Note: "platform" and "secure boot state" should probably never change in an install image, therefore, the following checks are enough
	installImagesEqual := schematicSpec.FullId == desiredSchematicID && talosVersionEqual

	if installImagesEqual {
		status.TypedSpec().Value.Status = "machine is up to date"
		status.TypedSpec().Value.Error = ""
		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_UpToDate

		return nil
	}

	// avoid excessive update calls
	if err = ctrl.checkCooldown(status, logger); err != nil {
		return err
	}

	status.TypedSpec().Value.Status = "pending Talos upgrade"
	status.TypedSpec().Value.Error = ""
	status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Pending

	// TODO: bring the non-maintenance upgrade logic in ClusterMachineConfigStatusController and the upgrades through the management API into this controller to centralize the Talos upgrades
	if !ms.TypedSpec().Value.Maintenance {
		status.TypedSpec().Value.Status = "not in maintenance mode"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	if omni.GetMachineStatusSystemDisk(ms) == "" {
		status.TypedSpec().Value.Status = "no system disk, Talos is not installed"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	securityState := ms.TypedSpec().Value.SecurityState

	platform := ms.TypedSpec().Value.GetPlatformMetadata().GetPlatform()
	if platform == "" {
		status.TypedSpec().Value.Status = "platform metadata is not available"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	if securityState == nil {
		status.TypedSpec().Value.Status = "security state is not available"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	installImage := &specs.MachineConfigGenOptionsSpec_InstallImage{
		TalosVersion:         talosVersion,
		SchematicId:          desiredSchematicID,
		SchematicInitialized: true,
		Platform:             platform,
		SecurityState:        securityState,
	}

	schematicMismatch := schematicSpec.FullId != desiredSchematicID

	if err = ctrl.upgrade(ctx, logger, ms, installImage, schematicMismatch); err != nil {
		if talos.IsClientNotReadyError(err) {
			status.TypedSpec().Value.Status = "Talos client is not yet ready: " + err.Error()
			status.TypedSpec().Value.Error = ""

			return nil
		}

		return err
	}

	logger.Info("Talos upgrade initiated")

	status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Upgrading
	status.TypedSpec().Value.Status = "Talos upgrade initiated"
	status.TypedSpec().Value.Error = ""

	// Requeue to check if the upgrade was succeeded and re-trigger it if it wasn't.
	return controller.NewRequeueInterval(time.Minute)
}

func (ctrl *StatusController) checkCooldown(status *omni.MachineUpgradeStatus, logger *zap.Logger) error {
	if status.TypedSpec().Value.Phase != specs.MachineUpgradeStatusSpec_Upgrading {
		return nil
	}

	lastUpdated := status.Metadata().Created()
	if status.Metadata().Updated().After(lastUpdated) {
		lastUpdated = status.Metadata().Updated()
	}

	const updateCooldownPeriod = 2 * time.Minute

	remainingTime := updateCooldownPeriod - time.Since(lastUpdated)
	if remainingTime <= 0 {
		return nil
	}

	logger.Debug("Talos upgrade is already in progress, wait for cooldown period before re-attempting",
		zap.Duration("since-last-update", time.Since(lastUpdated)),
		zap.Duration("cooldown-period", updateCooldownPeriod))

	return controller.NewRequeueInterval(remainingTime)
}

// upgrade runs Talos's LifecycleService.Upgrade for machines that support it (Talos 1.13+),
// falling back to the deprecated MachineService.Upgrade for older ones.
func (ctrl *StatusController) upgrade(
	ctx context.Context, logger *zap.Logger, ms *omni.MachineStatus, installImage *specs.MachineConfigGenOptionsSpec_InstallImage, schematicMismatch bool,
) error {
	switch op := lifecycle.DecideOp(ms, installImage, schematicMismatch, false); op {
	case lifecycle.OpMaintenanceUpgrade:
		return ctrl.lifecycleUpgrade(ctx, logger, ms, installImage)
	case lifecycle.OpLegacyUpgrade:
		return ctrl.legacyUpgrade(ctx, logger, ms, installImage)
	case lifecycle.OpNone, lifecycle.OpMaintenanceInstall, lifecycle.OpClusterUpgrade:
		return fmt.Errorf("unreachable lifecycle op %d for machine %q", op, ms.Metadata().ID())
	default:
		return fmt.Errorf("unknown lifecycle op %d for machine %q", op, ms.Metadata().ID())
	}
}

// lifecycleUpgrade upgrades the machine via Talos's LifecycleService.Upgrade.
func (ctrl *StatusController) lifecycleUpgrade(ctx context.Context, logger *zap.Logger, ms *omni.MachineStatus, installImage *specs.MachineConfigGenOptionsSpec_InstallImage) error {
	machineID := ms.Metadata().ID()

	operation := lifecycle.Operation{
		MachineID:     machineID,
		MachineStatus: ms,
		Version:       installImage.TalosVersion,
		InstallImage:  installImage,
		Kind:          lifecycle.KindUpgrade,
	}

	err := ctrl.lifecycleManager.Run(ctx, operation, lifecycle.WithProgress(func(msg string) {
		logger.Debug("upgrade progress", zap.String("machine", machineID), zap.String("message", msg))
	}))

	if errors.Is(err, lifecycle.ErrAlreadyInFlight) {
		logger.Info("upgrade already in flight", zap.String("machine", machineID))

		return controller.NewRequeueInterval(lifecycle.RetryInterval)
	}

	return err
}

// legacyUpgrade upgrades the machine via the deprecated MachineService.Upgrade, for Talos versions older than 1.13.
func (ctrl *StatusController) legacyUpgrade(ctx context.Context, logger *zap.Logger, ms *omni.MachineStatus, installImage *specs.MachineConfigGenOptionsSpec_InstallImage) error {
	machineID := ms.Metadata().ID()

	installImageStr, err := installimage.Build(ctrl.lifecycleManager.ImageFactoryHost(), machineID, installImage, ctrl.lifecycleManager.TalosRegistry())
	if err != nil {
		return fmt.Errorf("failed to build install image: %w", err)
	}

	logger.Info("built Talos upgrade image", zap.String("image", installImageStr))

	talosClient, err := ctrl.lifecycleManager.GetForMachine(ctx, machineID)
	if err != nil {
		return fmt.Errorf("failed to get talos client: %w", err)
	}

	//nolint:staticcheck
	if _, err = talosClient.UpgradeWithOptions(ctx, client.WithUpgradeImage(installImageStr)); err != nil {
		return fmt.Errorf("failed to do Talos upgrade: %w", err)
	}

	return nil
}
