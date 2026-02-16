// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"

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
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
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
				return omni.NewClusterMachineStatus(clusterMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterMachineStatus *omni.ClusterMachineStatus) *omni.ClusterMachine {
				return omni.NewClusterMachine(clusterMachineStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, clusterMachine *omni.ClusterMachine, clusterMachineStatus *omni.ClusterMachineStatus) error {
				machine, err := safe.ReaderGet[*omni.Machine](ctx, r, resource.NewMetadata(resources.DefaultNamespace, omni.MachineType, clusterMachine.Metadata().ID(), resource.VersionUndefined))
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, r, clusterMachine.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				helpers.CopyLabels(clusterMachine, clusterMachineStatus,
					omni.LabelCluster,
					omni.LabelControlPlaneRole,
					omni.LabelWorkerRole,
					omni.LabelMachineSet,
				)

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

				clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					return errors.New("failed to get cluster from the cluster machine")
				}

				cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if !locked {
					if cluster != nil {
						_, locked = cluster.Metadata().Annotations().Get(omni.ClusterLocked)
					}
				}

				if locked {
					clusterMachineStatus.Metadata().Annotations().Set(omni.MachineLocked, "")
				} else {
					clusterMachineStatus.Metadata().Annotations().Delete(omni.MachineLocked)
				}

				pendingMachineUpdate, err := safe.ReaderGetByID[*omni.MachinePendingUpdates](ctx, r, clusterMachine.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if pendingMachineUpdate != nil && locked {
					clusterMachineStatus.Metadata().Labels().Set(omni.UpdateLocked, "")
				} else {
					clusterMachineStatus.Metadata().Labels().Delete(omni.UpdateLocked)
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
					omni.NewClusterMachineConfigStatus(clusterMachineStatus.Metadata().ID()).Metadata(),
				)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				clusterMachineConfig, err := safe.ReaderGet[*omni.ClusterMachineConfig](
					ctx,
					r,
					omni.NewClusterMachineConfig(clusterMachineStatus.Metadata().ID()).Metadata(),
				)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if clusterMachineConfigStatus != nil && clusterMachineConfig != nil {
					cmsVal.ConfigUpToDate = !isOutdated(clusterMachineConfig, clusterMachineConfigStatus)
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

				// if the machine snapshot has a power stage set (not NONE), overwrite the stage
				switch statusSnapshot.TypedSpec().Value.PowerStage {
				case specs.MachineStatusSnapshotSpec_POWER_STAGE_NONE:
					// do not override
				case specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_POWERED_OFF
				case specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERING_ON:
					cmsVal.Stage = specs.ClusterMachineStatusSpec_POWERING_ON
				}

				clusterMachineIdentity, err := safe.ReaderGet[*omni.ClusterMachineIdentity](ctx, r, omni.NewClusterMachineIdentity(
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
		qtransform.WithExtraMappedInput[*omni.MachineStatusSnapshot](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.Machine](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineStatus](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfigStatus](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfig](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineIdentity](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*infra.Machine](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*infra.MachineStatus](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.MachinePendingUpdates](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.Cluster](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*infra.MachineRequestStatus](
			qtransform.MapperFuncFromTyped(
				func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, request *infra.MachineRequestStatus) ([]resource.Pointer, error) {
					if request.TypedSpec().Value.Id == "" {
						return nil, nil
					}

					return []resource.Pointer{
						omni.NewClusterMachine(request.TypedSpec().Value.Id).Metadata(),
					}, nil
				},
			),
		),
		qtransform.WithIgnoreTeardownUntil(ClusterMachineEncryptionKeyControllerName), // destroy the ClusterMachineStatus after the ClusterMachineEncryptionKey is destroyed
		qtransform.WithConcurrency(2),
	)
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

func isOutdated(clusterMachineConfig *omni.ClusterMachineConfig, configStatus *omni.ClusterMachineConfigStatus) bool {
	return configStatus.TypedSpec().Value.ClusterMachineConfigVersion != clusterMachineConfig.Metadata().Version().String() || configStatus.TypedSpec().Value.LastConfigError != ""
}
