// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/uncached"
)

// LifecycleOp identifies which upgrade/install path the controller should follow for a given machine.
type LifecycleOp int

const (
	// LifecycleOpNone means on-disk Talos already matches the install image; no action needed.
	LifecycleOpNone LifecycleOp = iota

	// LifecycleOpLegacyUpgrade is an in-place upgrade via MachineService.Upgrade.
	LifecycleOpLegacyUpgrade

	// LifecycleOpMaintenanceInstall is LifecycleService.Install for a maintenance machine with no Talos on disk.
	LifecycleOpMaintenanceInstall

	// LifecycleOpMaintenanceUpgrade is LifecycleService.Upgrade for a maintenance machine that has Talos on disk.
	LifecycleOpMaintenanceUpgrade

	// LifecycleOpClusterUpgrade is LifecycleService.Upgrade for an in-cluster machine (Talos 1.13+), with cordon/drain/reboot orchestrated by Omni.
	LifecycleOpClusterUpgrade
)

// ReconciliationContext describes all related data for one reconciliation call of the machine config status controller.
type ReconciliationContext struct {
	machineStatus         *omni.MachineStatus
	machineConfig         *omni.ClusterMachineConfig
	machineConfigStatus   *omni.ClusterMachineConfigStatus
	machineStatusSnapshot *omni.MachineStatusSnapshot
	clusterMachine        *omni.ClusterMachine
	installImage          *specs.MachineConfigGenOptionsSpec_InstallImage

	lastConfigError       string
	installDisk           string
	bootID                string
	redactedMachineConfig []byte

	configUpdatesAllowed bool
	locked               bool
	lifecycleOp          LifecycleOp
}

// hasPendingLifecycleOperation returns true when an upgrade/install action needs to run before config can be applied.
func (rc *ReconciliationContext) hasPendingLifecycleOperation() bool {
	return rc.lifecycleOp != LifecycleOpNone
}

func checkClusterReady(ctx context.Context, r controller.Reader, machineConfig *omni.ClusterMachineConfig) (bool, error) {
	clusterName, ok := machineConfig.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return false, nil
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster doesn't exist")
		}

		return false, err
	}

	if _, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked); locked && cluster.Metadata().Phase() == resource.PhaseRunning {
		return true, nil
	}

	return false, nil
}

func checkMachineStatus(ctx context.Context, r controller.Reader, machineStatus *omni.MachineStatus) error {
	if !machineStatus.TypedSpec().Value.Connected {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %q is not connected", machineStatus.Metadata().ID())
	}

	if !machineStatus.TypedSpec().Value.SchematicReady() {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %q schematic is not ready", machineStatus.Metadata().ID())
	}

	// if the machine is managed by a static infra provider, we need to ensure that the infra machine is ready to use
	if _, isManagedByStaticInfraProvider := machineStatus.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider); isManagedByStaticInfraProvider {
		var infraMachineStatus *infra.MachineStatus

		infraMachineStatus, err := safe.ReaderGetByID[*infra.MachineStatus](ctx, r, machineStatus.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is managed by a static infra provider but the infra machine status is not found: %w", err)
			}

			return fmt.Errorf("failed to get infra machine status %q: %w", machineStatus.Metadata().ID(), err)
		}

		if !infraMachineStatus.TypedSpec().Value.ReadyToUse {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is managed by static infra provider but is not ready to use")
		}
	}

	return nil
}

func (rc *ReconciliationContext) ID() string {
	return rc.machineConfig.Metadata().ID()
}

// BuildReconciliationContext is the COSI reader dependent method to build the reconciliation context.
//
//nolint:gocognit,gocyclo,cyclop
func BuildReconciliationContext(ctx context.Context, r controller.Reader,
	machineConfig *omni.ClusterMachineConfig, machineConfigStatus *omni.ClusterMachineConfigStatus,
) (*ReconciliationContext, error) {
	desiredConfig, err := machineConfig.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return nil, fmt.Errorf("failed to read desired config: %w", err)
	}

	defer desiredConfig.Free()

	machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, r, machineConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q machine set node not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, err
	}

	rc := &ReconciliationContext{
		machineConfig:       machineConfig,
		machineConfigStatus: machineConfigStatus,
	}

	_, locked := machineSetNode.Metadata().Annotations().Get(omni.MachineLocked)
	// we also check config SHA not being empty here as we don't want to block the initial config creation
	rc.locked = locked && machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 != ""

	rc.lastConfigError = machineConfig.TypedSpec().Value.GenerationError

	if rc.lastConfigError != "" {
		return rc, nil
	}

	config, err := configloader.NewFromBytes(desiredConfig.Data())
	if err != nil {
		return nil, fmt.Errorf("failed to decode desired config: %w", err)
	}

	rc.redactedMachineConfig, err = config.RedactSecrets(x509.Redacted).EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return nil, fmt.Errorf("failed to redact secrets: %w", err)
	}

	if !rc.locked {
		if rc.locked, err = checkClusterReady(ctx, r, machineConfig); err != nil {
			return nil, err
		}
	}

	rc.machineStatusSnapshot, err = safe.ReaderGetByID[*omni.MachineStatusSnapshot](
		ctx, r,
		machineConfig.Metadata().ID(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q machine status snapshot not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get machine status snapshot %q: %w", machineConfig.Metadata().ID(), err)
	}

	rc.machineStatus, err = safe.ReaderGetByID[*omni.MachineStatus](ctx, r, machineConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q machine status not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get machine status %q: %w", machineConfig.Metadata().ID(), err)
	}

	if err = checkMachineStatus(ctx, r, rc.machineStatus); err != nil {
		return nil, err
	}

	genOptions, err := safe.ReaderGetByID[*omni.MachineConfigGenOptions](ctx, r, machineConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q machine config gen options not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get install image %q: %w", machineConfig.Metadata().ID(), err)
	}

	rc.installImage = genOptions.TypedSpec().Value.InstallImage
	if rc.installImage == nil {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q install image not found", machineConfig.Metadata().ID())
	}

	rc.installDisk = genOptions.TypedSpec().Value.InstallDisk

	schematicMismatch := false

	// Invalid schematic means the machine was not provisioned via image factory.
	// Skip schematic comparison — schematic plays no role in upgrade decisions for these machines.
	if !rc.machineStatus.TypedSpec().Value.Schematic.Invalid {
		schematicMismatch = machineConfigStatus.TypedSpec().Value.SchematicId != rc.installImage.SchematicId ||
			rc.machineStatus.TypedSpec().Value.Schematic.FullId != rc.installImage.SchematicId
	}

	talosVersionMismatch := strings.TrimLeft(rc.machineStatus.TypedSpec().Value.TalosVersion, "v") != machineConfigStatus.TypedSpec().Value.TalosVersion ||
		machineConfigStatus.TypedSpec().Value.TalosVersion != rc.installImage.TalosVersion

	machineSetName, ok := machineConfig.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return nil, errors.New("failed to get machine set name from the machine config resource")
	}

	// A stale allow value could apply a config while updates are blocked.
	machineSetConfigStatus, err := safe.ReaderGetByID[*omni.MachineSetConfigStatus](ctx, uncached.Reader(r), machineSetName)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	rc.configUpdatesAllowed = true

	if machineSetConfigStatus != nil {
		rc.configUpdatesAllowed = machineSetConfigStatus.TypedSpec().Value.ConfigUpdatesAllowed
	}

	rc.clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machineConfig.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	rc.bootID = rc.machineStatusSnapshot.TypedSpec().Value.BootId

	rc.lifecycleOp = DecideLifecycleOp(rc.machineStatus, rc.installImage, schematicMismatch, talosVersionMismatch)
	if rc.lifecycleOp == LifecycleOpMaintenanceInstall && rc.installDisk == "" {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q install disk is not yet selected", machineConfig.Metadata().ID())
	}

	return rc, nil
}

// DecideLifecycleOp picks the upgrade/install path from the live machine state. Exported for tests.
func DecideLifecycleOp(machineStatus *omni.MachineStatus, installImage *specs.MachineConfigGenOptionsSpec_InstallImage, schematicMismatch, talosVersionMismatch bool) LifecycleOp {
	hasSystemDisk := omni.GetMachineStatusSystemDisk(machineStatus) != ""

	machineVersion, machineSupportsLifecycle := omni.ParseTalosVersionLifecycleSupport(machineStatus.TypedSpec().Value.TalosVersion)
	targetVersion, targetSupportsLifecycle := omni.ParseTalosVersionLifecycleSupport(installImage.TalosVersion)

	// The LifecycleService paths (Talos 1.13+ both ends) decide from the live version/schematic, not the
	// mismatch flags, so a stale status can't trigger a spurious first-reconcile upgrade.
	if machineSupportsLifecycle && targetSupportsLifecycle {
		if machineStatus.TypedSpec().Value.Maintenance {
			switch {
			case hasSystemDisk && (!machineVersion.EQ(targetVersion) || schematicDiffers(machineStatus, installImage)):
				return LifecycleOpMaintenanceUpgrade
			case !hasSystemDisk && (machineVersion.Major != targetVersion.Major || machineVersion.Minor != targetVersion.Minor):
				// Cross-minor without a disk: config-apply only installs within the same minor, so install explicitly.
				return LifecycleOpMaintenanceInstall
			default:
				// Already at target, or same-minor with no disk where config-apply installs it for us.
				return LifecycleOpNone
			}
		}

		if hasSystemDisk {
			if !machineVersion.EQ(targetVersion) || schematicDiffers(machineStatus, installImage) {
				return LifecycleOpClusterUpgrade
			}

			return LifecycleOpNone
		}
	}

	// Older machine or target: the legacy path, gated on the config-status mismatch flags.
	if !schematicMismatch && !talosVersionMismatch {
		return LifecycleOpNone
	}

	if hasSystemDisk {
		return LifecycleOpLegacyUpgrade
	}

	// No Talos on disk and not eligible for a maintenance install: Omni has no way to install Talos on this machine today, so nothing is done.
	return LifecycleOpNone
}

// schematicDiffers reports whether the machine's schematic differs from the target's. An invalid schematic
// (not provisioned via image factory) reports false.
func schematicDiffers(machineStatus *omni.MachineStatus, installImage *specs.MachineConfigGenOptionsSpec_InstallImage) bool {
	machineSchematic := machineStatus.TypedSpec().Value.GetSchematic()

	return !machineSchematic.GetInvalid() && machineSchematic.GetFullId() != installImage.SchematicId
}
