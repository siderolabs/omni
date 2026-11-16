// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/machineoperation"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineOperationStatusController publishes the operations Omni runs on the machines as omni.MachineOperationStatus resources.
//
// A status exists only while its operation runs. Only the run hook writes: on every wake-up, it writes a status for each
// reported operation and destroys the statuses without one.
type MachineOperationStatusController struct {
	operations *machineoperation.Reporter
	wake       chan struct{}
	generic.NamedController
}

// NewMachineOperationStatusController creates a new MachineOperationStatusController.
func NewMachineOperationStatusController(operations *machineoperation.Reporter) *MachineOperationStatusController {
	return &MachineOperationStatusController{
		ControllerName: "MachineOperationStatusController",
		operations:     operations,
		wake:           make(chan struct{}, 1),
	}
}

// Settings implements controller.QController interface.
func (ctrl *MachineOperationStatusController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineStatusSnapshotType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.EphemeralNamespace,
				Type:      omni.MachineOperationStatusType,
				Kind:      controller.InputQMappedDestroyReady,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputExclusive,
				Type: omni.MachineOperationStatusType,
			},
		},
		RunHook: func(ctx context.Context, _ *zap.Logger, r controller.QRuntime) error {
			for {
				if err := ctrl.publish(ctx, r); err != nil {
					return err
				}

				select {
				case <-ctx.Done():
					return nil
				case <-ctrl.operations.Changes():
				case <-ctrl.wake:
				}
			}
		},
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineOperationStatusController) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	if ptr.Type() == omni.ClusterMachineType { // the labels of the status follow the cluster machine
		ctrl.wakeUp()
	}

	return nil, nil
}

// Reconcile implements controller.QController interface.
func (ctrl *MachineOperationStatusController) Reconcile(context.Context, *zap.Logger, controller.QRuntime, resource.Pointer) error {
	ctrl.wakeUp() // the machine might have rebooted, or it might be removed

	return nil
}

func (ctrl *MachineOperationStatusController) wakeUp() {
	select {
	case ctrl.wake <- struct{}{}:
	default:
	}
}

// publish writes a status for each reported operation and destroys the statuses without one.
func (ctrl *MachineOperationStatusController) publish(ctx context.Context, r controller.QRuntime) error {
	published := map[string]struct{}{}

	for machineID, operation := range ctrl.operations.Snapshot() {
		ok, err := ctrl.publishMachine(ctx, r, machineID, operation)
		if err != nil {
			return fmt.Errorf("failed to publish the operation of machine %q: %w", machineID, err)
		}

		if ok {
			published[machineID] = struct{}{}
		}
	}

	statuses, err := safe.ReaderListAll[*omni.MachineOperationStatus](ctx, r)
	if err != nil {
		return err
	}

	for status := range statuses.All() {
		if _, ok := published[status.Metadata().ID()]; ok {
			continue
		}

		if err = r.Destroy(ctx, status.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return err
		}
	}

	return nil
}

// publishMachine writes the status of the machine. It returns false if the machine should have no status.
func (ctrl *MachineOperationStatusController) publishMachine(ctx context.Context, r controller.QRuntime, machineID string, operation machineoperation.State) (bool, error) {
	snapshot, err := safe.ReaderGetByID[*omni.MachineStatusSnapshot](ctx, r, machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// the machine is removed, or its snapshot is not in the cache yet
			return false, ctrl.endRebootOfRemovedMachine(ctx, r, machineID, operation)
		}

		return false, err
	}

	if operation.BootID != "" && operation.BootID != snapshot.TypedSpec().Value.BootId {
		// the machine is back
		ctrl.operations.Booted(machineID, snapshot.TypedSpec().Value.BootId)

		return false, nil
	}

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machineID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, err
	}

	return true, safe.WriterModify(ctx, r, omni.NewMachineOperationStatus(machineID), func(status *omni.MachineOperationStatus) error {
		// a user with access to a single cluster reads the status through these labels
		if clusterMachine != nil {
			helpers.SyncLabels(clusterMachine, status, omni.LabelCluster, omni.LabelMachineSet)
		} else {
			status.Metadata().Labels().Delete(omni.LabelCluster)
			status.Metadata().Labels().Delete(omni.LabelMachineSet)
		}

		status.TypedSpec().Value.Stage = operation.Stage
		status.TypedSpec().Value.Message = operation.Message
		status.TypedSpec().Value.StageStartedAt = timestamppb.New(operation.StartedAt)

		return nil
	})
}

// endRebootOfRemovedMachine ends the operation if it waits for the reboot of a removed machine, as nothing else would end it.
func (ctrl *MachineOperationStatusController) endRebootOfRemovedMachine(ctx context.Context, r controller.QRuntime, machineID string, operation machineoperation.State) error {
	if operation.BootID == "" {
		return nil
	}

	// uncached, as the snapshot might only be missing from the cache
	_, err := r.GetUncached(ctx, omni.NewMachineStatusSnapshot(machineID).Metadata())
	if state.IsNotFoundError(err) {
		ctrl.operations.Booted(machineID, "")

		return nil
	}

	return err
}
