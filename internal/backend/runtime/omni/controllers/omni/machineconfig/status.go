// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machineconfig implements the Talos config apply controller for Omni runtime.
package machineconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/diff"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	talosutils "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/uncached"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

const (
	// ConfigUpdateFinalizer is set on the ClusterMachine when this controller acquires the config change lock.
	// By counting the machines with the finalizer we can tell how many machines are being updated simultaneously.
	ConfigUpdateFinalizer     = "ConfigUpdatePendingFinalizer"
	UpgradeFinalizer          = "UpgradePendingFinalizer"
	gracefulResetAttemptCount = 4
	etcdLeaveAttemptsLimit    = 2
	maintenanceCheckAttempts  = 5
)

// LifecycleManager owns the node-side install/upgrade work. The controller decides timing.
type LifecycleManager interface {
	CheckAlive(ctx context.Context, machineID string) error
	GetForMachine(ctx context.Context, machineID string) (*talos.Client, error)
	Run(ctx context.Context, op lifecycle.Operation, opts ...lifecycle.Option) error
	FinalizeReboot(ctx context.Context, opts ...lifecycle.Option) error
	ImageFactoryHost() string
	TalosRegistry() string
}

// StatusController manages the ClusterMachineConfigStatus resource lifecycle.
//
// StatusController applies the generated machine config on each corresponding machine.
type StatusController struct {
	*qtransform.QController[*omni.ClusterMachineConfig, *omni.ClusterMachineConfigStatus]
	ongoingResets    *ongoingResets
	lifecycleManager LifecycleManager
	acquireLockMu    sync.Mutex
}

// NewStatusController initializes StatusController.
func NewStatusController(lifecycleManager LifecycleManager) *StatusController {
	ongoingResets := &ongoingResets{
		statuses: map[string]*resetStatus{},
	}

	ctrl := &StatusController{
		ongoingResets:    ongoingResets,
		lifecycleManager: lifecycleManager,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachineConfig, *omni.ClusterMachineConfigStatus]{
			Name: "ClusterMachineConfigStatusController",
			MapMetadataFunc: func(machineConfig *omni.ClusterMachineConfig) *omni.ClusterMachineConfigStatus {
				return omni.NewClusterMachineConfigStatus(machineConfig.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineConfigStatus *omni.ClusterMachineConfigStatus) *omni.ClusterMachineConfig {
				return omni.NewClusterMachineConfig(machineConfigStatus.Metadata().ID())
			},
			TransformExtraOutputFunc:        ctrl.reconcileRunning,
			FinalizerRemovalExtraOutputFunc: ctrl.reconcileTearingDown,
		},
		qtransform.WithConcurrency(8),
		qtransform.WithExtraMappedInput[*omni.Machine](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineStatus](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*infra.MachineStatus](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, md controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				clusterName, ok := md.Labels().Get(omni.LabelCluster)
				if !ok {
					return nil, nil
				}

				clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachineConfig](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelCluster, clusterName),
				))

				return slices.Collect(clusterMachines.Pointers()), err
			},
		),
		qtransform.WithExtraMappedInput[*omni.MachineStatusSnapshot](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineConfigGenOptions](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.NodeForceDestroyRequest](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.TalosConfig](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetConfigStatus](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, md controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				machines, err := safe.ReaderListAll[*omni.ClusterMachineConfig](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, md.ID())))
				if err != nil {
					return nil, err
				}

				return slices.Collect(machines.Pointers()), nil
			},
		),
		qtransform.WithExtraMappedInput[*omni.Cluster](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineIdentity](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterStatus](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedDestroyReadyInput[*omni.MachinePendingUpdates](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.UpgradeRollout](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraOutputs(controller.Output{
			Type: omni.NodeForceDestroyRequestType,
			Kind: controller.OutputShared,
		}, controller.Output{
			Type: omni.MachinePendingUpdatesType,
			Kind: controller.OutputExclusive,
		}),
	)

	return ctrl
}

//nolint:gocognit,gocyclo,cyclop
func (ctrl *StatusController) reconcileRunning(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	machineConfig *omni.ClusterMachineConfig, machineConfigStatus *omni.ClusterMachineConfigStatus,
) error {
	rc, err := BuildReconciliationContext(ctx, r, machineConfig, machineConfigStatus)
	if err != nil {
		if xerrors.TagIs[qtransform.SkipReconcileTag](err) {
			logger.Warn("status update skipped", zap.Error(err))
		}

		return err
	}

	configChanged, err := ctrl.computePendingUpdates(ctx, r, rc)
	if err != nil {
		return fmt.Errorf("failed to compute pending updates: %w", err)
	}

	if rc.locked {
		logger.Info("operations locked for machine")

		return nil
	}

	if rc.lastConfigError != "" {
		machineConfigStatus.TypedSpec().Value.LastConfigError = rc.lastConfigError

		logger.Info("config generation error", zap.String("error", rc.lastConfigError))

		return nil
	}

	// For invalid schematic machines, clear any stale schematic ID directly
	// instead of going through upgrade() which would make unnecessary Talos API calls.
	if rc.machineStatus.TypedSpec().Value.Schematic.Invalid && machineConfigStatus.TypedSpec().Value.SchematicId != "" {
		machineConfigStatus.TypedSpec().Value.SchematicId = ""
	}

	if err = ctrl.reconcileUpgrade(ctx, logger, r, rc); err != nil {
		return err
	}

	stage := rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage()
	if stage == machineapi.MachineStatusEvent_BOOTING || stage == machineapi.MachineStatusEvent_RUNNING {
		if err = ctrl.deleteUpgradeMetaKey(ctx, logger, r, rc); err != nil {
			return err
		}
	}

	buffer, err := machineConfig.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return fmt.Errorf("failed to get uncompressed config: %w", err)
	}

	defer buffer.Free()

	shaSum := sha256.Sum256(buffer.Data())
	shaSumString := hex.EncodeToString(shaSum[:])

	mode := machineapi.ApplyConfigurationRequest_NO_REBOOT

	// Re-apply when the confirmed config differs from the desired one (sha mismatch), or when the
	// config we last pushed to the machine differs from the desired one. The second case covers a
	// reboot-requiring change reverted before the machine confirms it: such a push is committed to
	// the machine but its sha is recorded only after the machine comes back, so a revert in that
	// window leaves the recorded sha matching the desired config. Comparing the last pushed config
	// lets us notice the machine still runs the reverted change and re-apply, instead of treating it
	// as in sync.
	if machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 != shaSumString || configChanged {
		mode, err = ctrl.applyConfig(ctx, logger, r, rc)
		if err != nil {
			grpcSt := client.Status(err)
			if grpcSt != nil && grpcSt.Code() == codes.InvalidArgument {
				machineConfigStatus.TypedSpec().Value.LastConfigError = grpcSt.Message()

				return nil
			}

			if errors.Is(err, errAcquireConfigLock) {
				logger.Info("failed to acquire config apply lock, another operation is ongoing", zap.Error(err))

				return nil
			}

			return fmt.Errorf("failed to apply config to machine '%s': %w", machineConfig.Metadata().ID(), err)
		}
	}

	// ClusterMachineConfig might receive an update that causes its version to change while not affecting the final MachineConfig therefore calculated hashsum.
	// Update ClusterMachineConfigVersion to reflect the actual version of the ClusterMachineConfig regardless of hash comparison.
	machineConfigStatus.TypedSpec().Value.ClusterMachineConfigVersion = machineConfig.Metadata().Version().String()

	if err = machineConfigStatus.TypedSpec().Value.SetUncompressedData(rc.redactedMachineConfig); err != nil {
		return err
	}

	if mode != machineapi.ApplyConfigurationRequest_NO_REBOOT {
		return nil
	}

	helpers.CopyLabels(machineConfig, machineConfigStatus, omni.LabelMachineSet, omni.LabelCluster, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

	machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 = shaSumString

	machineConfigStatus.TypedSpec().Value.LastConfigError = ""

	if _, err = helpers.TeardownAndDestroy(ctx, r, omni.NewMachinePendingUpdates(machineConfig.Metadata().ID()).Metadata()); err != nil {
		return err
	}

	if err = ctrl.releaseConfigUpdateLock(ctx, r, rc.clusterMachine); err != nil {
		return err
	}

	logger.Debug("released config update lock")

	return nil
}

func (ctrl *StatusController) reconcileTearingDown(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, machineConfig *omni.ClusterMachineConfig) error {
	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machineConfig.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	ready, err := helpers.TeardownAndDestroy(ctx, r, omni.NewMachinePendingUpdates(machineConfig.Metadata().ID()).Metadata())
	if err != nil {
		return err
	}

	if !ready {
		return nil
	}

	if err = ctrl.releaseConfigUpdateLock(ctx, r, clusterMachine); err != nil {
		return err
	}

	if err = ctrl.releaseUpgradeLock(ctx, r, clusterMachine); err != nil {
		return err
	}

	// perform reset of the node
	if err = ctrl.reset(ctx, logger, r, machineConfig); err != nil {
		return err
	}

	if err = r.Destroy(ctx, omni.NewNodeForceDestroyRequest(machineConfig.Metadata().ID()).Metadata(), controller.WithOwner("")); err != nil {
		if !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to destroy NodeForceDestroyRequest %q: %w", machineConfig.Metadata().ID(), err)
		}
	} else {
		logger.Info("destroyed NodeForceDestroyRequest")
	}

	// delete ongoing resets information if the machine was reset
	ctrl.ongoingResets.deleteStatus(machineConfig.Metadata().ID())

	return nil
}

// reconcileUpgrade brings the machine to its desired Talos version before its config is
// applied, and keeps the upgrade and config-update locks from ever being held at the same
// time. It returns nil once the machine is in sync and its config can be applied.
//
// The upgrade and config-update locks must never be held together by the same machine: that
// is what allows a lock-ordering inversion to deadlock the whole machine set, with one
// machine holding the config-update lock while waiting for the upgrade lock and another doing
// the opposite, leaving the set stuck until Omni is restarted. To avoid it:
//   - while the machine is parked waiting for a free upgrade slot, the config-update lock is
//     released, so it does not pin a config rollout slot and starve machines that only need a
//     config change. The lock is re-acquired later when the machine applies its config.
//   - once the machine is in sync, the upgrade lock is released before the config-apply phase.
func (ctrl *StatusController) reconcileUpgrade(
	ctx context.Context,
	logger *zap.Logger,
	r controller.ReaderWriter,
	rc *ReconciliationContext,
) error {
	// The legacy path doesn't use the reboot marker, so clear it lest a stale ID debounce a later operation.
	if rc.lifecycleOp == lifecycle.OpLegacyUpgrade {
		rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId = ""
	}

	if rc.hasPendingLifecycleOperation() {
		inSync, err := ctrl.upgrade(ctx, logger, r, rc)
		if err != nil {
			// Only release the config-update lock when the machine is specifically blocked
			// waiting for a free upgrade slot. It will not apply its config until it upgrades,
			// so holding the slot just starves machines that only need a config change. On other
			// (transient) upgrade failures the lock is kept, so a machine still completing its
			// config reboot is not disturbed.
			if errors.Is(err, errAcquireUpgradeLock) {
				if releaseErr := ctrl.releaseConfigUpdateLock(ctx, r, rc.clusterMachine); releaseErr != nil {
					return fmt.Errorf("failed to release config update lock: %w", releaseErr)
				}
			}

			return err
		}

		if !inSync {
			logger.Info("the machine talos version is out of sync, the config is not applied", zap.String("machine", rc.ID()))

			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine talos version is out of sync: %s", rc.ID())
		}
	} else {
		// The cached view can read None while the machine we triggered is still mid-reboot. Applying then
		// loses the config, so wait for the boot ID to change before clearing the marker and applying.
		if preRebootBootID := rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId; preRebootBootID != "" && rc.bootID == preRebootBootID {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine to reboot before applying config: %s", rc.ID())
		}

		// LifecycleOpNone here doesn't always mean "confirmed at target": for a maintenance machine with no
		// system disk yet, DecideLifecycleOp returns None to let the imminent config-apply perform the
		// install itself, so the live version is still the pre-install one, not what will end up on disk.
		// Only record it once the machine actually has Talos on disk.
		if omni.GetMachineStatusSystemDisk(rc.machineStatus) != "" {
			// Record the version so the status reflects an already-at-target machine that ran no upgrade path.
			rc.machineConfigStatus.TypedSpec().Value.TalosVersion = strings.TrimLeft(rc.machineStatus.TypedSpec().Value.TalosVersion, "v")

			if rc.machineStatus.TypedSpec().Value.GetSchematic().GetInvalid() {
				rc.machineConfigStatus.TypedSpec().Value.SchematicId = ""
			} else {
				rc.machineConfigStatus.TypedSpec().Value.SchematicId = rc.installImage.SchematicId
			}
		}
	}

	// Finalize before releasing the upgrade lock so the next machine can't start until this one is fully back.
	if err := ctrl.finalizeReboot(ctx, r, rc); err != nil {
		return err
	}

	if err := ctrl.releaseUpgradeLock(ctx, r, rc.clusterMachine); err != nil {
		return fmt.Errorf("failed to release upgrade lock: %w", err)
	}

	return nil
}

// finalizeReboot uncordons the node, then clears the reboot marker. The marker is shared with the
// maintenance paths, which never cordon, but Uncordon is a harmless no-op on an uncordoned node.
func (ctrl *StatusController) finalizeReboot(ctx context.Context, r controller.ReaderWriter, rc *ReconciliationContext) error {
	if rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId == "" {
		return nil
	}

	// A machine still in maintenance was never a Kubernetes node, so there is nothing to uncordon.
	if !rc.machineStatus.TypedSpec().Value.Maintenance {
		clusterName, ok := rc.clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
		if ok {
			nodeName, err := ctrl.getNodeName(ctx, r, rc.ID())
			if err != nil {
				return err
			}

			if nodeName != "" {
				if err = ctrl.lifecycleManager.FinalizeReboot(ctx, lifecycle.WithUncordon(clusterName, nodeName)); err != nil {
					// Keep PreRebootBootId so the next reconcile retries the uncordon.
					return controller.NewRequeueError(fmt.Errorf("failed to finalize reboot for machine %q: %w", rc.ID(), err), lifecycle.RetryInterval)
				}
			}
		}
	}

	rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId = ""

	return nil
}

func (ctrl *StatusController) upgrade(ctx context.Context, logger *zap.Logger, r controller.ReaderWriter, rc *ReconciliationContext) (bool, error) {
	switch rc.lifecycleOp {
	case lifecycle.OpNone:
		return true, nil

	case lifecycle.OpMaintenanceInstall, lifecycle.OpMaintenanceUpgrade:
		return ctrl.runMaintenanceLifecycle(ctx, logger, r, rc)

	case lifecycle.OpClusterUpgrade:
		return ctrl.runClusterLifecycle(ctx, logger, r, rc)

	case lifecycle.OpLegacyUpgrade:
		fallthrough
	default:
		return ctrl.legacyUpgrade(ctx, logger, r, rc)
	}
}

func (ctrl *StatusController) legacyUpgrade(inputCtx context.Context, logger *zap.Logger, r controller.ReaderWriter, rc *ReconciliationContext) (bool, error) {
	// use short timeout for all API calls except upgrade to quickly skip "dead" nodes
	ctx, cancel := context.WithTimeout(inputCtx, 5*time.Second)
	defer cancel()

	var maintenance bool

	//nolint:exhaustive
	switch rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage() {
	case machineapi.MachineStatusEvent_MAINTENANCE:
		maintenance = true
	case machineapi.MachineStatusEvent_BOOTING:
	case machineapi.MachineStatusEvent_RUNNING:
	default:
		return false, nil
	}

	if rc.installImage.TalosVersion == "" {
		return false, xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine '%s' does not have talos version", rc.ID()))
	}

	installed, err := ctrl.checkInstalledImage(ctx, rc)
	if err != nil {
		return false, err
	}

	if installed.atTarget {
		rc.machineConfigStatus.TypedSpec().Value.TalosVersion = installed.version
		rc.machineConfigStatus.TypedSpec().Value.SchematicId = installed.schematic

		return true, nil
	}

	image, err := installimage.Build(ctrl.lifecycleManager.ImageFactoryHost(), rc.ID(), rc.installImage, ctrl.lifecycleManager.TalosRegistry())
	if err != nil {
		return false, err
	}

	if err = ctrl.acquireUpgradeLock(ctx, r, rc); err != nil {
		return false, err
	}

	logger.Info("upgrading the machine",
		zap.String("from_version", installed.version),
		zap.String("to_version", rc.installImage.TalosVersion),
		zap.String("from_schematic", installed.currentSchematic),
		zap.String("to_schematic", rc.installImage.SchematicId),
		zap.String("image", image),
		zap.String("machine", rc.ID()))

	// give the Upgrade API longer timeout, as it pulls the installer image before returning
	upgradeCtx, upgradeCancel := context.WithTimeout(inputCtx, 5*time.Minute)
	defer upgradeCancel()

	stageUpgrade, err := ctrl.stageUpgrade(installed.version)
	if err != nil {
		return false, err
	}

	nodeClient, err := ctrl.lifecycleManager.GetForMachine(upgradeCtx, rc.ID())
	if err != nil {
		return false, fmt.Errorf("failed to get talos client: %w", err)
	}

	//nolint:staticcheck
	_, err = nodeClient.UpgradeWithOptions(
		upgradeCtx,
		client.WithUpgradeImage(image),
		client.WithUpgradePreserve(!maintenance),
		client.WithUpgradeStage(stageUpgrade),
		client.WithUpgradeForce(false),
	)

	// If upgrade is not implemented, it means that we run older Talos that doesn't support upgrades in maintenance mode.
	if status.Code(err) == codes.Unimplemented {
		return true, nil
	}

	return false, err
}

// runMaintenanceLifecycle performs a maintenance install or upgrade via Talos's LifecycleService.
func (ctrl *StatusController) runMaintenanceLifecycle(
	ctx context.Context,
	logger *zap.Logger,
	r controller.ReaderWriter,
	rc *ReconciliationContext,
) (bool, error) {
	machineID := rc.ID()

	operationName := "maintenance upgrade"
	if rc.lifecycleOp == lifecycle.OpMaintenanceInstall {
		operationName = "maintenance install"
	}

	if rc.bootID == "" {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine boot ID before %s: %s", operationName, machineID)
	}

	// Wait if we already triggered the install/upgrade and the node hasn't rebooted yet.
	if preRebootBootID := rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId; preRebootBootID != "" && rc.bootID == preRebootBootID {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine to reboot after %s: %s", operationName, machineID)
	}

	if stage := rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage(); stage != machineapi.MachineStatusEvent_MAINTENANCE {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %s is not in maintenance stage (%s), skipping %s", machineID, stage, operationName)
	}

	probeCtx, probeCancel := context.WithTimeout(ctx, lifecycle.LivenessProbeTimeout)
	defer probeCancel()

	if err := ctrl.lifecycleManager.CheckAlive(probeCtx, machineID); err != nil {
		logger.Info("liveness probe before "+operationName+" failed, will retry", zap.String("machine", machineID), zap.Error(err))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	}

	if err := ctrl.acquireUpgradeLock(ctx, r, rc); err != nil {
		return false, err
	}

	operation := lifecycle.Operation{
		MachineID:     machineID,
		MachineStatus: rc.machineStatus,
		Version:       rc.installImage.TalosVersion,
		InstallImage:  rc.installImage,
	}

	switch rc.lifecycleOp { //nolint:exhaustive
	case lifecycle.OpMaintenanceInstall:
		operation.Kind = lifecycle.KindInstall
		operation.Disk = rc.installDisk
	case lifecycle.OpMaintenanceUpgrade:
		operation.Kind = lifecycle.KindUpgrade
	}

	logger.Info("running "+operationName,
		zap.String("machine", machineID),
		zap.String("version", operation.Version),
		zap.String("disk", operation.Disk))

	err := ctrl.lifecycleManager.Run(ctx, operation, lifecycle.WithProgress(func(msg string) {
		logger.Debug(operationName+" progress", zap.String("machine", machineID), zap.String("message", msg))
	}))

	switch {
	case errors.Is(err, lifecycle.ErrAlreadyInFlight):
		// The management RPC is operating on this machine, so retry once it releases the slot.
		logger.Info(operationName+" already in flight", zap.String("machine", machineID))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	case err != nil:
		// Returning a raw error would make qtransform drop the output write and lose LastConfigError.
		rc.machineConfigStatus.TypedSpec().Value.LastConfigError = lifecycle.WrapErr(err, fmt.Sprintf("%s failed", operationName)).Error()

		logger.Error(operationName+" failed", zap.String("machine", machineID), zap.Error(err))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	default:
		rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId = rc.bootID
		rc.machineConfigStatus.TypedSpec().Value.LastConfigError = ""

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	}
}

// runClusterLifecycle upgrades an in-cluster Talos 1.13+ machine via LifecycleService.Upgrade.
func (ctrl *StatusController) runClusterLifecycle(
	ctx context.Context,
	logger *zap.Logger,
	r controller.ReaderWriter,
	rc *ReconciliationContext,
) (bool, error) {
	machineID := rc.ID()

	if rc.bootID == "" {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine boot ID before upgrade: %s", machineID)
	}

	if stage := rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage(); stage != machineapi.MachineStatusEvent_RUNNING {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %s is not running (%s), skipping upgrade", machineID, stage)
	}

	clusterName, ok := rc.clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return false, fmt.Errorf("cluster machine %q has no cluster label", machineID)
	}

	installed, err := ctrl.checkInstalledImage(ctx, rc)
	if err != nil {
		logger.Info("version check before upgrade failed, will retry", zap.String("machine", machineID), zap.Error(err))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	}

	if installed.atTarget {
		rc.machineConfigStatus.TypedSpec().Value.TalosVersion = installed.version
		rc.machineConfigStatus.TypedSpec().Value.SchematicId = installed.schematic

		return true, nil
	}

	// Wait if we already triggered the upgrade and the node hasn't rebooted yet.
	if preRebootBootID := rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId; preRebootBootID != "" && preRebootBootID == rc.bootID {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine to reboot after upgrade: %s", machineID)
	}

	nodeName, err := ctrl.getNodeName(ctx, r, machineID)
	if err != nil {
		return false, err
	}

	// Without a node name we can't cordon and drain, so wait rather than reboot a cluster member undrained.
	if nodeName == "" {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for node name before upgrade: %s", machineID)
	}

	if err = ctrl.acquireUpgradeLock(ctx, r, rc); err != nil {
		return false, err
	}

	opts := []lifecycle.Option{
		lifecycle.WithProgress(func(msg string) {
			logger.Debug("upgrade progress", zap.String("machine", machineID), zap.String("message", msg))
		}),
		lifecycle.WithCordonDrain(clusterName, nodeName),
	}

	clusterMachines, err := ctrl.listClusterMachines(ctx, r, clusterName)
	if err != nil {
		return false, err
	}

	if forfeitHook := ctrl.etcdForfeitHook(rc, clusterMachines); forfeitHook != nil {
		opts = append(opts, lifecycle.WithPreRebootHooks(forfeitHook))
	}

	operation := lifecycle.Operation{
		MachineID:     machineID,
		MachineStatus: rc.machineStatus,
		Version:       rc.installImage.TalosVersion,
		InstallImage:  rc.installImage,
		Kind:          lifecycle.KindUpgrade,
	}

	logger.Info("running upgrade",
		zap.String("machine", machineID),
		zap.String("version", operation.Version),
		zap.String("node", nodeName))

	err = ctrl.lifecycleManager.Run(ctx, operation, opts...)

	switch {
	case errors.Is(err, lifecycle.ErrAlreadyInFlight):
		// The management RPC is operating on this machine, so retry once it releases the slot.
		logger.Info("upgrade already in flight", zap.String("machine", machineID))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	case err != nil:
		// Returning a raw error would make qtransform drop the output write and lose LastConfigError.
		rc.machineConfigStatus.TypedSpec().Value.LastConfigError = lifecycle.WrapErr(err, "upgrade failed").Error()

		logger.Error("upgrade failed", zap.String("machine", machineID), zap.Error(err))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	default:
		rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId = rc.bootID
		rc.machineConfigStatus.TypedSpec().Value.LastConfigError = ""

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	}
}

// installedImage is the version and schematic read live from a node, plus whether they match the target.
type installedImage struct {
	version          string
	schematic        string
	currentSchematic string
	atTarget         bool
}

// checkInstalledImage reads the node's running version and schematic live and compares them to the target.
func (ctrl *StatusController) checkInstalledImage(
	ctx context.Context, rc *ReconciliationContext,
) (installedImage, error) {
	nodeClient, err := ctrl.lifecycleManager.GetForMachine(ctx, rc.ID())
	if err != nil {
		return installedImage{}, fmt.Errorf("failed to get talos client: %w", err)
	}

	actualVersion, err := getVersion(ctx, nodeClient.Client)
	if err != nil {
		return installedImage{}, err
	}

	// compatibility code for the machines running extensions installed bypassing image factory make schematic play no role in the checks
	if rc.machineStatus.TypedSpec().Value.Schematic.Invalid {
		return installedImage{
			version:          actualVersion,
			schematic:        "",
			atTarget:         actualVersion == rc.installImage.TalosVersion,
			currentSchematic: "",
		}, nil
	}

	// Use the existing protected args (e.g., the siderolink args) as the fallback args if we cannot determine the actual expected args
	fallbackKernelArgs := kernelargs.FilterProtected(rc.machineStatus.TypedSpec().Value.Schematic.KernelArgs)

	schematicInfo, err := talosutils.GetSchematicInfo(ctx, nodeClient.COSI, fallbackKernelArgs)
	if err != nil {
		if errors.Is(err, talosutils.ErrInvalidSchematic) {
			return installedImage{
				version:          actualVersion,
				schematic:        "",
				atTarget:         actualVersion == rc.installImage.TalosVersion,
				currentSchematic: "",
			}, nil
		}

		return installedImage{}, err
	}

	return installedImage{
		version:          actualVersion,
		schematic:        rc.installImage.SchematicId,
		currentSchematic: schematicInfo.FullID,
		atTarget:         actualVersion == rc.installImage.TalosVersion && schematicInfo.FullID == rc.installImage.SchematicId,
	}, nil
}

// etcdForfeitHook forfeits this node's etcd leadership if it's a ControlPlane node on a multi ControlPlane cluster.
func (ctrl *StatusController) etcdForfeitHook(rc *ReconciliationContext, clusterMachines []resource.Resource) lifecycle.PreRebootHook {
	if rc.clusterMachine == nil {
		return nil
	}

	if _, isControlPlane := rc.clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole); !isControlPlane {
		return nil
	}

	controlPlaneCount := 0

	for _, clusterMachine := range clusterMachines {
		if _, ok := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
			controlPlaneCount++
		}
	}

	if controlPlaneCount <= 1 {
		return nil
	}

	return func(hookCtx context.Context, talosClient *client.Client) error {
		_, forfeitErr := talosClient.EtcdForfeitLeadership(hookCtx, &machineapi.EtcdForfeitLeadershipRequest{})

		return forfeitErr
	}
}

// listClusterMachines lists the cluster's ClusterMachines uncached, so callers see the freshest finalizer set.
func (ctrl *StatusController) listClusterMachines(ctx context.Context, r controller.ReaderWriter, clusterName string) ([]resource.Resource, error) {
	qruntime := r.(controller.QRuntime) //nolint:forcetypeassert,errcheck

	list, err := qruntime.ListUncached(
		ctx,
		omni.NewClusterMachine("").Metadata(),
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
	)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

// getNodeName returns the machine's Kubernetes node name, or "" if its ClusterMachineIdentity isn't present yet.
func (ctrl *StatusController) getNodeName(ctx context.Context, r controller.Reader, machineID string) (string, error) {
	identity, err := safe.ReaderGetByID[*omni.ClusterMachineIdentity](ctx, r, machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get cluster machine identity %q: %w", machineID, err)
	}

	return identity.TypedSpec().Value.Nodename, nil
}

// stageUpgrade decides if the upgrade should be staged.
//
// Currently, it is only required as a workaround for this bug affecting Talos 1.9.0-1.9.2:
// https://github.com/siderolabs/talos/issues/10163.
func (ctrl *StatusController) stageUpgrade(actualTalosVersion string) (bool, error) {
	version, err := semver.ParseTolerant(actualTalosVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse talos version %q: %w", actualTalosVersion, err)
	}

	return version.Major == 1 && version.Minor == 9 && version.Patch < 3, nil
}

var errAcquireConfigLock = errors.New("failed to acquire config update lock")

func (ctrl *StatusController) applyConfig(inputCtx context.Context,
	logger *zap.Logger,
	r controller.ReaderWriter,
	rc *ReconciliationContext,
) (machineapi.ApplyConfigurationRequest_Mode, error) {
	ctx, cancel := context.WithTimeout(inputCtx, 5*time.Second)
	defer cancel()

	applyMaintenance := false

	switch rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage() {
	case machineapi.MachineStatusEvent_BOOTING,
		machineapi.MachineStatusEvent_RUNNING:
		// can apply config normal mode
	case machineapi.MachineStatusEvent_MAINTENANCE:
		// can apply config maintenance mode
		applyMaintenance = true
	case machineapi.MachineStatusEvent_INSTALLING,
		machineapi.MachineStatusEvent_REBOOTING,
		machineapi.MachineStatusEvent_RESETTING,
		machineapi.MachineStatusEvent_SHUTTING_DOWN,
		machineapi.MachineStatusEvent_UNKNOWN,
		machineapi.MachineStatusEvent_UPGRADING:
		// no way to apply config at this stage
		return 0, xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine '%s' is in %s stage", rc.ID(), rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage()))
	}

	if rc.machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 != "" && applyMaintenance {
		return 0, fmt.Errorf("failed to apply machine config: the machine is expected to be running in the normal mode, but is running in maintenance")
	}

	c, err := ctrl.getClient(ctx, r, applyMaintenance, rc.machineStatus, rc.machineConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to get client: %w", err)
	}

	defer logClose(c, logger, fmt.Sprintf("machine '%s'", rc.ID()))

	_, err = c.Version(ctx)
	if err != nil {
		return 0, err
	}

	ctx, applyCancel := context.WithTimeout(inputCtx, time.Minute)
	defer applyCancel()

	data, err := rc.machineConfig.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return 0, err
	}

	defer data.Free()

	if err = ctrl.acquireConfigUpdateLock(ctx, r, rc); err != nil {
		return 0, err
	}

	resp, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
		Data: data.Data(),
		Mode: machineapi.ApplyConfigurationRequest_AUTO,
	})
	if err != nil {
		logger.Error(
			"apply config failed",
			zap.String("machine", rc.ID()),
			zap.Error(err),
			zap.Stringer("config_version", rc.machineConfig.Metadata().Version()),
		)

		return 0, fmt.Errorf("failed to apply config to machine '%s': %w", rc.ID(), err)
	}

	if len(resp.Messages) != 1 {
		return 0, fmt.Errorf("unexpected number of responses: %d", len(resp.Messages))
	}

	mode := resp.Messages[0].GetMode()
	logger.Info(
		"applied machine config",
		zap.String("machine", rc.ID()),
		zap.Stringer("config_version", rc.machineConfig.Metadata().Version()),
		zap.Stringer("mode", mode),
	)

	return mode, nil
}

func logClose(c io.Closer, logger *zap.Logger, additional string) {
	if err := c.Close(); err != nil {
		logger.Error(additional+": failed to close client", zap.Error(err))
	}
}

//nolint:gocyclo,cyclop
func (ctrl *StatusController) reset(
	ctx context.Context,
	logger *zap.Logger,
	r controller.Reader,
	machineConfig *omni.ClusterMachineConfig,
) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	machineID := machineConfig.Metadata().ID()

	machineStatus, err := safe.ReaderGet[*omni.MachineStatus](ctx, r, omni.NewMachineStatus(machineID).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get machine status '%s': %w", machineID, err)
	}

	shouldReset, err := ctrl.shouldReset(ctx, r, machineConfig, machineStatus)
	if err != nil {
		return err
	}

	if !shouldReset {
		logger.Info("removed without reset")

		return nil
	}

	if !machineStatus.TypedSpec().Value.Connected {
		// machine is not connected, so we can't reset it
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine '%s' is not connected", machineID)
	}

	statusSnapshot, err := safe.ReaderGet[*omni.MachineStatusSnapshot](ctx, r, omni.NewMachineStatusSnapshot(machineID).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine '%s' status snapshot is not found: %w", machineID, err)
		}

		return fmt.Errorf("failed to get machine status snapshot '%s': %w", machineID, err)
	}

	var c *client.Client

	machineStage := statusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage()

	if machineStage == machineapi.MachineStatusEvent_RESETTING {
		return controller.NewRequeueErrorf(time.Minute, "the machine is already being reset")
	}

	logger.Debug("getting ready to reset the machine", zap.Stringer("stage", machineStage))

	inMaintenance := machineStage == machineapi.MachineStatusEvent_MAINTENANCE

	if inMaintenance {
		// verify that we are in maintenance mode
		c, err = ctrl.getClient(ctx, r, true, machineStatus, machineConfig)
		if err != nil {
			return fmt.Errorf("failed to get maintenance client for machine '%s': %w", machineID, err)
		}

		defer logClose(c, logger, "reset maintenance")

		_, err = c.Version(ctx)

		logger.Debug("maintenance mode check", zap.Error(err))

		if err == nil {
			// really in maintenance mode, no need to reset
			return nil
		}

		wrappedErr := fmt.Errorf("failed to get version in maintenance mode for machine '%s': %w", machineID, err)

		attempt := ctrl.ongoingResets.handleMaintenanceCheck(machineStatus.Metadata().ID())

		if attempt <= maintenanceCheckAttempts {
			// retry in N seconds
			return controller.NewRequeueError(wrappedErr, time.Second*time.Duration(attempt))
		}

		return xerrors.NewTagged[qtransform.SkipReconcileTag](wrappedErr)
	}

	clusterMachine, err := safe.ReaderGet[*omni.ClusterMachine](ctx, r, omni.NewClusterMachine(machineConfig.Metadata().ID()).Metadata())
	if err != nil {
		return fmt.Errorf("finalizer: failed to get cluster machine '%s': %w", machineConfig.Metadata().ID(), err)
	}

	graceful, err := ctrl.shouldResetGraceful(ctx, logger, r, clusterMachine)
	if err != nil {
		return fmt.Errorf("failed to determine if graceful reset should be performed: %w", err)
	}

	_, isControlPlane := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	switch {
	// check that the machine is ready to be reset
	// if running allow reset always
	case machineStage == machineapi.MachineStatusEvent_RUNNING:
	// if booting allow only non-graceful reset for control plane nodes
	case (!graceful || !isControlPlane) && machineStage == machineapi.MachineStatusEvent_BOOTING:
	default:
		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine '%s' is in %s stage", machineID, machineStage))
	}

	c, err = ctrl.getClient(ctx, r, false, machineStatus, machineConfig)
	if err != nil {
		return fmt.Errorf("failed to get client for machine '%s': %w", machineID, err)
	}

	defer logClose(c, logger, "reset")

	err = c.MetaDelete(ctx, meta.StateEncryptionConfig)
	if err != nil {
		//nolint:exhaustive
		switch status.Code(err) {
		case
			codes.NotFound,
			codes.Unimplemented,
			codes.FailedPrecondition:
		default:
			return fmt.Errorf("failed resetting node '%s': %w", machineID, err)
		}
	}

	// if is control plane first leave etcd
	if isControlPlane && ctrl.ongoingResets.shouldLeaveEtcd(machineID) {
		ctrl.ongoingResets.handleEtcdLeave(machineID)

		err = ctrl.gracefulEtcdLeave(ctx, c, machineID)
		if err != nil {
			return controller.NewRequeueError(err, time.Second)
		}
	}

	err = c.ResetGeneric(ctx, &machineapi.ResetRequest{
		Graceful: graceful,
		Reboot:   true,
		SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
			{
				Label: constants.EphemeralPartitionLabel,
				Wipe:  true,
			},
			{
				Label: constants.StatePartitionLabel,
				Wipe:  true,
			},
		},
	})
	if err == nil {
		attempt := ctrl.ongoingResets.handleReset(machineID)
		logger.Info("resetting node", zap.Uint("attempt", attempt), zap.Bool("graceful", graceful))

		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("check back when machine '%s' gets into maintenance mode", machineID)
	}

	logger.Error(
		"failed resetting node",
		zap.Error(err),
	)

	return fmt.Errorf("failed resetting node '%s': %w", machineID, err)
}

func (ctrl *StatusController) shouldReset(
	ctx context.Context,
	r controller.Reader,
	machineConfig *omni.ClusterMachineConfig,
	machineStatus *omni.MachineStatus,
) (bool, error) {
	if machineStatus == nil {
		return false, nil
	}

	machineID := machineConfig.Metadata().ID()

	clusterName, ok := machineConfig.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return false, fmt.Errorf("failed to determine the cluster name from the cluster machine config %q", machineConfig.Metadata().ID())
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		return false, fmt.Errorf("finalizer: failed to get cluster '%s': %w", clusterName, err)
	}

	clusterStatus, err := safe.ReaderGetByID[*omni.ClusterStatus](ctx, r, clusterName)
	if err != nil {
		return false, fmt.Errorf("finalizer: failed to get cluster status '%s': %w", clusterName, err)
	}

	// Read the Machine uncached: a cached read can briefly still report a Machine that is being torn
	// down or destroyed as present and running, so the reset decision below could be made on stale data.
	machine, err := safe.ReaderGetByID[*omni.Machine](ctx, uncached.Reader(r), machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get machine '%s': %w", machineID, err)
	}

	if machine.Metadata().Phase() == resource.PhaseTearingDown {
		return false, nil
	}

	_, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked)
	_, importing := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
	_, exporting := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByExporting)

	if locked && (importing || exporting) && cluster.Metadata().Phase() == resource.PhaseTearingDown {
		return false, nil
	}

	return true, nil
}

func (ctrl *StatusController) shouldResetGraceful(
	ctx context.Context,
	logger *zap.Logger,
	r controller.Reader,
	clusterMachine *omni.ClusterMachine,
) (bool, error) {
	forceDestroyRequest, err := safe.ReaderGetByID[*omni.NodeForceDestroyRequest](ctx, r, clusterMachine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get node force delete request %q: %w", clusterMachine.Metadata().ID(), err)
	}

	if forceDestroyRequest != nil {
		logger.Info("node force etcd leave was requested")

		return false, nil
	}

	machineSetName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return false, fmt.Errorf("failed to determine machine set of the cluster machine %s", clusterMachine.Metadata().ID())
	}

	machineSetConfigStatus, err := safe.ReaderGetByID[*omni.MachineSetConfigStatus](ctx, r, machineSetName)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get machine set '%s': %w", machineSetName, err)
	}

	if !ctrl.ongoingResets.isGraceful(clusterMachine.Metadata().ID()) {
		return false, nil
	}

	return machineSetConfigStatus != nil && machineSetConfigStatus.TypedSpec().Value.ShouldResetGraceful, nil
}

func (ctrl *StatusController) gracefulEtcdLeave(ctx context.Context, c *client.Client, id string) error {
	_, err := c.EtcdForfeitLeadership(ctx, &machineapi.EtcdForfeitLeadershipRequest{})
	if err != nil {
		return fmt.Errorf("failed to forfeit leadership, node %q: %w", id, err)
	}

	err = c.EtcdLeaveCluster(ctx, &machineapi.EtcdLeaveClusterRequest{})
	if err != nil {
		return fmt.Errorf("failed to leave etcd cluster, node %q: %w", id, err)
	}

	return nil
}

func (ctrl *StatusController) getClient(
	ctx context.Context,
	r controller.Reader,
	useMaintenance bool,
	machineStatus *omni.MachineStatus,
	machineConfig *omni.ClusterMachineConfig,
) (*client.Client, error) {
	address := machineStatus.TypedSpec().Value.ManagementAddress

	if useMaintenance {
		return talos.NewMaintenanceClient(ctx, address)
	}

	opts := talos.GetSocketOptions(address)

	clusterName, ok := machineConfig.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, errors.New("no cluster name label")
	}

	talosConfig, err := safe.ReaderGet[*omni.TalosConfig](ctx, r, omni.NewTalosConfig(clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster '%s' talosconfig not found: %w", clusterName, err)
		}

		return nil, fmt.Errorf("cluster '%s' failed to get talosconfig: %w", clusterName, err)
	}

	var endpoints []string

	if opts == nil {
		endpoints = []string{address}
	}

	config := omni.NewTalosClientConfig(talosConfig, endpoints...)
	opts = append(opts, client.WithConfig(config))

	result, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client to machine '%s': %w", machineConfig.Metadata().ID(), err)
	}

	return result, nil
}

// errAcquireUpgradeLock marks the case where a machine cannot upgrade because no upgrade slot
// is free. It is also SkipReconcile-tagged so the framework still treats it as a skip, while
// callers can detect this specific reason with errors.Is.
var errAcquireUpgradeLock = errors.New("failed to acquire upgrade lock")

func (ctrl *StatusController) acquireUpgradeLock(ctx context.Context, r controller.ReaderWriter,
	rc *ReconciliationContext,
) error {
	ctrl.acquireLockMu.Lock()
	defer ctrl.acquireLockMu.Unlock()

	if rc.machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 == "" {
		return nil
	}

	if rc.clusterMachine.Metadata().Finalizers().Has(UpgradeFinalizer) {
		return nil
	}

	clusterName, ok := rc.clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return errors.New("failed to get cluster name from the cluster machine")
	}

	machineSetName, ok := rc.clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return fmt.Errorf("failed to determine machine set of the cluster machine %s", rc.clusterMachine.Metadata().ID())
	}

	rollout, err := safe.ReaderGetByID[*omni.UpgradeRollout](ctx, r, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for upgrade rollout rules to propagate")
		}

		return fmt.Errorf("failed to get upgrade rollout: %w", err)
	}

	quota := rollout.TypedSpec().Value.MachineSetsUpgradeQuota[machineSetName]
	if quota == 0 {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%w: waiting for free quota for upgrade", errAcquireUpgradeLock)
	}

	clusterMachines, err := ctrl.listClusterMachines(ctx, r, clusterName)
	if err != nil {
		return err
	}

	maxQuota := quota

	for _, clusterMachine := range clusterMachines {
		if clusterMachine.Metadata().Finalizers().Has(UpgradeFinalizer) {
			quota--
		}

		if quota <= 0 {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%w: quota %d for upgrades reached, waiting for the locks to be released", errAcquireUpgradeLock, maxQuota)
		}
	}

	return r.AddFinalizer(ctx, rc.clusterMachine.Metadata(), UpgradeFinalizer)
}

func (ctrl *StatusController) acquireConfigUpdateLock(ctx context.Context, r controller.ReaderWriter,
	rc *ReconciliationContext,
) error {
	ctrl.acquireLockMu.Lock()
	defer ctrl.acquireLockMu.Unlock()

	if rc.machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 == "" {
		return nil
	}

	if rc.clusterMachine.Metadata().Finalizers().Has(ConfigUpdateFinalizer) {
		return nil
	}

	if !rc.configUpdatesAllowed {
		return fmt.Errorf("%w: machine set blocks the config changes", errAcquireConfigLock)
	}

	machineSetName, ok := rc.clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return errors.New("failed to get machine set name from the cluster machine")
	}

	// A stale strategy could exceed the configured update parallelism.
	machineSetConfigStatus, err := safe.ReaderGetByID[*omni.MachineSetConfigStatus](ctx, uncached.Reader(r), machineSetName)
	if err != nil {
		return err
	}

	qruntime := r.(controller.QRuntime) //nolint:forcetypeassert,errcheck

	clusterMachines, err := qruntime.ListUncached(
		ctx,
		omni.NewClusterMachine("").Metadata(),
		state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSetName)),
	)
	if err != nil {
		return err
	}

	updateParallelism := omni.GetParallelism(machineSetConfigStatus.TypedSpec().Value.UpdateStrategy, machineSetConfigStatus.TypedSpec().Value.UpdateStrategyConfig, 1)

	quota := updateParallelism

	pendingMachines := []string{}

	for _, clusterMachine := range clusterMachines.Items {
		if clusterMachine.Metadata().Finalizers().Has(ConfigUpdateFinalizer) {
			pendingMachines = append(pendingMachines, clusterMachine.Metadata().ID())

			quota--
		}

		if quota <= 0 {
			return fmt.Errorf("%w: quota %d for updates reached, waiting for the locks to be released, pending: %#v", errAcquireConfigLock, updateParallelism, pendingMachines)
		}
	}

	return r.AddFinalizer(ctx, rc.clusterMachine.Metadata(), ConfigUpdateFinalizer)
}

func (ctrl *StatusController) releaseConfigUpdateLock(ctx context.Context, r controller.ReaderWriter, clusterMachine *omni.ClusterMachine) error {
	if clusterMachine == nil || !clusterMachine.Metadata().Finalizers().Has(ConfigUpdateFinalizer) {
		return nil
	}

	return r.RemoveFinalizer(ctx, clusterMachine.Metadata(), ConfigUpdateFinalizer)
}

func (ctrl *StatusController) releaseUpgradeLock(ctx context.Context, r controller.ReaderWriter, clusterMachine *omni.ClusterMachine) error {
	if clusterMachine == nil || !clusterMachine.Metadata().Finalizers().Has(UpgradeFinalizer) {
		return nil
	}

	return r.RemoveFinalizer(ctx, clusterMachine.Metadata(), UpgradeFinalizer)
}

// computePendingUpdates reconciles the MachinePendingUpdates resource and reports whether the
// config last pushed to the machine (the redacted config stored in the status) differs from the
// desired config. The caller uses this to re-apply a machine that has drifted from the desired
// config even when the recorded (confirmed) sha already matches it.
func (ctrl *StatusController) computePendingUpdates(ctx context.Context, r controller.ReaderWriter, rc *ReconciliationContext) (bool, error) {
	pendingUpdates := omni.NewMachinePendingUpdates(rc.machineConfig.Metadata().ID())

	currentRedactedMachineConfig, err := rc.machineConfigStatus.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return false, err
	}

	defer currentRedactedMachineConfig.Free()

	configDiff, err := diff.Compute(currentRedactedMachineConfig.Data(), rc.redactedMachineConfig)
	if err != nil {
		return false, err
	}

	var (
		upgradeDiff         bool
		currentSchematicID  string
		currentTalosVersion string
	)

	if rc.machineConfigStatus != nil && rc.installImage != nil &&
		rc.machineConfigStatus.TypedSpec().Value.TalosVersion != "" && rc.machineConfigStatus.TypedSpec().Value.SchematicId != "" && rc.hasPendingLifecycleOperation() {
		currentSchematicID = rc.machineConfigStatus.TypedSpec().Value.SchematicId
		currentTalosVersion = rc.machineConfigStatus.TypedSpec().Value.TalosVersion

		upgradeDiff = currentSchematicID != rc.installImage.SchematicId ||
			currentTalosVersion != rc.installImage.TalosVersion
	}

	// if no pending changes, delete the pending updates resource
	if !upgradeDiff && configDiff == "" {
		_, err = helpers.TeardownAndDestroy(ctx, r, pendingUpdates.Metadata())

		return false, err
	}

	return configDiff != "", safe.WriterModify(ctx, r, pendingUpdates, func(res *omni.MachinePendingUpdates) error {
		helpers.CopyAllLabels(rc.machineConfig, res)

		if upgradeDiff {
			res.TypedSpec().Value.Upgrade = &specs.MachinePendingUpdatesSpec_Upgrade{
				FromSchematic: currentSchematicID,
				ToSchematic:   rc.installImage.SchematicId,

				FromVersion: currentTalosVersion,
				ToVersion:   rc.installImage.TalosVersion,
			}
		} else {
			res.TypedSpec().Value.Upgrade = nil
		}

		res.TypedSpec().Value.ConfigDiff = configDiff

		return nil
	})
}

func (ctrl *StatusController) deleteUpgradeMetaKey(
	ctx context.Context,
	logger *zap.Logger,
	r controller.Reader,
	rc *ReconciliationContext,
) error {
	client, err := ctrl.getClient(ctx, r, false, rc.machineStatus, rc.machineConfig)
	if err != nil {
		return fmt.Errorf("failed to get client for machine %q: %w", rc.ID(), err)
	}

	defer logClose(client, logger, fmt.Sprintf("machine %q", rc.ID()))

	if err = client.MetaDelete(ctx, meta.Upgrade); err != nil {
		if status.Code(err) == codes.NotFound {
			logger.Debug("upgrade meta key not found", zap.String("machine", rc.ID()))

			return nil
		}

		if status.Code(err) == codes.Unimplemented {
			logger.Debug("upgrade meta key is not removed, unimplemented in the Talos version", zap.String("machine", rc.ID()))

			return nil
		}

		return err
	}

	logger.Info("deleted upgrade meta key", zap.String("machine", rc.ID()))

	return nil
}

func getVersion(ctx context.Context, c *client.Client) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	versionResponse, err := c.Version(ctx)
	if err != nil {
		return "", err
	}

	for _, m := range versionResponse.Messages {
		return strings.TrimLeft(m.Version.Tag, "v"), nil
	}

	return "", errors.New("failed to get Talos version on the machine")
}
