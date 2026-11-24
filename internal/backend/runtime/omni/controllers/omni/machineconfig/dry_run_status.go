// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configtry"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

const (
	dryRunTimeout       = 10 * time.Second
	dryRunRetryInterval = time.Minute
)

// TalosClientFactory creates Talos clients for cluster machines.
type TalosClientFactory interface {
	GetForMachine(ctx context.Context, machineID string) (*talos.Client, error)
}

// DryRunStatusController runs a Talos dry run of the pending config of the machines in dry-run mode.
type DryRunStatusController struct {
	*qtransform.QController[*omni.ClusterMachineConfig, *omni.DryRunStatus]

	clientFactory TalosClientFactory
}

// NewDryRunStatusController creates a new DryRunStatusController.
func NewDryRunStatusController(clientFactory TalosClientFactory) *DryRunStatusController {
	ctrl := &DryRunStatusController{
		clientFactory: clientFactory,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachineConfig, *omni.DryRunStatus]{
			Name: "DryRunStatusController",
			MapMetadataFunc: func(machineConfig *omni.ClusterMachineConfig) *omni.DryRunStatus {
				return omni.NewDryRunStatus(machineConfig.Metadata().ID())
			},
			UnmapMetadataFunc: func(dryRunStatus *omni.DryRunStatus) *omni.ClusterMachineConfig {
				return omni.NewClusterMachineConfig(dryRunStatus.Metadata().ID())
			},
			TransformFunc: ctrl.transform,
		},
		qtransform.WithConcurrency(8),
		qtransform.WithExtraMappedInput[*omni.Cluster](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineStatus](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineStatusSnapshot](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfigStatus](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
		qtransform.WithExtraMappedInput[*omni.MachinePendingUpdates](
			qtransform.MapperSameID[*omni.ClusterMachineConfig](),
		),
	)

	return ctrl
}

func (ctrl *DryRunStatusController) transform(ctx context.Context, r controller.Reader, logger *zap.Logger,
	machineConfig *omni.ClusterMachineConfig, dryRunStatus *omni.DryRunStatus,
) error {
	id := machineConfig.Metadata().ID()

	machineConfigStatus, err := safe.ReaderGetByID[*omni.ClusterMachineConfigStatus](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	dryRun, err := ctrl.inDryRunMode(ctx, r, machineConfig, machineConfigStatus)
	if err != nil {
		return err
	}

	if !dryRun {
		return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("the machine is not in dry-run mode")
	}

	helpers.CopyLabels(machineConfig, dryRunStatus, omni.LabelCluster, omni.LabelMachineSet)

	spec := dryRunStatus.TypedSpec().Value

	if generationError := machineConfig.TypedSpec().Value.GenerationError; generationError != "" {
		ctrl.setFailed(spec, generationError)

		return nil
	}

	machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// checked before the pending diff, which goes stale while the machine is not reachable
	reason, checkAgainAfter, err := ctrl.notCheckedReason(ctx, r, id, machineStatus, machineConfigStatus)
	if err != nil {
		return err
	}

	if reason != specs.DryRunStatusSpec_NONE {
		ctrl.setNotChecked(spec, reason)

		if checkAgainAfter > 0 {
			return controller.NewRequeueInterval(checkAgainAfter)
		}

		return nil
	}

	pendingUpdates, err := safe.ReaderGetByID[*omni.MachinePendingUpdates](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// the diff is made from configs with the secrets redacted, so a change to secrets only reads as no pending changes
	if pendingUpdates == nil || !pendingUpdates.TypedSpec().Value.HasConfigDiff() {
		ctrl.setResult(spec, specs.DryRunStatusSpec_NO_PENDING_CHANGES)

		return nil
	}

	buffer, err := machineConfig.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return ctrl.recordCheckError(logger, spec, id, fmt.Errorf("failed to get uncompressed config: %w", err))
	}

	defer buffer.Free()

	shaSum := sha256.Sum256(buffer.Data())
	configSha256 := hex.EncodeToString(shaSum[:])

	if ctrl.alreadyChecked(spec, configSha256) {
		return nil
	}

	if err = ctrl.runDryRun(ctx, logger, spec, id, buffer.Data()); err != nil {
		return err
	}

	spec.ClusterMachineConfigSha256 = configSha256

	return nil
}

// alreadyChecked does not compare the Talos version: changing it reboots the machine, which resets the result through NOT_RUNNING.
func (ctrl *DryRunStatusController) alreadyChecked(spec *specs.DryRunStatusSpec, configSha256 string) bool {
	checked := spec.Outcome == specs.DryRunStatusSpec_PASSED || spec.Outcome == specs.DryRunStatusSpec_FAILED

	return checked && spec.ClusterMachineConfigSha256 == configSha256
}

func (ctrl *DryRunStatusController) runDryRun(ctx context.Context, logger *zap.Logger, spec *specs.DryRunStatusSpec, id string, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, dryRunTimeout)
	defer cancel()

	c, err := ctrl.clientFactory.GetForMachine(ctx, id)
	if err != nil {
		return ctrl.recordCheckError(logger, spec, id, fmt.Errorf("failed to get client: %w", err))
	}

	defer logClose(c, logger, fmt.Sprintf("machine '%s'", id))

	resp, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
		Data:   data,
		Mode:   machineapi.ApplyConfigurationRequest_AUTO,
		DryRun: true,
	})

	if grpcStatus := client.Status(err); grpcStatus != nil && grpcStatus.Code() == codes.InvalidArgument {
		ctrl.setFailed(spec, strings.TrimSpace(grpcStatus.Message()))

		return nil
	}

	if err != nil {
		return ctrl.recordCheckError(logger, spec, id, err)
	}

	ctrl.setResult(spec, specs.DryRunStatusSpec_PASSED)

	spec.RebootRequired = len(resp.GetMessages()) > 0 && resp.GetMessages()[0].GetMode() != machineapi.ApplyConfigurationRequest_NO_REBOOT

	return nil
}

// notCheckedReason also returns when to check again, for the reasons no input change clears.
func (ctrl *DryRunStatusController) notCheckedReason(ctx context.Context, r controller.Reader, id string,
	machineStatus *omni.MachineStatus, machineConfigStatus *omni.ClusterMachineConfigStatus,
) (specs.DryRunStatusSpec_NotCheckedReason, time.Duration, error) {
	if machineStatus == nil || !machineStatus.TypedSpec().Value.Connected {
		return specs.DryRunStatusSpec_DISCONNECTED, 0, nil
	}

	if machineStatus.TypedSpec().Value.Maintenance {
		return specs.DryRunStatusSpec_MAINTENANCE, 0, nil
	}

	snapshot, err := safe.ReaderGetByID[*omni.MachineStatusSnapshot](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, 0, err
	}

	var stage machineapi.MachineStatusEvent_MachineStage

	if snapshot != nil {
		stage = snapshot.TypedSpec().Value.GetMachineStatus().GetStage()
	}

	// the snapshot reports maintenance before MachineStatus does
	if stage == machineapi.MachineStatusEvent_MAINTENANCE {
		return specs.DryRunStatusSpec_MAINTENANCE, 0, nil
	}

	if stage != machineapi.MachineStatusEvent_BOOTING && stage != machineapi.MachineStatusEvent_RUNNING {
		return specs.DryRunStatusSpec_NOT_RUNNING, 0, nil
	}

	if machineConfigStatus == nil {
		return specs.DryRunStatusSpec_NONE, 0, nil
	}

	now := time.Now()

	configTry := machineConfigStatus.TypedSpec().Value.GetConfigTry()
	if !configtry.Default.InFlight(configTry, now) {
		return specs.DryRunStatusSpec_NONE, 0, nil
	}

	return specs.DryRunStatusSpec_TRY_IN_FLIGHT, configtry.Default.RollbackEnd(configTry).Sub(now), nil
}

// recordCheckError records the error instead of returning it, as an error return would leave the result of the previous config visible.
func (ctrl *DryRunStatusController) recordCheckError(logger *zap.Logger, spec *specs.DryRunStatusSpec, id string, err error) error {
	logger.Warn("dry run failed", zap.String("machine", id), zap.Error(err))

	ctrl.setNotChecked(spec, specs.DryRunStatusSpec_CHECK_ERROR)

	spec.Error = err.Error()

	return controller.NewRequeueInterval(dryRunRetryInterval)
}

func (ctrl *DryRunStatusController) inDryRunMode(ctx context.Context, r controller.Reader,
	machineConfig *omni.ClusterMachineConfig, machineConfigStatus *omni.ClusterMachineConfigStatus,
) (bool, error) {
	clusterName, ok := machineConfig.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return false, nil
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil && !state.IsNotFoundError(err) {
		return false, err
	}

	if cluster != nil && clusterLocked(cluster) {
		return true, nil
	}

	if machineConfigStatus == nil {
		return false, nil
	}

	machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, r, machineConfig.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return false, err
	}

	if machineSetNode == nil {
		return false, nil
	}

	return machineLocked(machineSetNode, machineConfigStatus), nil
}

func (ctrl *DryRunStatusController) setResult(spec *specs.DryRunStatusSpec, outcome specs.DryRunStatusSpec_Outcome) {
	spec.Outcome = outcome
	spec.NotCheckedReason = specs.DryRunStatusSpec_NONE
	spec.Error = ""
	spec.RebootRequired = false
	spec.ClusterMachineConfigSha256 = ""
}

func (ctrl *DryRunStatusController) setFailed(spec *specs.DryRunStatusSpec, message string) {
	ctrl.setResult(spec, specs.DryRunStatusSpec_FAILED)

	spec.Error = message
}

func (ctrl *DryRunStatusController) setNotChecked(spec *specs.DryRunStatusSpec, reason specs.DryRunStatusSpec_NotCheckedReason) {
	ctrl.setResult(spec, specs.DryRunStatusSpec_NOT_CHECKED)

	spec.NotCheckedReason = reason
}
