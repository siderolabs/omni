// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machineconfig implements the Talos config apply controller for Omni runtime.
package machineconfig

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
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
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	talosutils "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

const (
	// ConfigUpdateFinalizer is set on the ClusterMachine when this controller acquires the config change lock.
	// By counting the machines with the finalizer we can tell how many machines are being updated simultaneously.
	ConfigUpdateFinalizer     = "ConfigUpdatePendingFinalizer"
	gracefulResetAttemptCount = 4
	etcdLeaveAttemptsLimit    = 2
	maintenanceCheckAttempts  = 5
)

// ClusterMachineConfigStatusController manages ClusterMachineStatus resource lifecycle.
//
// ClusterMachineConfigStatusController applies the generated machine config  on each corresponding machine.
type ClusterMachineConfigStatusController struct {
	*qtransform.QController[*omni.ClusterMachineConfig, *omni.ClusterMachineConfigStatus]
	ongoingResets    *ongoingResets
	imageFactoryHost string
	acquireLockMu    sync.Mutex
}

// NewClusterMachineConfigStatusController initializes ClusterMachineConfigStatusController.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func NewClusterMachineConfigStatusController(imageFactoryHost string) *ClusterMachineConfigStatusController {
	ongoingResets := &ongoingResets{
		statuses: map[string]*resetStatus{},
	}

	ctrl := &ClusterMachineConfigStatusController{
		imageFactoryHost: imageFactoryHost,
		ongoingResets:    ongoingResets,
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
		qtransform.WithExtraMappedInput[*siderolink.MachineJoinConfig](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, md controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				machineSetName, ok := md.Labels().Get(omni.LabelMachineSet)
				if !ok {
					return nil, nil
				}

				clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachineConfig](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelMachineSet, machineSetName),
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
		qtransform.WithExtraMappedInput[*omni.MachineSetStatus](
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
		qtransform.WithExtraMappedInput[*omni.ClusterStatus](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedDestroyReadyInput[*omni.MachinePendingUpdates](
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
func (ctrl *ClusterMachineConfigStatusController) reconcileRunning(
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

	if err = ctrl.computePendingUpdates(ctx, r, rc); err != nil {
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

	if rc.shouldUpgrade {
		inSync, syncErr := ctrl.upgrade(ctx, logger, r, rc)
		if syncErr != nil {
			return syncErr
		}

		if !inSync {
			logger.Info("the machine talos version is out of sync, the config is not applied",
				zap.String("machine", machineConfig.Metadata().ID()),
			)

			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' the machine talos version is out of sync: %w", machineConfig.Metadata().ID(), err)
		}
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

	if machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 != shaSumString { // latest config is not yet applied, perform config apply
		mode, err = ctrl.applyConfig(ctx, logger, r, rc)
		if err != nil {
			grpcSt := client.Status(err)
			if grpcSt != nil && grpcSt.Code() == codes.InvalidArgument {
				machineConfigStatus.TypedSpec().Value.LastConfigError = grpcSt.Message()

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

func (ctrl *ClusterMachineConfigStatusController) reconcileTearingDown(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, machineConfig *omni.ClusterMachineConfig) error {
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

func (ctrl *ClusterMachineConfigStatusController) upgrade(
	inputCtx context.Context,
	logger *zap.Logger,
	r controller.Reader,
	rc *ReconciliationContext,
) (bool, error) {
	// use short timeout for the all API calls but upgrade to quickly skip "dead" nodes
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
		return rc.machineConfigStatus.TypedSpec().Value.TalosVersion != "", nil
	}

	if rc.installImage.TalosVersion == "" {
		return false, xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine '%s' does not have talos version", rc.ID()))
	}

	c, err := ctrl.getClient(ctx, r, maintenance, rc.machineStatus, rc.machineConfig)
	if err != nil {
		return false, fmt.Errorf("failed to get client: %w", err)
	}

	defer logClose(c, logger, fmt.Sprintf("machine '%s'", rc.machineConfig.Metadata().ID()))

	expectedVersion := rc.installImage.TalosVersion
	expectedSchematic := rc.installImage.SchematicId

	if rc.machineStatus.TypedSpec().Value.Schematic.Invalid {
		rc.installImage.SchematicId = ""
	}

	actualVersion, err := getVersion(ctx, c)
	if err != nil {
		return false, err
	}

	params, err := safe.ReaderGetByID[*siderolink.MachineJoinConfig](ctx, r, rc.machineConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return false, err
	}

	schematicInfo, err := talosutils.GetSchematicInfo(ctx, c, params.TypedSpec().Value.Config.KernelArgs)
	if err != nil {
		if !errors.Is(err, talosutils.ErrInvalidSchematic) {
			return false, err
		}

		// compatibility code for the machines running extensions installed bypassing image factory
		// make schematic play no role in the checks
		expectedSchematic = ""
	}

	if actualVersion == expectedVersion && rc.SchematicEqual(schematicInfo, expectedSchematic) {
		rc.machineConfigStatus.TypedSpec().Value.TalosVersion = actualVersion
		rc.machineConfigStatus.TypedSpec().Value.SchematicId = expectedSchematic

		return true, nil
	}

	image, err := installimage.Build(ctrl.imageFactoryHost, rc.ID(), rc.installImage)
	if err != nil {
		return false, err
	}

	logger.Info("upgrading the machine",
		zap.String("from_version", actualVersion),
		zap.String("to_version", expectedVersion),
		zap.String("from_schematic", schematicInfo.ID),
		zap.String("to_schematic", expectedSchematic),
		zap.String("image", image),
		zap.String("machine", rc.ID()))

	// give the Upgrade API longer timeout, as it pulls the installer image before returning
	upgradeCtx, upgradeCancel := context.WithTimeout(inputCtx, 5*time.Minute)
	defer upgradeCancel()

	stageUpgrade, err := ctrl.stageUpgrade(actualVersion)
	if err != nil {
		return false, err
	}

	_, err = c.UpgradeWithOptions(upgradeCtx,
		client.WithUpgradeImage(image),
		client.WithUpgradePreserve(!maintenance),
		client.WithUpgradeStage(stageUpgrade),
		client.WithUpgradeForce(false),
	)

	// Failed Precondition means that the node is not in a state when the system can be upgraded.
	if status.Code(err) == codes.FailedPrecondition {
		return true, nil
	}

	// If upgrade is not implemented, it means that we run older Talos that doesn't support upgrades in maintenance mode.
	if status.Code(err) == codes.Unimplemented {
		return true, nil
	}

	return false, err
}

// stageUpgrade decides if the upgrade should be staged.
//
// Currently, it is only required as a workaround for this bug affecting Talos 1.9.0-1.9.2:
// https://github.com/siderolabs/talos/issues/10163.
func (ctrl *ClusterMachineConfigStatusController) stageUpgrade(actualTalosVersion string) (bool, error) {
	version, err := semver.ParseTolerant(actualTalosVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse talos version %q: %w", actualTalosVersion, err)
	}

	return version.Major == 1 && version.Minor == 9 && version.Patch < 3, nil
}

func (ctrl *ClusterMachineConfigStatusController) applyConfig(inputCtx context.Context,
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
		logger.Debug("skip applying the config, failed to acquire lock", zap.Error(err))

		return 0, err
	}

	resp, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
		Data: data.Data(),
		Mode: machineapi.ApplyConfigurationRequest_AUTO,
	})
	if err != nil {
		logger.Error("apply config failed",
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
	logger.Info("applied machine config",
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

//nolint:gocyclo,cyclop,gocognit,maintidx
func (ctrl *ClusterMachineConfigStatusController) reset(
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

	logger.Error("failed resetting node",
		zap.Error(err),
	)

	return fmt.Errorf("failed resetting node '%s': %w", machineID, err)
}

func (ctrl *ClusterMachineConfigStatusController) shouldReset(
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

	machine, err := safe.ReaderGetByID[*omni.Machine](ctx, r, machineID)
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

func (ctrl *ClusterMachineConfigStatusController) shouldResetGraceful(
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
		logger.Info("node is requested to be force destroyed")

		return false, nil
	}

	machineSetName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return false, fmt.Errorf("failed to determine machine set of the cluster machine %s", clusterMachine.Metadata().ID())
	}

	machineSetStatus, err := safe.ReaderGet[*omni.MachineSetStatus](ctx, r, omni.NewMachineSetStatus(machineSetName).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get machine set '%s': %w", machineSetName, err)
	}

	if !ctrl.ongoingResets.isGraceful(clusterMachine.Metadata().ID()) {
		return false, nil
	}

	return machineSetStatus != nil && machineSetStatus.Metadata().Phase() == resource.PhaseRunning &&
		machineSetStatus.TypedSpec().Value.Phase != specs.MachineSetPhase_Destroying, nil
}

func (h *ClusterMachineConfigStatusController) gracefulEtcdLeave(ctx context.Context, c *client.Client, id string) error {
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

func (ctrl *ClusterMachineConfigStatusController) getClient(
	ctx context.Context,
	r controller.Reader,
	useMaintenance bool,
	machineStatus *omni.MachineStatus,
	machineConfig *omni.ClusterMachineConfig,
) (*client.Client, error) {
	address := machineStatus.TypedSpec().Value.ManagementAddress
	opts := talos.GetSocketOptions(address)

	if useMaintenance {
		return client.New(ctx,
			append(
				opts,
				client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
				client.WithEndpoints(address),
			)...)
	}

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

func (ctrl *ClusterMachineConfigStatusController) acquireConfigUpdateLock(ctx context.Context, r controller.ReaderWriter,
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
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine set blocks the config changes")
	}

	machineSetName, ok := rc.clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return errors.New("failed to get machine set name from the cluster machine")
	}

	machineSetStatus, err := safe.ReaderGetByID[*omni.MachineSetStatus](ctx, r, machineSetName)
	if err != nil {
		return err
	}

	qruntime := r.(controller.QRuntime) //nolint:forcetypeassert,errcheck

	clusterMachines, err := qruntime.ListUncached(ctx,
		omni.NewClusterMachine("").Metadata(),
		state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSetName)),
	)
	if err != nil {
		return err
	}

	updateParallelism := getParallelismOrDefault(machineSetStatus.TypedSpec().Value.UpdateStrategy, machineSetStatus.TypedSpec().Value.UpdateStrategyConfig, 1)

	quota := updateParallelism

	pendingMachines := []string{}

	for _, clusterMachine := range clusterMachines.Items {
		if clusterMachine.Metadata().Finalizers().Has(ConfigUpdateFinalizer) {
			pendingMachines = append(pendingMachines, clusterMachine.Metadata().ID())

			quota--
		}

		if quota <= 0 {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("quota %d for updates reached, waiting for the locks to be released, pending: %#v", updateParallelism, pendingMachines)
		}
	}

	return r.AddFinalizer(ctx, rc.clusterMachine.Metadata(), ConfigUpdateFinalizer)
}

func (ctrl *ClusterMachineConfigStatusController) releaseConfigUpdateLock(ctx context.Context, r controller.ReaderWriter, clusterMachine *omni.ClusterMachine) error {
	if !clusterMachine.Metadata().Finalizers().Has(ConfigUpdateFinalizer) {
		return nil
	}

	return r.RemoveFinalizer(ctx, clusterMachine.Metadata(), ConfigUpdateFinalizer)
}

func (ctrl *ClusterMachineConfigStatusController) computePendingUpdates(ctx context.Context, r controller.ReaderWriter, rc *ReconciliationContext) error {
	pendingUpdates := omni.NewMachinePendingUpdates(rc.machineConfig.Metadata().ID())

	currentRedactedMachineConfig, err := rc.machineConfigStatus.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return err
	}

	configDiff, err := computeDiff(currentRedactedMachineConfig.Data(), rc.redactedMachineConfig)
	if err != nil {
		return err
	}

	var shouldUpgrade bool

	if rc.machineConfigStatus != nil && rc.installImage != nil {
		shouldUpgrade = rc.machineConfigStatus.TypedSpec().Value.SchematicId != rc.installImage.SchematicId ||
			rc.machineConfigStatus.TypedSpec().Value.TalosVersion != rc.installImage.TalosVersion
	}

	// if no pending changes, delete the pending updates resource
	if !shouldUpgrade && configDiff == "" {
		_, err = helpers.TeardownAndDestroy(ctx, r, pendingUpdates.Metadata())

		return err
	}

	return safe.WriterModify(ctx, r, pendingUpdates, func(res *omni.MachinePendingUpdates) error {
		helpers.CopyAllLabels(rc.machineConfig, res)

		if shouldUpgrade {
			res.TypedSpec().Value.Upgrade = &specs.MachinePendingUpdatesSpec_Upgrade{
				FromSchematic: rc.machineConfigStatus.TypedSpec().Value.SchematicId,
				ToSchematic:   rc.installImage.SchematicId,

				FromVersion: rc.machineConfigStatus.TypedSpec().Value.TalosVersion,
				ToVersion:   rc.installImage.TalosVersion,
			}
		} else {
			res.TypedSpec().Value.Upgrade = nil
		}

		defer currentRedactedMachineConfig.Free()

		res.TypedSpec().Value.ConfigDiff = configDiff

		return nil
	})
}

func (ctrl *ClusterMachineConfigStatusController) deleteUpgradeMetaKey(
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

func getParallelismOrDefault(strategyType specs.MachineSetSpec_UpdateStrategy, strategy *specs.MachineSetSpec_UpdateStrategyConfig, def int) int {
	if strategyType == specs.MachineSetSpec_Rolling {
		if strategy == nil {
			return def
		}

		return int(strategy.Rolling.MaxParallelism)
	}

	return def
}
