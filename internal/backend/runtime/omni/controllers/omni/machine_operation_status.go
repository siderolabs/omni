// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineOperations represents the operations Omni runs on the machines.
type MachineOperations interface {
	Forget(machineID string, machineRemoved bool)
}

// MachineOperationStatusController manages omni.MachineOperationStatus.
type MachineOperationStatusController = qtransform.QController[*omni.Machine, *omni.MachineOperationStatus]

// NewMachineOperationStatusController creates a new MachineOperationStatusController.
func NewMachineOperationStatusController(operations MachineOperations) *MachineOperationStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Machine, *omni.MachineOperationStatus]{
			Name: "MachineOperationStatusController",
			MapMetadataFunc: func(machine *omni.Machine) *omni.MachineOperationStatus {
				return omni.NewMachineOperationStatus(machine.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MachineOperationStatus) *omni.Machine {
				return omni.NewMachine(status.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, machine *omni.Machine, status *omni.MachineOperationStatus) error {
				machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, r, machine.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if machineSetNode != nil {
					helpers.SyncLabels(machineSetNode, status, omni.LabelCluster, omni.LabelMachineSet)
				} else {
					status.Metadata().Labels().Delete(omni.LabelCluster)
					status.Metadata().Labels().Delete(omni.LabelMachineSet)
				}

				operation, err := safe.ReaderGetByID[*omni.MachineOperation](ctx, r, machine.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				var operationSpec *specs.MachineOperationSpec

				if operation != nil {
					operationSpec = operation.TypedSpec().Value
				}

				back, err := rebootedSince(ctx, r, machine.Metadata().ID(), operationSpec.GetPreRebootBootId())
				if err != nil {
					return err
				}

				if back && operationSpec.GetStage() == specs.MachineOperationSpec_REBOOTING {
					operations.Forget(machine.Metadata().ID(), false)

					operationSpec = nil
				}

				statusSpec := status.TypedSpec().Value

				statusSpec.Stage = operationSpec.GetStage()
				statusSpec.Message = operationSpec.GetMessage()
				statusSpec.LastError = operationSpec.GetLastError()
				statusSpec.FailedStage = operationSpec.GetFailedStage()
				statusSpec.StageStartedAt = nil

				if startedAt := operationSpec.GetStageStartedAt(); startedAt != nil {
					statusSpec.StageStartedAt = timestamppb.New(startedAt.AsTime())
				}

				return nil
			},
			FinalizerRemovalFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, machine *omni.Machine) error {
				operations.Forget(machine.Metadata().ID(), true)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.MachineOperation](qtransform.MapperSameID[*omni.Machine]()),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](qtransform.MapperSameID[*omni.Machine]()),
		qtransform.WithExtraMappedInput[*omni.MachineStatusSnapshot](qtransform.MapperSameID[*omni.Machine]()),
	)
}

// rebootedSince reports whether the machine runs with a boot ID other than the given one.
func rebootedSince(ctx context.Context, r controller.Reader, machineID, bootID string) (bool, error) {
	if bootID == "" {
		return false, nil
	}

	snapshot, err := safe.ReaderGetByID[*omni.MachineStatusSnapshot](ctx, r, machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, err
	}

	current := snapshot.TypedSpec().Value.BootId

	return current != "" && current != bootID, nil
}
