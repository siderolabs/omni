// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/uncached"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

// machineOperationSyncInterval limits the writes, as progress messages arrive per installer log line and per drained pod.
const machineOperationSyncInterval = 5 * time.Second

// MachineOperations represents the source of the operations Omni runs on the machines.
type MachineOperations interface {
	Operations() map[string]lifecycle.OperationState
	Changes() <-chan struct{}
}

// MachineOperationStatusController manages omni.MachineOperationStatus.
type MachineOperationStatusController struct {
	operations   MachineOperations
	syncInterval time.Duration
}

// NewMachineOperationStatusController creates a new MachineOperationStatusController.
func NewMachineOperationStatusController(operations MachineOperations) *MachineOperationStatusController {
	return &MachineOperationStatusController{operations: operations, syncInterval: machineOperationSyncInterval}
}

// Name implements controller.Controller interface.
func (ctrl *MachineOperationStatusController) Name() string {
	return "MachineOperationStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineOperationStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *MachineOperationStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineOperationStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MachineOperationStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ctrl.operations.Changes():
		}

		// the cache is fed only by the controllers which declare the type as an input
		tracker := trackResource(uncached.ReaderWriter(r), resources.EphemeralNamespace, omni.MachineOperationStatusType)

		for machineID, operation := range ctrl.operations.Operations() {
			res := omni.NewMachineOperationStatus(machineID)

			tracker.keep(res)

			if err := safe.WriterModify(ctx, r, res, func(res *omni.MachineOperationStatus) error {
				spec := res.TypedSpec().Value

				spec.Phase = operation.Phase
				spec.Message = operation.Message
				spec.LastError = operation.LastError
				spec.FailedPhase = operation.FailedPhase
				spec.PhaseStartedAt = nil

				if !operation.PhaseStartedAt.IsZero() {
					spec.PhaseStartedAt = timestamppb.New(operation.PhaseStartedAt)
				}

				return nil
			}); err != nil {
				return err
			}
		}

		if err := tracker.cleanup(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(ctrl.syncInterval):
		}
	}
}
