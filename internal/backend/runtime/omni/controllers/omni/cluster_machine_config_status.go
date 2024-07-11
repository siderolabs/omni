// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

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
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	talosutils "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

const (
	gracefulResetAttemptCount = 4
	etcdLeaveAttemptsLimit    = 2
	maintenanceCheckAttempts  = 5
)

// ClusterMachineConfigStatusController manages ClusterMachineStatus resource lifecycle.
//
// ClusterMachineConfigStatusController applies the generated machine config  on each corresponding machine.
type ClusterMachineConfigStatusController = qtransform.QController[*omni.ClusterMachineConfig, *omni.ClusterMachineConfigStatus]

// NewClusterMachineConfigStatusController initializes ClusterMachineConfigStatusController.
//
//nolint:gocognit,gocyclo,cyclop
func NewClusterMachineConfigStatusController() *ClusterMachineConfigStatusController {
	ongoingResets := &ongoingResets{
		statuses: map[string]*resetStatus{},
	}

	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachineConfig, *omni.ClusterMachineConfigStatus]{
			Name: "ClusterMachineConfigStatusController",
			MapMetadataFunc: func(machineConfig *omni.ClusterMachineConfig) *omni.ClusterMachineConfigStatus {
				return omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, machineConfig.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineConfigStatus *omni.ClusterMachineConfigStatus) *omni.ClusterMachineConfig {
				return omni.NewClusterMachineConfig(resources.DefaultNamespace, machineConfigStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *omni.ClusterMachineConfig, configStatus *omni.ClusterMachineConfigStatus) error {
				handler := clusterMachineConfigStatusControllerHandler{
					r:             r,
					logger:        logger,
					ongoingResets: ongoingResets,
				}

				if machineConfig.TypedSpec().Value.GenerationError != "" {
					configStatus.TypedSpec().Value.LastConfigError = machineConfig.TypedSpec().Value.GenerationError

					return nil
				}

				statusSnapshot, err := safe.ReaderGet[*omni.MachineStatusSnapshot](ctx, r, omni.NewMachineStatusSnapshot(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata())
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' machine status snapshot not found: %w", machineConfig.Metadata().ID(), err)
					}

					return fmt.Errorf("failed to get machine status snapshot '%s': %w", machineConfig.Metadata().ID(), err)
				}

				machineStatus, err := safe.ReaderGet[*omni.MachineStatus](ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata())
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' machine status not found: %w", machineConfig.Metadata().ID(), err)
					}

					return fmt.Errorf("failed to get machine status '%s': %w", machineConfig.Metadata().ID(), err)
				}

				if !machineStatus.TypedSpec().Value.Connected {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' machine is not connected", machineConfig.Metadata().ID())
				}

				if machineStatus.TypedSpec().Value.Schematic == nil {
					logger.Error("machine schematic is not set, skip reconcile")

					return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine status '%s' does not have schematic information", machineConfig.Metadata().ID()))
				}

				genOptions, err := safe.ReaderGet[*omni.MachineConfigGenOptions](ctx, r, omni.NewMachineConfigGenOptions(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata())
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' machine config gen options not found: %w", machineConfig.Metadata().ID(), err)
					}

					return fmt.Errorf("failed to get install image '%s': %w", machineConfig.Metadata().ID(), err)
				}

				installImage := genOptions.TypedSpec().Value.InstallImage
				if installImage == nil {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' install image not found", machineConfig.Metadata().ID())
				}

				// compatibility code for the machines having extensions that bypass the image factory
				// drop when there's no such machine, or when we are able to detect schematic id for the machines of that kind
				expectedSchematic := installImage.SchematicId
				if machineStatus.TypedSpec().Value.Schematic.Invalid {
					expectedSchematic = ""
				}

				versionMismatch := strings.TrimLeft(machineStatus.TypedSpec().Value.TalosVersion, "v") != configStatus.TypedSpec().Value.TalosVersion ||
					configStatus.TypedSpec().Value.TalosVersion != installImage.TalosVersion ||
					configStatus.TypedSpec().Value.SchematicId != expectedSchematic ||
					machineStatus.TypedSpec().Value.Schematic.Id != expectedSchematic

				// don't run the upgrade check if the running version and expected versions match
				if versionMismatch && installImage.TalosVersion != "" {
					inSync, err := handler.syncInstallImageAndSchematic(ctx, configStatus, machineStatus, machineConfig, statusSnapshot, installImage)
					if err != nil {
						return err
					}

					if !inSync {
						logger.Info("the machine talos version is out of sync, the config is not applied",
							zap.String("machine", machineConfig.Metadata().ID()),
						)

						return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' the machine talos version is out of sync: %w", machineConfig.Metadata().ID(), err)
					}
				}

				shaSum := sha256.Sum256(machineConfig.TypedSpec().Value.Data)
				shaSumString := hex.EncodeToString(shaSum[:])

				if configStatus.TypedSpec().Value.ClusterMachineConfigSha256 == shaSumString {
					// config is already applied

					return nil
				}

				// perform config apply
				if err := handler.applyConfig(ctx, machineStatus, machineConfig, statusSnapshot); err != nil {
					grpcSt := client.Status(err)
					if grpcSt != nil && grpcSt.Code() == codes.InvalidArgument {
						configStatus.TypedSpec().Value.LastConfigError = grpcSt.Message()

						return nil
					}

					return fmt.Errorf("failed to apply config to machine '%s': %w", machineConfig.Metadata().ID(), err)
				}

				helpers.CopyLabels(machineConfig, configStatus, omni.LabelMachineSet, omni.LabelCluster, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

				configStatus.TypedSpec().Value.ClusterMachineVersion = machineConfig.TypedSpec().Value.ClusterMachineVersion
				configStatus.TypedSpec().Value.ClusterMachineConfigVersion = machineConfig.Metadata().Version().String()
				configStatus.TypedSpec().Value.ClusterMachineConfigSha256 = shaSumString

				configStatus.TypedSpec().Value.LastConfigError = ""

				return nil
			},
			FinalizerRemovalFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *omni.ClusterMachineConfig) error {
				handler := clusterMachineConfigStatusControllerHandler{
					r:             r,
					logger:        logger,
					ongoingResets: ongoingResets,
				}

				clusterMachine, err := safe.ReaderGet[*omni.ClusterMachine](ctx, r, omni.NewClusterMachine(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata())
				if err != nil {
					return fmt.Errorf("finalizer: failed to get cluster machine '%s': %w", machineConfig.Metadata().ID(), err)
				}

				// perform reset of the node
				err = handler.reset(ctx, machineConfig, clusterMachine)
				if err != nil {
					return err
				}

				// delete ongoing resets information if the machine was reset
				handler.ongoingResets.deleteStatus(clusterMachine.Metadata().ID())

				return nil
			},
		},
		qtransform.WithConcurrency(8),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterMachine, *omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineStatus, *omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineStatusSnapshot, *omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineConfigGenOptions, *omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.Machine, *omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.TalosConfig, *omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*siderolink.ConnectionParams](),
		),
	)
}

type resetStatus struct {
	resetAttempts            uint
	etcdLeaveAttempts        uint
	maintenanceCheckAttempts uint
}

type ongoingResets struct {
	statuses map[resource.ID]*resetStatus
	mu       sync.Mutex
}

func (r *ongoingResets) getStatus(id resource.ID) (*resetStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rs, ok := r.statuses[id]

	return rs, ok
}

func (r *ongoingResets) isGraceful(id resource.ID) bool {
	rs, ok := r.getStatus(id)
	if !ok {
		return true
	}

	return rs.resetAttempts < gracefulResetAttemptCount
}

func (r *ongoingResets) shouldLeaveEtcd(id string) bool {
	rs, ok := r.getStatus(id)
	if !ok {
		return true
	}

	return rs.etcdLeaveAttempts < etcdLeaveAttemptsLimit
}

func (r *ongoingResets) handleReset(id resource.ID) uint {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[id]; !ok {
		r.statuses[id] = &resetStatus{}
	}

	r.statuses[id].resetAttempts++

	return r.statuses[id].resetAttempts
}

func (r *ongoingResets) handleMaintenanceCheck(id resource.ID) uint {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[id]; !ok {
		r.statuses[id] = &resetStatus{}
	}

	r.statuses[id].maintenanceCheckAttempts++

	return r.statuses[id].maintenanceCheckAttempts
}

func (r *ongoingResets) handleEtcdLeave(id resource.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[id]; !ok {
		r.statuses[id] = &resetStatus{}
	}

	r.statuses[id].etcdLeaveAttempts++
}

func (r *ongoingResets) deleteStatus(id resource.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.statuses, id)
}

type clusterMachineConfigStatusControllerHandler struct {
	r             controller.Reader
	logger        *zap.Logger
	ongoingResets *ongoingResets
}

func (h *clusterMachineConfigStatusControllerHandler) syncInstallImageAndSchematic(inputCtx context.Context, configStatus *omni.ClusterMachineConfigStatus,
	machineStatus *omni.MachineStatus, machineConfig *omni.ClusterMachineConfig, statusSnapshot *omni.MachineStatusSnapshot, installImage *specs.MachineConfigGenOptionsSpec_InstallImage,
) (bool, error) {
	// use short timeout for the all API calls but upgrade to quickly skip "dead" nodes
	ctx, cancel := context.WithTimeout(inputCtx, 5*time.Second)
	defer cancel()

	var maintenance bool

	//nolint:exhaustive
	switch statusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage() {
	case machineapi.MachineStatusEvent_MAINTENANCE:
		maintenance = true
	case machineapi.MachineStatusEvent_BOOTING:
	case machineapi.MachineStatusEvent_RUNNING:
	default:
		return configStatus.TypedSpec().Value.TalosVersion != "", nil
	}

	if installImage.TalosVersion == "" {
		return false, xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine '%s' does not have talos version", machineConfig.Metadata().ID()))
	}

	c, err := h.getClient(ctx, maintenance, machineStatus, machineConfig)
	if err != nil {
		return false, fmt.Errorf("failed to get client: %w", err)
	}

	defer logClose(c, h.logger, fmt.Sprintf("machine '%s'", machineConfig.Metadata().ID()))

	expectedVersion := installImage.TalosVersion
	expectedSchematic := installImage.SchematicId

	actualVersion, err := getVersion(ctx, c)
	if err != nil {
		return false, err
	}

	params, err := safe.ReaderGetByID[*siderolink.ConnectionParams](ctx, h.r, siderolink.ConfigID)
	if err != nil {
		return false, err
	}

	schematicInfo, err := talosutils.GetSchematicInfo(ctx, c, siderolink.KernelArgs(params))
	if err != nil {
		if !errors.Is(err, talosutils.ErrInvalidSchematic) {
			return false, err
		}

		// compatibility code for the machines running extensions installed bypassing image factory
		// make schematic play no role in the checks
		expectedSchematic = ""
	}

	if actualVersion == expectedVersion && schematicInfo.Equal(expectedSchematic) {
		configStatus.TypedSpec().Value.TalosVersion = actualVersion
		configStatus.TypedSpec().Value.SchematicId = expectedSchematic

		return true, nil
	}

	image, err := buildInstallImage(machineStatus.Metadata().ID(), installImage, expectedVersion)
	if err != nil {
		return false, err
	}

	h.logger.Info("upgrading the machine",
		zap.String("from_version", actualVersion),
		zap.String("to_version", expectedVersion),
		zap.String("from_schematic", schematicInfo.ID),
		zap.String("to_schematic", expectedSchematic),
		zap.String("image", image),
		zap.String("machine", machineConfig.Metadata().ID()))

	// give the Upgrade API longer timeout, as it pulls the installer image before returning
	upgradeCtx, upgradeCancel := context.WithTimeout(inputCtx, 5*time.Minute)
	defer upgradeCancel()

	_, err = c.Upgrade(upgradeCtx, image, !maintenance, false, false)

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

func (h *clusterMachineConfigStatusControllerHandler) applyConfig(inputCtx context.Context,
	machineStatus *omni.MachineStatus, machineConfig *omni.ClusterMachineConfig, statusSnapshot *omni.MachineStatusSnapshot,
) error {
	ctx, cancel := context.WithTimeout(inputCtx, 5*time.Second)
	defer cancel()

	applyMaintenance := false

	switch statusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage() {
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
		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine '%s' is in %s stage", machineConfig.Metadata().ID(), statusSnapshot.TypedSpec().Value.GetMachineStatus().GetStage()))
	}

	c, err := h.getClient(ctx, applyMaintenance, machineStatus, machineConfig)
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}

	defer logClose(c, h.logger, fmt.Sprintf("machine '%s'", machineConfig.Metadata().ID()))

	_, err = c.Version(ctx)
	if err != nil {
		return err
	}

	ctx, applyCancel := context.WithTimeout(inputCtx, time.Minute)
	defer applyCancel()

	resp, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
		Data: machineConfig.TypedSpec().Value.Data,
		Mode: machineapi.ApplyConfigurationRequest_AUTO,
	})
	if err != nil {
		h.logger.Error("apply config failed",
			zap.String("machine", machineConfig.Metadata().ID()),
			zap.Error(err),
			zap.Stringer("config_version", machineConfig.Metadata().Version()),
		)

		return fmt.Errorf("failed to apply config to machine '%s': %w", machineConfig.Metadata().ID(), err)
	}

	if len(resp.Messages) != 1 {
		return fmt.Errorf("unexpected number of responses: %d", len(resp.Messages))
	}

	mode := resp.Messages[0].GetMode()
	h.logger.Info("applied machine config",
		zap.String("machine", machineConfig.Metadata().ID()),
		zap.Stringer("config_version", machineConfig.Metadata().Version()),
		zap.Stringer("mode", mode),
	)

	if mode != machineapi.ApplyConfigurationRequest_NO_REBOOT {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("applied config to machine '%s' in %s mode", machineConfig.Metadata().ID(), mode))
	}

	return nil
}

func logClose(c io.Closer, logger *zap.Logger, additional string) {
	if err := c.Close(); err != nil {
		logger.Error(additional+": failed to close client", zap.Error(err))
	}
}

//nolint:gocyclo,cyclop,gocognit
func (h *clusterMachineConfigStatusControllerHandler) reset(
	ctx context.Context,
	machineConfig *omni.ClusterMachineConfig,
	clusterMachine *omni.ClusterMachine,
) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	logger := h.logger.With(
		zap.String("machine", clusterMachine.Metadata().ID()),
	)

	machine, err := safe.ReaderGetByID[*omni.Machine](ctx, h.r, machineConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			// Machine is gone, means that we should just let it go
			logger.Info("removed without reset")

			return nil
		}

		return fmt.Errorf("failed to get machine '%s': %w", machineConfig.Metadata().ID(), err)
	}

	if machine.Metadata().Phase() == resource.PhaseTearingDown {
		// Machine is tearing down, means that we should just let it go
		logger.Info("removed without reset")

		return nil
	}

	machineStatus, err := safe.ReaderGet[*omni.MachineStatus](ctx, h.r, omni.NewMachineStatus(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			// MachineStatus is gone, means that we should just let it go
			logger.Info("removed without reset")

			return nil
		}

		return fmt.Errorf("failed to get machine status '%s': %w", machineConfig.Metadata().ID(), err)
	}

	if !machineStatus.TypedSpec().Value.Connected {
		// machine is not connected, so we can't reset it
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine '%s' is not connected", machineConfig.Metadata().ID())
	}

	statusSnapshot, err := safe.ReaderGet[*omni.MachineStatusSnapshot](ctx, h.r, omni.NewMachineStatusSnapshot(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine '%s' status snapshot is not found: %w", machineConfig.Metadata().ID(), err)
		}

		return fmt.Errorf("failed to get machine status snapshot '%s': %w", machineConfig.Metadata().ID(), err)
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
		c, err = h.getClient(ctx, true, machineStatus, machineConfig)
		if err != nil {
			return fmt.Errorf("failed to get maintenance client for machine '%s': %w", machineConfig.Metadata().ID(), err)
		}

		defer logClose(c, logger, "reset maintenance")

		_, err = c.Version(ctx)

		logger.Debug("maintenance mode check", zap.Error(err))

		if err == nil {
			// really in maintenance mode, no need to reset
			return nil
		}

		wrappedErr := fmt.Errorf("failed to get version in maintenance mode for machine '%s': %w", machineConfig.Metadata().ID(), err)

		attempt := h.ongoingResets.handleMaintenanceCheck(machineStatus.Metadata().ID())

		if attempt <= maintenanceCheckAttempts {
			// retry in N seconds
			return controller.NewRequeueError(wrappedErr, time.Second*time.Duration(attempt))
		}

		return xerrors.NewTagged[qtransform.SkipReconcileTag](wrappedErr)
	}

	machineSetName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return fmt.Errorf("failed to determine machine set of the cluster machine %s", clusterMachine.Metadata().ID())
	}

	machineSet, err := safe.ReaderGet[*omni.MachineSet](ctx, h.r, omni.NewMachineSet(resources.DefaultNamespace, machineSetName).Metadata())
	if err != nil {
		return fmt.Errorf("failed to get machine set '%s': %w", machineSetName, err)
	}

	graceful := machineSet.Metadata().Phase() == resource.PhaseRunning

	if !h.ongoingResets.isGraceful(clusterMachine.Metadata().ID()) {
		graceful = false
	}

	_, isControlPlane := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	switch {
	// check that machine is ready to be reset
	// if running allow reset always
	case machineStage == machineapi.MachineStatusEvent_RUNNING:
	// if booting allow only non-graceful reset for control plane nodes
	case (!graceful || !isControlPlane) && machineStage == machineapi.MachineStatusEvent_BOOTING:
	default:
		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine '%s' is in %s stage", machineConfig.Metadata().ID(), machineStage))
	}

	c, err = h.getClient(ctx, false, machineStatus, machineConfig)
	if err != nil {
		return fmt.Errorf("failed to get client for machine '%s': %w", machineConfig.Metadata().ID(), err)
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
			return fmt.Errorf("failed resetting node '%s': %w", machineConfig.Metadata().ID(), err)
		}
	}

	// if is control plane first leave etcd
	if isControlPlane && h.ongoingResets.shouldLeaveEtcd(clusterMachine.Metadata().ID()) {
		h.ongoingResets.handleEtcdLeave(clusterMachine.Metadata().ID())

		err = h.gracefulEtcdLeave(ctx, c, clusterMachine.Metadata().ID())
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
		attempt := h.ongoingResets.handleReset(clusterMachine.Metadata().ID())
		logger.Info("resetting node", zap.Uint("attempt", attempt), zap.Bool("graceful", graceful))

		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("check back when machine '%s' gets into maintenance mode", machineConfig.Metadata().ID())
	}

	logger.Error("failed resetting node",
		zap.Error(err),
	)

	return fmt.Errorf("failed resetting node '%s': %w", machineConfig.Metadata().ID(), err)
}

func (h *clusterMachineConfigStatusControllerHandler) gracefulEtcdLeave(ctx context.Context, c *client.Client, id string) error {
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

func (h *clusterMachineConfigStatusControllerHandler) getClient(
	ctx context.Context,
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
				client.WithTLSConfig(insecureTLSConfig),
				client.WithEndpoints(address),
			)...)
	}

	clusterName, ok := machineConfig.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, errors.New("no cluster name label")
	}

	talosConfig, err := safe.ReaderGet[*omni.TalosConfig](ctx, h.r, omni.NewTalosConfig(resources.DefaultNamespace, clusterName).Metadata())
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
		return nil, fmt.Errorf("failed to create client to machine '%s': %w", machineStatus.Metadata().ID(), err)
	}

	return result, nil
}

var insecureTLSConfig = &tls.Config{
	InsecureSkipVerify: true,
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
