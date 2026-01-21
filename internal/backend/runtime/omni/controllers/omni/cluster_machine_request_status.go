// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// ClusterMachineRequestStatusController reflects the status of the machine request which will be added to a machine set.
type ClusterMachineRequestStatusController = qtransform.QController[*infra.MachineRequest, *omni.ClusterMachineRequestStatus]

const clusterMachineRequestStatusControllerName = "ClusterMachineRequestStatusController"

// NewClusterMachineRequestStatusController initializes ClusterMachineRequestStatusController.
//
//nolint:gocognit,gocyclo,cyclop
func NewClusterMachineRequestStatusController() *ClusterMachineRequestStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*infra.MachineRequest, *omni.ClusterMachineRequestStatus]{
			Name: clusterMachineRequestStatusControllerName,
			MapMetadataFunc: func(machineRequest *infra.MachineRequest) *omni.ClusterMachineRequestStatus {
				return omni.NewClusterMachineRequestStatus(machineRequest.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterMachineStatus *omni.ClusterMachineRequestStatus) *infra.MachineRequest {
				return infra.NewMachineRequest(clusterMachineStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger,
				machineRequest *infra.MachineRequest, clusterMachineRequestStatus *omni.ClusterMachineRequestStatus,
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

				clusterMachineRequestStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_PENDING
				clusterMachineRequestStatus.TypedSpec().Value.Status = "Waiting for the infra provider to start provision"

				switch {
				case machineRequest.Metadata().Phase() == resource.PhaseTearingDown:
					clusterMachineRequestStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_DEPROVISIONING

					clusterMachineRequestStatus.TypedSpec().Value.Status = "Waiting for the infra provider to finish teardown"
				case machineRequestStatus != nil:
					clusterMachineRequestStatus.TypedSpec().Value.MachineUuid = machineRequestStatus.TypedSpec().Value.Id

					clusterMachineRequestStatus.TypedSpec().Value.Status = machineRequestStatus.TypedSpec().Value.Status

					switch machineRequestStatus.TypedSpec().Value.Stage {
					case specs.MachineRequestStatusSpec_UNKNOWN:
						clusterMachineRequestStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_PENDING
					case specs.MachineRequestStatusSpec_PROVISIONING:
						clusterMachineRequestStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_PROVISIONING

					case specs.MachineRequestStatusSpec_PROVISIONED:
						clusterMachineRequestStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_PROVISIONED

						clusterMachineRequestStatus.TypedSpec().Value.Status = "Waiting for the machine to join Omni"
					case specs.MachineRequestStatusSpec_FAILED:
						clusterMachineRequestStatus.TypedSpec().Value.Stage = specs.ClusterMachineRequestStatusSpec_FAILED

						clusterMachineRequestStatus.TypedSpec().Value.Status = fmt.Sprintf("Provision Failed: %s", machineRequestStatus.TypedSpec().Value.Error)
					}
				}

				if machineRequestStatus != nil {
					clusterMachineStatus, err := safe.ReaderGetByID[*omni.ClusterMachineStatus](ctx, r, machineRequestStatus.TypedSpec().Value.Id)
					if err != nil && !state.IsNotFoundError(err) {
						return err
					}

					if clusterMachineStatus != nil {
						clusterMachineRequestStatus.Metadata().Labels().Set(omni.LabelMachineRequestInUse, "")
					} else {
						clusterMachineRequestStatus.Metadata().Labels().Delete(omni.LabelMachineRequestInUse)
					}
				}

				clusterMachineRequestStatus.Metadata().Labels().Set(omni.LabelMachineSet, machineSetName)

				clusterMachineRequestStatus.TypedSpec().Value.ProviderId, _ = machineRequest.Metadata().Labels().Get(omni.LabelInfraProviderID)

				helpers.CopyLabels(machineSet, clusterMachineRequestStatus, omni.LabelWorkerRole, omni.LabelControlPlaneRole, omni.LabelCluster)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.MachineSet](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, res controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				list, err := safe.ReaderListAll[*infra.MachineRequest](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelMachineRequestSet, res.ID()),
				))
				if err != nil {
					return nil, err
				}

				return slices.Collect(list.Pointers()), nil
			},
		),
		qtransform.WithExtraMappedInput[*infra.MachineRequestStatus](
			qtransform.MapperSameID[*infra.MachineRequest](),
		),
		qtransform.WithExtraMappedInput[*infra.ProviderStatus](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, res controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				list, err := safe.ReaderListAll[*infra.MachineRequest](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelInfraProviderID, res.ID()),
				))
				if err != nil {
					return nil, err
				}

				return slices.Collect(list.Pointers()), nil
			},
		),
		qtransform.WithExtraMappedInput[*omni.Machine](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, res controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				machineRequestID, ok := res.Labels().Get(omni.LabelMachineRequest)
				if !ok {
					return nil, nil
				}

				return []resource.Pointer{
					infra.NewMachineRequest(machineRequestID).Metadata(),
				}, nil
			},
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, res controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				machine, err := safe.ReaderGetByID[*omni.Machine](ctx, r, res.ID())
				if err != nil {
					if state.IsNotFoundError(err) {
						return nil, nil
					}

					return nil, err
				}

				machineRequestID, ok := machine.Metadata().Labels().Get(omni.LabelMachineRequest)
				if !ok {
					return nil, nil
				}

				return []resource.Pointer{
					infra.NewMachineRequest(machineRequestID).Metadata(),
				}, nil
			},
		),
		qtransform.WithIgnoreTeardownUntil(),
		qtransform.WithConcurrency(2),
	)
}
