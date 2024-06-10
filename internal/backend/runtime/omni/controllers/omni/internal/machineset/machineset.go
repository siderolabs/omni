// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machineset contains machine set controller reconciliation code.
package machineset

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
)

// ControllerName controller name constant.
const ControllerName = "MachineSetStatusController"

func toSlice[T resource.Resource](list safe.List[T]) []T {
	res := make([]T, 0, list.Len())

	list.ForEach(func(t T) {
		res = append(res, t)
	})

	return res
}

// ReconcileMachines creates, updates and tears down the machines using the ReconciliationContext.
func ReconcileMachines(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) (bool, error) {
	var operations []Operation

	machineSet := rc.GetMachineSet()

	for _, clusterMachine := range rc.GetClusterMachines() {
		if clusterMachine.Metadata().Phase() == resource.PhaseRunning {
			if err := safe.WriterModify(ctx, r, clusterMachine, func(clusteMachine *omni.ClusterMachine) error {
				if rc.GetLockedUpdates().Contains(clusteMachine.Metadata().ID()) {
					clusteMachine.Metadata().Labels().Set(omni.UpdateLocked, "")

					return nil
				}

				if _, ok := clusteMachine.Metadata().Labels().Get(omni.UpdateLocked); ok {
					clusteMachine.Metadata().Labels().Delete(omni.UpdateLocked)
				}

				return nil
			}); err != nil {
				return false, err
			}
		}
	}

	_, isControlPlane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	switch {
	case machineSet.Metadata().Phase() == resource.PhaseTearingDown:
		operations = ReconcileTearingDown(rc)
	case isControlPlane:
		var err error

		operations, err = ReconcileControlPlanes(ctx, rc, func(ctx context.Context) (*check.EtcdStatusResult, error) {
			return check.EtcdStatus(ctx, r, machineSet)
		})
		if err != nil {
			return false, err
		}
	default:
		operations = ReconcileWorkers(rc)
	}

	for _, op := range operations {
		if err := op.Apply(ctx, r, logger, rc); err != nil {
			return false, err
		}
	}

	return len(operations) > 0, nil
}

// UpdateFinalizers sets finalizers to all machine set nodes, adds finalizers on the Machines resources.
func UpdateFinalizers(ctx context.Context, r controller.ReaderWriter, rc *ReconciliationContext) error {
	// add finalizers to all running machine set nodes
	machineSetRunning := rc.GetMachineSet().Metadata().Phase() == resource.PhaseRunning

	for _, machineSetNode := range rc.GetMachineSetNodes() {
		if machineSetNode.Metadata().Phase() == resource.PhaseTearingDown || !machineSetRunning {
			if err := r.RemoveFinalizer(ctx, machineSetNode.Metadata(), ControllerName); err != nil {
				return err
			}

			continue
		}

		if machineSetNode.Metadata().Finalizers().Has(ControllerName) {
			continue
		}

		if err := r.AddFinalizer(ctx, machineSetNode.Metadata(), ControllerName); err != nil {
			return err
		}
	}

	// add finalizers to all machines which have running cluster machines.
	for _, clusterMachine := range rc.GetClusterMachines() {
		if clusterMachine.Metadata().Phase() == resource.PhaseRunning && !clusterMachine.Metadata().Finalizers().Has(ControllerName) {
			if err := r.AddFinalizer(ctx, omni.NewMachine(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(), ControllerName); err != nil {
				if state.IsNotFoundError(err) {
					continue
				}

				return err
			}
		}
	}

	return nil
}
