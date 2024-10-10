// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// ClusterMachineRequestStatusController reflects the status of the machine request which will be added to a machine set.
type ClusterMachineRequestStatusController = qtransform.QController[*infra.MachineRequest, *omni.ClusterMachineRequestStatus]

const clusterMachineRequestStatusControllerName = "ClusterMachineRequestStatusController"

// NewClusterMachineRequestStatusController initializes ClusterMachineRequestStatusController.
func NewClusterMachineRequestStatusController() *ClusterMachineRequestStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*infra.MachineRequest, *omni.ClusterMachineRequestStatus]{
			Name: clusterMachineRequestStatusControllerName,
			MapMetadataFunc: func(machineRequest *infra.MachineRequest) *omni.ClusterMachineRequestStatus {
				return omni.NewClusterMachineRequestStatus(resources.DefaultNamespace, machineRequest.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterMachineStatus *omni.ClusterMachineRequestStatus) *infra.MachineRequest {
				return infra.NewMachineRequest(clusterMachineStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger,
				machineRequest *infra.MachineRequest, clusterMachineStatus *omni.ClusterMachineRequestStatus,
			) error {
				machineSetName, ok := machineRequest.Metadata().Labels().Get(omni.LabelMachineRequestSet)
				if !ok {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("the machine request is not a part of the machine set")
				}

				machineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, r, machineSetName)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("the machine request set is not associated with the machine set")
					}

					return err
				}

				machineRequestStatus, err := safe.ReaderGetByID[*infra.MachineRequestStatus](ctx, r, machineRequest.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				clusterMachineStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_PENDING
				clusterMachineStatus.TypedSpec().Value.Status = "Waiting for the infra provider to start provision"

				switch {
				case machineRequest.Metadata().Phase() == resource.PhaseTearingDown:
					clusterMachineStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_DEPROVISIONING

					clusterMachineStatus.TypedSpec().Value.Status = "Waiting for the infra provider to finish teardown"
				case machineRequestStatus != nil:
					clusterMachineStatus.TypedSpec().Value.MachineUuid = machineRequestStatus.TypedSpec().Value.Id

					clusterMachineStatus.TypedSpec().Value.Status = machineRequestStatus.TypedSpec().Value.Status

					switch machineRequestStatus.TypedSpec().Value.Stage {
					case specs.MachineRequestStatusSpec_UNKNOWN:
						clusterMachineStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_PENDING
					case specs.MachineRequestStatusSpec_PROVISIONING:
						clusterMachineStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_PROVISIONING

					case specs.MachineRequestStatusSpec_PROVISIONED:
						clusterMachineStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_PROVISIONED

						clusterMachineStatus.TypedSpec().Value.Status = "Waiting for the machine to join Omni"
					case specs.MachineRequestStatusSpec_FAILED:
						clusterMachineStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_FAILED

						clusterMachineStatus.TypedSpec().Value.Status = fmt.Sprintf("Provision Failed: %s", machineRequestStatus.TypedSpec().Value.Error)
					}
				}

				clusterMachineStatus.Metadata().Labels().Set(omni.LabelMachineSet, machineSetName)

				clusterMachineStatus.TypedSpec().Value.ProviderId, _ = machineRequest.Metadata().Labels().Get(omni.LabelInfraProviderID)

				helpers.CopyLabels(machineSet, clusterMachineStatus, omni.LabelWorkerRole, omni.LabelControlPlaneRole, omni.LabelCluster)

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, res *omni.MachineSet) ([]resource.Pointer, error) {
				list, err := safe.ReaderListAll[*infra.MachineRequest](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelMachineRequestSet, res.Metadata().ID()),
				))
				if err != nil {
					return nil, err
				}

				return safe.ToSlice(list, func(item *infra.MachineRequest) resource.Pointer {
					return item.Metadata()
				}), nil
			},
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*infra.MachineRequestStatus, *infra.MachineRequest](),
		),
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, res *infra.ProviderStatus) ([]resource.Pointer, error) {
				list, err := safe.ReaderListAll[*infra.MachineRequest](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelInfraProviderID, res.Metadata().ID()),
				))
				if err != nil {
					return nil, err
				}

				return safe.ToSlice(list, func(item *infra.MachineRequest) resource.Pointer {
					return item.Metadata()
				}), nil
			},
		),
		qtransform.WithIgnoreTeardownUntil(ClusterMachineEncryptionKeyControllerName), // destroy the ClusterMachineRequestStatus after the ClusterMachineEncryptionKey is destroyed
		qtransform.WithConcurrency(2),
	)
}
