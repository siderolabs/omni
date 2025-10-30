// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machineupgrade implements the Talos upgrade controller for Omni runtime.
package machineupgrade

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

type StatusController struct {
	*qtransform.QController[*omni.MachineStatus, *omni.MachineUpgradeStatus]
	imageFactoryClient ImageFactoryClient
	talosClientFactory TalosClientFactory
}

func NewStatusController(imageFactoryClient ImageFactoryClient, talosClientFactory TalosClientFactory) *StatusController {
	if talosClientFactory == nil {
		talosClientFactory = &talosCliFactory{}
	}

	ctrl := &StatusController{
		imageFactoryClient: imageFactoryClient,
		talosClientFactory: talosClientFactory,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.MachineUpgradeStatus]{
			Name: "MachineUpgradeStatusController",
			MapMetadataFunc: func(ms *omni.MachineStatus) *omni.MachineUpgradeStatus {
				return omni.NewMachineUpgradeStatus(ms.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MachineUpgradeStatus) *omni.MachineStatus {
				return omni.NewMachineStatus(resources.DefaultNamespace, status.Metadata().ID())
			},
			TransformFunc: ctrl.transform,
		},
		qtransform.WithExtraMappedInput[*omni.KernelArgs](qtransform.MapperSameID[*omni.MachineStatus]()),
	)

	return ctrl
}

//nolint:gocyclo,cyclop,maintidx
func (ctrl *StatusController) transform(ctx context.Context, r controller.Reader, logger *zap.Logger, ms *omni.MachineStatus, status *omni.MachineUpgradeStatus) error {
	helpers.SyncLabels(ms, status, omni.LabelCluster, omni.LabelMachineSet)

	status.TypedSpec().Value.IsMaintenance = ms.TypedSpec().Value.Maintenance

	schematicSpec := ms.TypedSpec().Value.Schematic
	if schematicSpec == nil {
		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = "schematic info is not available"
		status.TypedSpec().Value.Error = ""

		return nil
	}

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

	currentSchematic := *machineSchematic

	schematicID, err := currentSchematic.ID()
	if err != nil {
		return fmt.Errorf("failed to calculate schematic ID: %w", err)
	}

	if schematicID != schematicSpec.FullId {
		// when we calculate the schematic ID, we get a different result than machine reports. The image factory library version might be outdated.
		// Do not take any action to prevent undesired schematic changes/upgrades.
		logger.Error("schematic ID mismatch, skip upgrade", zap.String("reported", schematicSpec.FullId), zap.String("calculated", schematicID))

		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = ""
		status.TypedSpec().Value.Error = "schematic ID mismatch"

		return nil
	}

	kernelArgsRes, err := safe.ReaderGetByID[*omni.KernelArgs](ctx, r, ms.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting extra kernel args configuration: %w", err)
	}

	kernelArgs, kernelArgsOk, err := kernelargs.Calculate(ms, kernelArgsRes)
	if err != nil {
		return fmt.Errorf("error getting extra kernel args: %w", err)
	}

	if !kernelArgsOk {
		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = "extra kernel args are not yet initialized"
		status.TypedSpec().Value.Error = ""

		// if extra kernel args were not initialized yet by the MachineStatusController, we cannot determine the desired set of kernel args, skip
		return nil
	}

	desiredSchematic := currentSchematic
	desiredSchematic.Customization.ExtraKernelArgs = kernelArgs

	desiredSchematicID, err := desiredSchematic.ID()
	if err != nil {
		return fmt.Errorf("failed to compute desired schematic ID: %w", err)
	}

	status.TypedSpec().Value.SchematicId = desiredSchematicID

	talosVersion := ms.TypedSpec().Value.TalosVersion
	if talosVersion == "" {
		status.TypedSpec().Value.Phase = specs.MachineUpgradeStatusSpec_Unknown
		status.TypedSpec().Value.Status = "Talos version is not available"
		status.TypedSpec().Value.Error = ""

		return nil
	}

	schematicEqual, schematicEqualWithoutKernelArgs, err := ctrl.checkSchematicEquality(currentSchematic, desiredSchematic)
	if err != nil {
		return fmt.Errorf("failed to get schematic diff: %w", err)
	}

	// TODO: - We could centralize the maintenance mode upgrades in this controller and drop it from the management API.
	//       - Here, we would also manage the Talos version updates, and even extensions management.
	//       - For now, we hardcode "talosVersionEqual" to true, as we atm only handle schematic updates.
	//
	talosVersionEqual := true
	status.TypedSpec().Value.TalosVersion = talosVersion
	status.TypedSpec().Value.CurrentTalosVersion = talosVersion

	// Note: "platform" and "secure boot state" should probably never change in an install image, therefore, the following checks are enough
	installImagesEqual := schematicEqual && talosVersionEqual

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

	if schematicEqualWithoutKernelArgs {
		var updateSupported bool

		if updateSupported, err = kernelargs.UpdateSupported(ms); err != nil {
			return fmt.Errorf("failed to check if kernel args update is supported: %w", err)
		}

		if !updateSupported {
			status.TypedSpec().Value.Status = "kernel args are cannot be updated: machine is not booted with UKI and Talos version is < 1.12"
			status.TypedSpec().Value.Error = ""

			return nil
		}
	}

	ensuredSchematic, err := ctrl.imageFactoryClient.EnsureSchematic(ctx, desiredSchematic)
	if err != nil {
		return fmt.Errorf("failed to ensure schematic in image factory: %w", err)
	}

	if desiredSchematicID != ensuredSchematic.FullID {
		logger.Error("schematic ID returned from image factory does not match desired schematic ID",
			zap.String("desired", desiredSchematicID), zap.String("returned", ensuredSchematic.FullID))

		status.TypedSpec().Value.Status = ""
		status.TypedSpec().Value.Error = "schematic ID returned from image factory does not match desired schematic ID"

		return nil
	}

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

	installImageStr, err := installimage.Build(ctrl.imageFactoryClient.Host(), ms.Metadata().ID(), installImage)
	if err != nil {
		return fmt.Errorf("failed to build install image: %w", err)
	}

	logger.Info("built Talos upgrade image", zap.String("image", installImageStr))

	if err = ctrl.doUpgrade(ctx, ms, installImageStr); err != nil {
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

func (ctrl *StatusController) doUpgrade(ctx context.Context, machineStatus *omni.MachineStatus, installImage string) (retErr error) {
	talosClient, err := ctrl.talosClientFactory.New(ctx, machineStatus.TypedSpec().Value.ManagementAddress)
	if err != nil {
		return fmt.Errorf("failed to create Talos client: %w", err)
	}

	defer func() {
		if closeErr := talosClient.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("failed to close Talos client: %w", closeErr))
		}
	}()

	if _, err = talosClient.UpgradeWithOptions(ctx, client.WithUpgradeImage(installImage)); err != nil {
		return fmt.Errorf("failed to do Talos upgrade: %w", err)
	}

	return nil
}

func (ctrl *StatusController) checkSchematicEquality(currentSchematic, desiredSchematic schematic.Schematic) (equal, equalWithoutKernelArgs bool, err error) {
	kernelArgsMismatch := !slices.Equal(currentSchematic.Customization.ExtraKernelArgs, desiredSchematic.Customization.ExtraKernelArgs)

	currentSchematic.Customization.ExtraKernelArgs = desiredSchematic.Customization.ExtraKernelArgs

	currentMarshaled, err := currentSchematic.Marshal()
	if err != nil {
		return false, false, fmt.Errorf("failed to marshal current schematic: %w", err)
	}

	desiredMarshaled, err := desiredSchematic.Marshal()
	if err != nil {
		return false, false, fmt.Errorf("failed to marshal desired schematic: %w", err)
	}

	equalWithoutKernelArgs = bytes.Equal(currentMarshaled, desiredMarshaled)
	equal = equalWithoutKernelArgs && !kernelArgsMismatch

	return equal, equalWithoutKernelArgs, nil
}
