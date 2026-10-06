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
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/diff"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configtry"
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
	GetForMachine(ctx context.Context, machineID string) (*talos.Client, error)
	Run(ctx context.Context, op lifecycle.Operation, opts ...lifecycle.Option) error
	FinalizeReboot(ctx context.Context, opts ...lifecycle.Option) error
}

// StatusController manages the ClusterMachineConfigStatus resource lifecycle.
//
// StatusController applies the generated machine config on each corresponding machine.
type StatusController struct {
	*qtransform.QController[*omni.ClusterMachineConfig, *omni.ClusterMachineConfigStatus]
	ongoingResets    *ongoingResets
	lifecycleManager LifecycleManager
	tryTimings       configtry.Timings
	acquireLockMu    sync.Mutex
}

// Option configures StatusController.
type Option func(*StatusController)

// WithTryTimings overrides the try mode timings. Tests use it to keep the wait short.
func WithTryTimings(timings configtry.Timings) Option {
	return func(ctrl *StatusController) {
		ctrl.tryTimings = timings
	}
}

// NewStatusController initializes StatusController.
func NewStatusController(lifecycleManager LifecycleManager, opts ...Option) *StatusController {
	ongoingResets := &ongoingResets{
		statuses: map[string]*resetStatus{},
	}

	ctrl := &StatusController{
		ongoingResets:    ongoingResets,
		lifecycleManager: lifecycleManager,
		tryTimings:       configtry.Default,
	}

	for _, opt := range opts {
		opt(ctrl)
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
		qtransform.WithExtraMappedInput[*omni.MaintenanceConfigStatus](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*siderolinkres.Link](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
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
	now := time.Now()

	rc, err := BuildReconciliationContext(ctx, r, machineConfig, machineConfigStatus, ctrl.tryTimings, now)
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
		// the machine rolls the try back on its own, so it does not count as an attempt
		if rc.tryInFlight {
			machineConfigStatus.TypedSpec().Value.ConfigTry = nil
		}

		logger.Info("operations locked for machine")

		return nil
	}

	if rc.lastConfigError != "" {
		// the machine rolls the try back on its own, so it does not count as an attempt
		if rc.tryInFlight {
			machineConfigStatus.TypedSpec().Value.ConfigTry = nil
		}

		machineConfigStatus.TypedSpec().Value.LastConfigError = rc.lastConfigError

		logger.Info("config generation error", zap.String("error", rc.lastConfigError))

		return nil
	}

	// For invalid schematic machines, clear any stale schematic ID directly
	// instead of going through upgrade() which would make unnecessary Talos API calls.
	if rc.machineStatus.TypedSpec().Value.Schematic.Invalid && machineConfigStatus.TypedSpec().Value.SchematicId != "" {
		machineConfigStatus.TypedSpec().Value.SchematicId = ""
	}

	if !rc.tryInFlight {
		if err = ctrl.reconcileUpgrade(ctx, logger, r, rc); err != nil {
			// LifecycleManager reported that underlying Talos API call failed with a permanent error,
			// so we don't retry the operation and just record the error in status.
			if lifecycle.IsPermanentInstallerFailure(err) {
				return nil
			}

			return err
		}

		stage := rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage()
		if stage == machineapi.MachineStatusEvent_BOOTING || stage == machineapi.MachineStatusEvent_RUNNING {
			if err = ctrl.deleteUpgradeMetaKey(ctx, logger, r, rc); err != nil {
				return err
			}
		}
	}

	buffer, err := machineConfig.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return fmt.Errorf("failed to get uncompressed config: %w", err)
	}

	defer buffer.Free()

	shaSum := sha256.Sum256(buffer.Data())
	shaSumString := hex.EncodeToString(shaSum[:])

	// Re-apply when the confirmed config differs from the desired one (sha mismatch), or when the
	// config we last pushed to the machine differs from the desired one. The second case covers a
	// reboot-requiring change reverted before the machine confirms it: such a push is committed to
	// the machine but its sha is recorded only after the machine comes back, so a revert in that
	// window leaves the recorded sha matching the desired config. Comparing the last pushed config
	// lets us notice the machine still runs the reverted change and re-apply, instead of treating it
	// as in sync.
	needsApply := machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 != shaSumString || configChanged

	result, err := ctrl.reconcileConfigApply(ctx, logger, r, rc, shaSumString, needsApply, now)

	// ClusterMachineConfig might receive an update that causes its version to change while not affecting the final MachineConfig therefore calculated hashsum.
	// Update ClusterMachineConfigVersion to reflect the actual version of the ClusterMachineConfig regardless of hash comparison.
	machineConfigStatus.TypedSpec().Value.ClusterMachineConfigVersion = machineConfig.Metadata().Version().String()

	if err != nil {
		// a requeue carries try state that has to be written
		if requeue, ok := errors.AsType[*controller.RequeueError](err); ok {
			return requeue
		}

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

	// A config the machine only holds tentatively must not be recorded: computePendingUpdates would
	// then see no diff and drop MachinePendingUpdates, so the UI would call the machine in sync
	// before it actually is.
	if result.recordConfig {
		if err = machineConfigStatus.TypedSpec().Value.SetUncompressedData(rc.redactedMachineConfig); err != nil {
			return err
		}
	}

	if !result.committed {
		return nil
	}

	helpers.CopyLabels(machineConfig, machineConfigStatus, omni.LabelMachineSet, omni.LabelCluster, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

	machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 = shaSumString

	// The machine now carries the desired config (either just applied, or already in sync), so its
	// high-priority documents are up to date. Recording the hash clears highPriorityPending; it also
	// backfills already-configured machines on the first reconcile after this feature is deployed,
	// without a re-apply (the whole-config sha still matches, so no apply was needed).
	machineConfigStatus.TypedSpec().Value.AppliedHighPriorityConfigHash = rc.highPriorityHash

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
//
//nolint:gocognit,gocyclo,cyclop
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
		switch {
		// If high-priority config (image factory registry auth / custom CA) changes are pending on an
		// in-cluster machine, defer the upgrade and let the config be applied first: the upgrade pulls
		// the installer image and needs those credentials/CA to be on the machine already. We neither
		// call ctrl.upgrade nor skip-reconcile, so the flow falls through to applyConfig below. The
		// TalosVersion/SchematicId are not recorded because hasPendingLifecycleOperation() stays true,
		// so the version-recording branch below is not entered. The upgrade is retried on a later
		// reconcile, once the high-priority config is applied and no longer pending.
		case (rc.lifecycleOp == lifecycle.OpClusterUpgrade || rc.lifecycleOp == lifecycle.OpLegacyUpgrade) && rc.highPriorityPending:
			logger.Info("deferring upgrade until high-priority config is applied", zap.String("machine", rc.ID()))

		// A machine being installed/upgraded while still in maintenance mode has its config — including any
		// high-priority documents (image factory registry auth / custom CA) — applied by the maintenance
		// config controller, not by this one. Block the install/upgrade until that controller has applied
		// its config for the current connection, otherwise the installer image pull could run before the
		// credentials/CA are on the machine.
		//
		// We gate every maintenance install/upgrade, not only when the rendered ClusterMachineConfig already
		// carries high-priority documents: that rendered config lags its inputs, so right after credentials
		// are added it can still be empty of them while the machine becomes installable. The maintenance
		// controller applies the credentials/CA straight from their source resources (ImageFactoryAuth,
		// machine ConfigPatches), independent of the render, so waiting on it is what actually guarantees the
		// machine has them. This only affects Talos 1.13+ machines (older ones take the legacy upgrade path),
		// which the maintenance controller always serves, so the gate cannot get permanently stuck.
		case (rc.lifecycleOp == lifecycle.OpMaintenanceInstall || rc.lifecycleOp == lifecycle.OpMaintenanceUpgrade) &&
			!rc.maintenanceConfigApplied:
			logger.Info("waiting for maintenance config controller to apply config before install/upgrade", zap.String("machine", rc.ID()))

			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for maintenance config controller to apply config: %s", rc.ID())

		default:
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
		}
	} else {
		// The cached view can read None while the machine we triggered is still mid-reboot. Applying then
		// loses the config, so wait for the boot ID to change before clearing the marker and applying.
		if preRebootBootID := rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId; preRebootBootID != "" && rc.bootID == preRebootBootID {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine to reboot before applying config: %s", rc.ID())
		}

		// A machine with no system disk hasn't installed Talos yet: its live TalosVersion is still the
		// boot media it's running from, not what will end up on disk once config-apply installs it, so
		// only record the version once it actually has one. This can only happen for Talos < 1.13 on
		// either end.
		if omni.GetMachineStatusSystemDisk(rc.machineStatus) != "" {
			// Record the version so the status reflects an already-at-target machine that ran no upgrade path.
			rc.machineConfigStatus.TypedSpec().Value.TalosVersion = strings.TrimLeft(rc.machineStatus.TypedSpec().Value.TalosVersion, "v")

			if rc.machineStatus.TypedSpec().Value.GetSchematic().GetInvalid() {
				rc.machineConfigStatus.TypedSpec().Value.SchematicId = ""
				rc.machineConfigStatus.TypedSpec().Value.ImageFactoryHost = ""
			} else {
				rc.machineConfigStatus.TypedSpec().Value.SchematicId = rc.installImage.SchematicId
				rc.machineConfigStatus.TypedSpec().Value.ImageFactoryHost = rc.installImage.ImageFactoryHost
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
		rc.machineConfigStatus.TypedSpec().Value.ImageFactoryHost = installed.factoryHost

		return true, nil
	}

	if rc.installImage.ImageFactoryHost == "" {
		return false, xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine '%s' does not have image factory host", rc.ID()))
	}

	image, err := installimage.Build(rc.ID(), rc.installImage)
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

	// An unhealthy machine also gets a staged upgrade. The regular upgrade sequence on the node
	// drains the node first, and the drain waits for the node to be cordoned through the Kubernetes
	// API. An unhealthy machine may have no reachable Kubernetes API at all, e.g. a single control
	// plane whose kubelet is crashlooping. The drain then times out, the whole upgrade sequence
	// fails, and the machine reboots into the same broken image over and over. A staged upgrade
	// skips the drain and applies the new image on the next boot instead.
	recovering := rc.upgradeBlockedByOwnHealth()
	stageUpgrade = stageUpgrade || recovering

	nodeClient, err := ctrl.lifecycleManager.GetForMachine(upgradeCtx, rc.ID())
	if err != nil {
		return false, fmt.Errorf("failed to get talos client: %w", err)
	}

	defer nodeClient.Close() //nolint:errcheck

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

	if err == nil && recovering {
		// A machine stuck mid-boot cannot run the staged upgrade sequence at all: the sequencer
		// refuses to start it while the boot sequence is running, and only a reboot is allowed to
		// take over a stuck boot. The staged image is already recorded on the machine at this
		// point, so an explicit reboot applies it. Talos acknowledges the reboot request before
		// running the sequence, so an error here only means the request did not reach the machine.
		// The next reconcile then stages and reboots again.
		if rebootErr := nodeClient.Reboot(upgradeCtx); rebootErr != nil {
			logger.Warn("reboot request after staged upgrade failed, retrying on the next reconcile",
				zap.String("machine", rc.ID()), zap.Error(rebootErr))
		}
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
	if preRebootBootID := rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId; preRebootBootID != "" {
		if rc.bootID == preRebootBootID {
			return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine to reboot after %s: %s", operationName, machineID)
		}

		if omni.GetMachineStatusSystemDisk(rc.machineStatus) == "" {
			return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine status to get valid system disk after %s: %s", operationName, machineID)
		}
	}

	if stage := rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage(); stage != machineapi.MachineStatusEvent_MAINTENANCE {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %s is not in maintenance stage (%s), skipping %s", machineID, stage, operationName)
	}

	probeCtx, probeCancel := context.WithTimeout(ctx, lifecycle.LivenessProbeTimeout)
	defer probeCancel()

	installed, err := ctrl.checkInstalledImage(probeCtx, rc)
	if err != nil {
		logger.Info("version check before "+operationName+" failed, will retry", zap.String("machine", machineID), zap.Error(err))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	}

	if installed.atTarget && omni.GetMachineStatusSystemDisk(rc.machineStatus) != "" {
		rc.machineConfigStatus.TypedSpec().Value.TalosVersion = installed.version
		rc.machineConfigStatus.TypedSpec().Value.SchematicId = installed.schematic
		rc.machineConfigStatus.TypedSpec().Value.ImageFactoryHost = installed.factoryHost

		return true, nil
	}

	if err = ctrl.acquireUpgradeLock(ctx, r, rc); err != nil {
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

	err = ctrl.lifecycleManager.Run(ctx, operation, lifecycle.WithProgress(func(msg string) {
		logger.Debug(operationName+" progress", zap.String("machine", machineID), zap.String("message", msg))
	}))

	switch {
	case errors.Is(err, lifecycle.ErrAlreadyInFlight):
		// The management RPC is operating on this machine, so retry once it releases the slot.
		logger.Info(operationName+" already in flight", zap.String("machine", machineID))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	case lifecycle.IsPermanentInstallerFailure(err):
		rc.machineConfigStatus.TypedSpec().Value.LastConfigError = lifecycle.WrapErr(err, fmt.Sprintf("%s failed", operationName)).Error()

		logger.Error(operationName+" failed permanently", zap.String("machine", machineID), zap.Error(err))

		return false, err
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

	// Booting machines are eligible too: a machine stuck in the booting stage may need this very
	// upgrade to become healthy again (e.g. reverting a kernel args change that broke it), and the
	// legacy upgrade path and the config apply path both accept booting machines already.
	if stage := rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage(); stage != machineapi.MachineStatusEvent_RUNNING && stage != machineapi.MachineStatusEvent_BOOTING {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %s is not running or booting (%s), skipping upgrade", machineID, stage)
	}

	clusterName, ok := rc.clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return false, fmt.Errorf("cluster machine %q has no cluster label", machineID)
	}

	// Wait if we already triggered the upgrade and the node hasn't rebooted yet.
	if preRebootBootID := rc.machineConfigStatus.TypedSpec().Value.PreRebootBootId; preRebootBootID != "" && preRebootBootID == rc.bootID {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("waiting for machine to reboot after upgrade: %s", machineID)
	}

	installed, err := ctrl.checkInstalledImage(ctx, rc)
	if err != nil {
		logger.Info("version check before upgrade failed, will retry", zap.String("machine", machineID), zap.Error(err))

		return false, controller.NewRequeueInterval(lifecycle.RetryInterval)
	}

	if installed.atTarget {
		rc.machineConfigStatus.TypedSpec().Value.TalosVersion = installed.version
		rc.machineConfigStatus.TypedSpec().Value.SchematicId = installed.schematic
		rc.machineConfigStatus.TypedSpec().Value.ImageFactoryHost = installed.factoryHost

		return true, nil
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
		lifecycle.WithCordonDrain(clusterName, nodeName, rc.upgradeBlockedByOwnHealth()),
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
	case lifecycle.IsPermanentInstallerFailure(err):
		rc.machineConfigStatus.TypedSpec().Value.LastConfigError = lifecycle.WrapErr(err, "upgrade failed").Error()

		logger.Error("upgrade failed permanently", zap.String("machine", machineID), zap.Error(err))

		return false, err
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
	factoryHost      string
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

	defer nodeClient.Close() //nolint:errcheck

	actualVersion, err := getVersion(ctx, nodeClient.Client)
	if err != nil {
		return installedImage{}, err
	}

	var schematicInfo talosutils.SchematicInfo

	invalid := rc.machineStatus.TypedSpec().Value.Schematic.Invalid
	if !invalid { // the machine status may not have caught up with the machine yet
		// Use the existing protected args (e.g., the siderolink args) as the fallback args if we cannot determine the actual expected args
		fallbackKernelArgs := kernelargs.FilterProtected(rc.machineStatus.TypedSpec().Value.Schematic.KernelArgs)

		schematicInfo, err = talosutils.GetSchematicInfo(ctx, nodeClient.COSI, fallbackKernelArgs)
		if err != nil {
			return installedImage{}, err
		}

		invalid = schematicInfo.Invalid
	}

	// the schematic plays no role in the checks for the machines running extensions installed bypassing the image factory
	if invalid {
		return installedImage{
			version:  actualVersion,
			atTarget: actualVersion == rc.installImage.TalosVersion,
		}, nil
	}

	return installedImage{
		version:          actualVersion,
		schematic:        rc.installImage.SchematicId,
		factoryHost:      rc.installImage.ImageFactoryHost,
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

// stageUpgrade decides if the upgrade should be staged based on the Talos version.
//
// It is required as a workaround for this bug affecting Talos 1.9.0-1.9.2:
// https://github.com/siderolabs/talos/issues/10163.
func (ctrl *StatusController) stageUpgrade(actualTalosVersion string) (bool, error) {
	version, err := semver.ParseTolerant(actualTalosVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse talos version %q: %w", actualTalosVersion, err)
	}

	return version.Major == 1 && version.Minor == 9 && version.Patch < 3, nil
}

var errAcquireConfigLock = errors.New("failed to acquire config update lock")

// configApplyResult reports how far reconcileConfigApply got with the desired config.
type configApplyResult struct {
	// recordConfig is set when the config Omni pushed is what the machine will be running, so the
	// status may record it as the last pushed config.
	recordConfig bool

	// committed is set when the machine is confirmed running the config and has it on disk.
	committed bool
}

// reconcileConfigApply gets the desired config onto the machine, in try mode where the machine supports it.
//
// It returns a *controller.RequeueError whenever a try is in flight, so that the recorded try state is written.
func (ctrl *StatusController) reconcileConfigApply(
	ctx context.Context,
	logger *zap.Logger,
	r controller.ReaderWriter,
	rc *ReconciliationContext,
	shaSum string,
	needsApply bool,
	now time.Time,
) (configApplyResult, error) {
	status := rc.machineConfigStatus.TypedSpec().Value

	if !ctrl.tryModeEligible(rc) {
		status.ConfigTry = nil

		if !needsApply {
			return configApplyResult{recordConfig: true, committed: true}, nil
		}

		result, err := ctrl.applyConfig(ctx, logger, r, rc, applyOptions{mode: machineapi.ApplyConfigurationRequest_AUTO})
		if err != nil {
			return configApplyResult{}, err
		}

		return configApplyResult{recordConfig: true, committed: result.mode == machineapi.ApplyConfigurationRequest_NO_REBOOT}, nil
	}

	action, delay := ctrl.tryTimings.Decide(status.ConfigTry, shaSum, now)

	// the desired config is already committed and no try is left on the machine
	if !needsApply && action == configtry.Try && !ctrl.tryTimings.InFlight(status.ConfigTry, now) {
		status.ConfigTry = nil

		return configApplyResult{recordConfig: true, committed: true}, nil
	}

	switch action {
	case configtry.Wait:
		return configApplyResult{}, controller.NewRequeueInterval(delay)

	case configtry.Stop:
		status.LastConfigError = fmt.Sprintf("the config was rolled back %d times, change it to try again", status.ConfigTry.GetAttempts())

		return configApplyResult{}, nil

	case configtry.Confirm:
		_, err := ctrl.applyConfig(ctx, logger, r, rc, applyOptions{
			mode:         machineapi.ApplyConfigurationRequest_NO_REBOOT,
			expectBootID: status.ConfigTry.GetBootId(),
			deadline:     status.ConfigTry.GetStartedAt().AsTime().Add(ctrl.tryTimings.Timeout),
		})
		if err != nil {
			logger.Warn("try mode config apply could not be confirmed yet", zap.String("machine", rc.ID()), zap.Error(err))

			return configApplyResult{}, controller.NewRequeueInterval(ctrl.tryTimings.ConfirmRetry)
		}

		status.ConfigTry = nil

		logger.Info("confirmed try mode config apply", zap.String("machine", rc.ID()))

		return configApplyResult{recordConfig: true, committed: true}, nil

	case configtry.Try:
		result, err := ctrl.applyConfig(ctx, logger, r, rc, applyOptions{
			mode: machineapi.ApplyConfigurationRequest_TRY,
		})
		if err != nil {
			return configApplyResult{}, err
		}

		status.ConfigTry = configtry.Begin(status.ConfigTry, shaSum, result.bootID, result.startedAt)
		status.LastConfigError = ""

		logger.Info("applied config in try mode", zap.String("machine", rc.ID()), zap.Uint32("attempt", status.ConfigTry.GetAttempts()))

		return configApplyResult{}, controller.NewRequeueInterval(ctrl.tryTimings.ConfirmAfter)

	default:
		return configApplyResult{}, fmt.Errorf("unexpected try mode action %d for machine '%s'", action, rc.ID())
	}
}

// tryModeEligible reports whether the desired config can be pushed to this machine in try mode.
func (ctrl *StatusController) tryModeEligible(rc *ReconciliationContext) bool {
	// the first config takes the machine out of maintenance, and a rollback cannot bring it back
	if rc.machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 == "" {
		return false
	}

	version, err := semver.ParseTolerant(rc.machineStatus.TypedSpec().Value.TalosVersion)
	if err != nil {
		return false
	}

	return version.GTE(configtry.MinTalosVersion)
}

// applyOptions describes one call into the machine's ApplyConfiguration RPC.
type applyOptions struct {
	// deadline, when set, is the latest point at which the apply may reach the machine: a confirm that arrives after the rollback would persist the tried config without any check.
	deadline time.Time

	// expectBootID, when set, aborts the apply unless the machine still reports this boot ID. It is
	// how a try mode apply is confirmed: a machine that rebooted has already dropped the try config.
	expectBootID string

	mode machineapi.ApplyConfigurationRequest_Mode
}

// applyResult is what the machine reported back, plus the boot ID observed on the way in.
type applyResult struct {
	// startedAt is taken right before the apply RPC, so a try window recorded from it never runs ahead of the machine's rollback timer.
	startedAt time.Time
	bootID    string
	mode      machineapi.ApplyConfigurationRequest_Mode
}

//nolint:gocyclo,cyclop
func (ctrl *StatusController) applyConfig(inputCtx context.Context,
	logger *zap.Logger,
	r controller.ReaderWriter,
	rc *ReconciliationContext,
	opts applyOptions,
) (applyResult, error) {
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
		return applyResult{}, xerrors.NewTagged[qtransform.SkipReconcileTag](
			fmt.Errorf("machine '%s' is in %s stage", rc.ID(), rc.machineStatusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage()),
		)
	}

	if rc.machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 != "" && applyMaintenance {
		return applyResult{}, fmt.Errorf("failed to apply machine config: the machine is expected to be running in the normal mode, but is running in maintenance")
	}

	c, err := ctrl.getClient(ctx, r, applyMaintenance, rc.machineStatus, rc.machineConfig)
	if err != nil {
		return applyResult{}, fmt.Errorf("failed to get client: %w", err)
	}

	defer logClose(c, logger, fmt.Sprintf("machine '%s'", rc.ID()))

	_, err = c.Version(ctx)
	if err != nil {
		return applyResult{}, err
	}

	// the stage above is Omni's view, which lags a machine that has just rebooted into maintenance
	if opts.mode == machineapi.ApplyConfigurationRequest_TRY {
		inMaintenance, maintenanceErr := talosutils.InMaintenance(ctx, c.COSI)
		if maintenanceErr != nil {
			return applyResult{}, fmt.Errorf("failed to read the stage of machine '%s': %w", rc.ID(), maintenanceErr)
		}

		if inMaintenance {
			return applyResult{}, fmt.Errorf("machine '%s' is in maintenance, where a try of the config cannot be rolled back", rc.ID())
		}
	}

	data, err := rc.machineConfig.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return applyResult{}, err
	}

	defer data.Free()

	result := applyResult{}

	result.bootID, err = talosutils.CheckBeforeApply(
		ctx,
		func(ctx context.Context) (string, error) { return talosutils.GetBootID(ctx, c.COSI) },
		func(ctx context.Context) (bool, error) { return talosutils.ConfigIsActive(ctx, c.COSI, data.Data()) },
		opts.mode, opts.expectBootID,
	)
	if err != nil {
		return applyResult{}, fmt.Errorf("machine '%s': %w", rc.ID(), err)
	}

	applyTimeout := time.Minute
	if opts.mode == machineapi.ApplyConfigurationRequest_TRY {
		// a config that cuts the machine off from Omni might also lose the reply to the try apply,
		// so the try is recorded only when this timeout expires, and that has to happen before the machine rolls it back
		applyTimeout = 15 * time.Second
	}

	ctx, applyCancel := context.WithTimeout(inputCtx, applyTimeout)
	defer applyCancel()

	if !opts.deadline.IsZero() {
		var deadlineCancel context.CancelFunc

		ctx, deadlineCancel = context.WithDeadline(ctx, opts.deadline)
		defer deadlineCancel()
	}

	if err = ctrl.acquireConfigUpdateLock(ctx, r, rc); err != nil {
		return applyResult{}, err
	}

	request := &machineapi.ApplyConfigurationRequest{
		Data: data.Data(),
		Mode: opts.mode,
	}

	if opts.mode == machineapi.ApplyConfigurationRequest_TRY {
		request.TryModeTimeout = durationpb.New(ctrl.tryTimings.Timeout)
	}

	result.startedAt = time.Now()

	resp, err := c.ApplyConfiguration(ctx, request)
	if err != nil {
		if opts.mode == machineapi.ApplyConfigurationRequest_TRY && talosutils.ApplyMayHaveLanded(err) {
			// the try has to be recorded, so that the rollback is waited out instead of the same try being sent again
			logger.Warn("no answer to the try mode apply, assuming the config reached the machine", zap.String("machine", rc.ID()), zap.Error(err))

			result.mode = opts.mode

			return result, nil
		}

		logger.Error(
			"apply config failed",
			zap.String("machine", rc.ID()),
			zap.Error(err),
			zap.Stringer("config_version", rc.machineConfig.Metadata().Version()),
			zap.Stringer("mode", opts.mode),
		)

		return applyResult{}, fmt.Errorf("failed to apply config to machine '%s': %w", rc.ID(), err)
	}

	if len(resp.Messages) != 1 {
		return applyResult{}, fmt.Errorf("unexpected number of responses: %d", len(resp.Messages))
	}

	result.mode = resp.Messages[0].GetMode()
	logger.Info(
		"applied machine config",
		zap.String("machine", rc.ID()),
		zap.Stringer("config_version", rc.machineConfig.Metadata().Version()),
		zap.Stringer("mode", result.mode),
		zap.Stringer("requested_mode", opts.mode),
	)

	return result, nil
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

	// if is control plane first leave etcd
	if isControlPlane && ctrl.ongoingResets.shouldLeaveEtcd(machineID) {
		ctrl.ongoingResets.handleEtcdLeave(machineID)

		err = ctrl.gracefulEtcdLeave(ctx, c, machineID)
		if err != nil {
			return controller.NewRequeueError(err, time.Second)
		}
	}

	return ctrl.resetMachine(ctx, c, logger, graceful, machineID, machineStatus.TypedSpec().Value.TalosVersion)
}

func (ctrl *StatusController) resetMachine(
	ctx context.Context,
	c *client.Client,
	logger *zap.Logger,
	graceful bool,
	machineID resource.ID,
	talosVersionStr string,
) error {
	resetRequest := &machineapi.ResetRequest{
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
	}

	talosVersion, err := semver.ParseTolerant(talosVersionStr)
	if err != nil {
		return fmt.Errorf("failed to parse talos version %q: %w", talosVersionStr, err)
	}

	if talosVersion.GTE(semver.MustParse("1.14.0-beta.1")) {
		resetRequest.SystemPartitionsToWipe = append(
			resetRequest.SystemPartitionsToWipe,
			&machineapi.ResetPartitionSpec{Label: constants.CRIContainerdVolumeID, Wipe: true},
			&machineapi.ResetPartitionSpec{Label: constants.KubeletDataVolumeID, Wipe: true},
			&machineapi.ResetPartitionSpec{Label: constants.EtcdDataVolumeID, Wipe: true},
			&machineapi.ResetPartitionSpec{Label: constants.LogVolumeID, Wipe: true},
		)
	}

	err = c.ResetGeneric(ctx, resetRequest)
	if err != nil {
		logger.Error(
			"failed resetting node",
			zap.Error(err),
		)

		return fmt.Errorf("failed resetting node '%s': %w", machineID, err)
	}

	attempt := ctrl.ongoingResets.handleReset(machineID)
	logger.Info("resetting node", zap.Uint("attempt", attempt), zap.Bool("graceful", graceful))

	return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("check back when machine '%s' gets into maintenance mode", machineID)
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

	// The published quota subtracts every not-ready machine in the cluster and disappears entirely
	// while control planes are updating, both to protect healthy machines from a rollout that would
	// degrade the cluster further. A machine that is itself not ready needs no such protection: the
	// pending update is its recovery path (e.g. reverting a kernel args change that broke it), and
	// blocking on its own unhealthiness would leave it stuck forever. Guarantee it one upgrade slot;
	// the finalizer count below still bounds the concurrency cluster-wide, so broken machines
	// recover one by one, and a recovery may also have to wait out an upgrade already in flight.
	if quota < 1 && rc.upgradeBlockedByOwnHealth() {
		quota = 1
	}

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

		return res.TypedSpec().Value.SetUncompressedData([]byte(configDiff))
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

		// Talos 1.14 and later own the key and do not allow deleting it via the API, they drop it themselves once the machine is running and ready
		if status.Code(err) == codes.PermissionDenied {
			logger.Debug("upgrade meta key is not removed, not writeable via the API in the Talos version", zap.String("machine", rc.ID()))

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
