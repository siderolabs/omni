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
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
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

				machineSetNode, err := helpers.HandleInput[*omni.MachineSetNode](ctx, r, clusterMachineStatusControllerName, clusterMachine)
				if err != nil {
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

				if _, ok := machineStatus.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider); ok {
					clusterMachineStatus.Metadata().Labels().Set(omni.LabelIsManagedByStaticInfraProvider, "")
				} else {
					clusterMachineStatus.Metadata().Labels().Delete(omni.LabelIsManagedByStaticInfraProvider)
				}

				if err = updateMachineProvisionStatus(ctx, r, machineStatus, cmsVal); err != nil {
					return err
				}

				cmsVal.IsRemoved = machineStatus.Metadata().Phase() == resource.PhaseTearingDown

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

				if err = reconcilePoweringOn(ctx, r, clusterMachineStatus); err != nil {
					return err
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
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, rw controller.ReaderWriter, _ *zap.Logger, clusterMachine *omni.ClusterMachine) error {
				_, err := helpers.HandleInput[*omni.MachineSetNode](ctx, rw, clusterMachineStatusControllerName, clusterMachine)
				if err != nil {
					return err
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
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*infra.Machine, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*infra.MachineStatus, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, request *infra.MachineRequestStatus) ([]resource.Pointer, error) {
				if request.TypedSpec().Value.Id == "" {
					return nil, nil
				}

				return []resource.Pointer{
					omni.NewClusterMachine(resources.DefaultNamespace, request.TypedSpec().Value.Id).Metadata(),
				}, nil
			},
		),
		qtransform.WithIgnoreTeardownUntil(ClusterMachineEncryptionKeyControllerName), // destroy the ClusterMachineStatus after the ClusterMachineEncryptionKey is destroyed
		qtransform.WithConcurrency(2),
	)
}

// reconcilePoweringOn overwrites the machine stage if the machine is managed by the bare metal infra provider and is being powered on right now.
func reconcilePoweringOn(ctx context.Context, r controller.Reader, clusterMachineStatus *omni.ClusterMachineStatus) error {
	_, err := safe.ReaderGetByID[*infra.Machine](ctx, r, clusterMachineStatus.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	infraMachineStatus, err := safe.ReaderGetByID[*infra.MachineStatus](ctx, r, clusterMachineStatus.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// if the expected state is powered on, but the actual power state is off overwrite the cluster machine state to be "POWERING_ON"
	if infraMachineStatus == nil || infraMachineStatus.TypedSpec().Value.PowerState != specs.InfraMachineStatusSpec_POWER_STATE_ON {
		clusterMachineStatus.TypedSpec().Value.Stage = specs.ClusterMachineStatusSpec_POWERING_ON
	}

	return nil
}

func updateMachineProvisionStatus(ctx context.Context, r controller.Reader, machineStatus *omni.MachineStatus, cmsVal *specs.ClusterMachineStatusSpec) error {
	machineRequestID, ok := machineStatus.Metadata().Labels().Get(omni.LabelMachineRequest)

	cmsVal.ProvisionStatus = &specs.ClusterMachineStatusSpec_ProvisionStatus{}

	if ok {
		cmsVal.ProvisionStatus.RequestId = machineRequestID
	}

	machineRequestStatus, err := safe.ReaderGetByID[*infra.MachineRequestStatus](ctx, r, machineRequestID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if machineRequestStatus == nil {
		return nil
	}

	cmsVal.ProvisionStatus.ProviderId, ok = machineRequestStatus.Metadata().Labels().Get(omni.LabelInfraProviderID)

	if !ok {
		return nil
	}

	return nil
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
