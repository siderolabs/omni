// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// ClusterMachineStatusController reflects the status of a machine that is a member of a cluster.
type ClusterMachineStatusController = qtransform.QController[*omni.ClusterMachine, *omni.ClusterMachineStatus]

const clusterMachineStatusControllerName = "ClusterMachineStatusController"

// NewClusterMachineStatusController initializes ClusterMachineStatusController.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func NewClusterMachineStatusController() *ClusterMachineStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachine, *omni.ClusterMachineStatus]{
			Name: clusterMachineStatusControllerName,
			MapMetadataFunc: func(clusterMachine *omni.ClusterMachine) *omni.ClusterMachineStatus {
				return omni.NewClusterMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterMachineStatus *omni.ClusterMachineStatus) *omni.ClusterMachine {
				return omni.NewClusterMachine(resources.DefaultNamespace, clusterMachineStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, clusterMachine *omni.ClusterMachine, clusterMachineStatus *omni.ClusterMachineStatus) error {
				machine, err := safe.ReaderGet[*omni.Machine](ctx, r, resource.NewMetadata(resources.DefaultNamespace, omni.MachineType, clusterMachine.Metadata().ID(), resource.VersionUndefined))
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				helpers.CopyLabels(clusterMachine, clusterMachineStatus,
					omni.LabelCluster,
					omni.LabelControlPlaneRole,
					omni.LabelWorkerRole,
					omni.LabelMachineSet,
				)

				_, updateLocked := clusterMachine.Metadata().Labels().Get(omni.UpdateLocked)

				if updateLocked {
					clusterMachineStatus.Metadata().Labels().Set(omni.UpdateLocked, "")
				} else {
					clusterMachineStatus.Metadata().Labels().Delete(omni.UpdateLocked)
				}

				if machine != nil && machine.TypedSpec().Value.Connected {
					clusterMachineStatus.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
				} else {
					clusterMachineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelConnected)
				}

				cmsVal := clusterMachineStatus.TypedSpec().Value

				machineStatus, err := safe.ReaderGet[*omni.MachineStatus](
					ctx, r,
					resource.NewMetadata(resources.DefaultNamespace, omni.MachineStatusType, clusterMachine.Metadata().ID(), resource.VersionUndefined),
				)
				if err != nil {
					if state.IsNotFoundError(err) { // ignore, as another reconciliation will be triggered once it appears
						cmsVal.Stage = specs.ClusterMachineStatusSpec_UNKNOWN
						cmsVal.ApidAvailable = false
						cmsVal.Ready = false
						cmsVal.IsRemoved = true

						return nil
					}

					return err
				}

				cmsVal.IsRemoved = machineStatus.Metadata().Phase() == resource.PhaseTearingDown

				machineSetNode, err := helpers.HandleInput[*omni.MachineSetNode](ctx, r, clusterMachineStatusControllerName, clusterMachine)
				if err != nil {
					return err
				}

				var locked bool
				if machineSetNode != nil {
					_, locked = machineSetNode.Metadata().Annotations().Get(omni.MachineLocked)
				}

				if locked {
					clusterMachineStatus.Metadata().Annotations().Set(omni.MachineLocked, "")
				} else {
					clusterMachineStatus.Metadata().Annotations().Delete(omni.MachineLocked)
				}

				if machineStatus.TypedSpec().Value.Network != nil {
					clusterMachineStatus.Metadata().Labels().Set(omni.LabelHostname, machineStatus.TypedSpec().Value.Network.Hostname)
				}

				cmsVal.ManagementAddress = machineStatus.TypedSpec().Value.ManagementAddress

				statusSnapshot, err := safe.ReaderGet[*omni.MachineStatusSnapshot](
					ctx, r,
					resource.NewMetadata(resources.DefaultNamespace, omni.MachineStatusSnapshotType, clusterMachine.Metadata().ID(), resource.VersionUndefined),
				)
				if err != nil {
					if state.IsNotFoundError(err) {
						cmsVal.Stage = specs.ClusterMachineStatusSpec_UNKNOWN
						cmsVal.ApidAvailable = false
						cmsVal.Ready = false

						return nil
					}

					return err
				}

				clusterMachineConfigStatus, err := safe.ReaderGet[*omni.ClusterMachineConfigStatus](
					ctx,
					r,
					omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, clusterMachineStatus.Metadata().ID()).Metadata(),
				)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if clusterMachineConfigStatus != nil {
					cmsVal.ConfigUpToDate = !isOutdated(clusterMachine, clusterMachineConfigStatus)
					cmsVal.LastConfigError = clusterMachineConfigStatus.TypedSpec().Value.LastConfigError
				} else {
					cmsVal.ConfigUpToDate = false
					cmsVal.LastConfigError = ""
				}

				cmsVal.ConfigApplyStatus = configApplyStatus(cmsVal)

				status := statusSnapshot.TypedSpec().Value.GetMachineStatus()

				cmsVal.ApidAvailable = false

				_, isControlPlaneNode := clusterMachineStatus.Metadata().Labels().Get(omni.LabelControlPlaneRole)
				if (status.GetStage() == machineapi.MachineStatusEvent_BOOTING || status.GetStage() == machineapi.MachineStatusEvent_RUNNING) && isControlPlaneNode {
					cmsVal.ApidAvailable = machine != nil && machine.TypedSpec().Value.Connected
				}

				// should we also mark it as not ready when the clustermachine is tearing down (?)
				cmsVal.Ready = status.GetStatus().GetReady() &&
					machine != nil && machine.TypedSpec().Value.Connected

				if clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
					cmsVal.Stage = specs.ClusterMachineStatusSpec_DESTROYING

					return nil
				}

				if machineSetNode == nil {
					cmsVal.Stage = specs.ClusterMachineStatusSpec_BEFORE_DESTROY

					return nil
				}

				switch status.GetStage() {
				case machineapi.MachineStatusEvent_UNKNOWN:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_UNKNOWN
				case machineapi.MachineStatusEvent_RESETTING:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_DESTROYING
				case machineapi.MachineStatusEvent_INSTALLING:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_INSTALLING
				case machineapi.MachineStatusEvent_UPGRADING:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_UPGRADING
				case machineapi.MachineStatusEvent_REBOOTING:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_REBOOTING
				case machineapi.MachineStatusEvent_SHUTTING_DOWN:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_SHUTTING_DOWN
				case machineapi.MachineStatusEvent_BOOTING:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_BOOTING
				case machineapi.MachineStatusEvent_RUNNING:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_RUNNING
				case machineapi.MachineStatusEvent_MAINTENANCE:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_CONFIGURING
				}

				clusterMachineIdentity, err := safe.ReaderGet[*omni.ClusterMachineIdentity](ctx, r, omni.NewClusterMachineIdentity(
					resources.DefaultNamespace,
					clusterMachineStatus.Metadata().ID(),
				).Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if clusterMachineIdentity != nil {
					clusterMachineStatus.Metadata().Labels().Set(omni.ClusterMachineStatusLabelNodeName, clusterMachineIdentity.TypedSpec().Value.Nodename)
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineStatusSnapshot, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.Machine, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineStatus, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterMachineConfigStatus, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterMachineIdentity, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineSetNode, *omni.ClusterMachine](),
		),
		qtransform.WithIgnoreTeardownUntil(ClusterMachineEncryptionKeyControllerName), // destroy the ClusterMachineStatus after the ClusterMachineEncryptionKey is destroyed
		qtransform.WithConcurrency(2),
	)
}

// configApplyStatus returns the config apply status for the given cluster machine status spec.
func configApplyStatus(spec *specs.ClusterMachineStatusSpec) specs.ConfigApplyStatus {
	switch {
	case spec.ConfigUpToDate:
		return specs.ConfigApplyStatus_APPLIED
	case spec.LastConfigError == "":
		return specs.ConfigApplyStatus_PENDING
	default:
		return specs.ConfigApplyStatus_FAILED
	}
}

func isOutdated(clusterMachine *omni.ClusterMachine, configStatus *omni.ClusterMachineConfigStatus) bool {
	return configStatus.TypedSpec().Value.ClusterMachineVersion != clusterMachine.Metadata().Version().String() || configStatus.TypedSpec().Value.LastConfigError != ""
}
