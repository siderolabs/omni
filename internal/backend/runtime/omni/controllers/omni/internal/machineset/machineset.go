// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machineset contains machine set controller reconciliation code.
package machineset

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

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
	if rc.cluster == nil {
		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine set cluster does not exist")
	}

	_, locked := rc.cluster.Metadata().Annotations().Get(omni.ClusterLocked)
	_, importIsInProgress := rc.cluster.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)

	// if a cluster has become tearing down while it's locked that means that the import process was aborted so we should not skip reconciling
	// we should also not skip reconciling if the cluster import is in progress
	if locked && !importIsInProgress && rc.cluster.Metadata().Phase() == resource.PhaseRunning {
		logger.Warn("cluster is locked, skip reconcile", zap.String("cluster", rc.cluster.Metadata().ID()))

		return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("reconciling machines are not allowed: the cluster is locked")
	}

	var operations []Operation

	machineSet := rc.GetMachineSet()

	_, isControlPlane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	switch {
	case machineSet.Metadata().Phase() == resource.PhaseTearingDown:
		operations = ReconcileTearingDown(rc)
	case isControlPlane:
		var err error

		operations, err = ReconcileControlPlanes(ctx, rc, func(ctx context.Context) (*check.EtcdStatusResult, error) {
			ctx, cancel := context.WithTimeout(ctx, time.Second*30)

			defer cancel()

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
			if err := r.AddFinalizer(ctx, omni.NewMachine(clusterMachine.Metadata().ID()).Metadata(), ControllerName); err != nil {
				if state.IsNotFoundError(err) {
					continue
				}

				return err
			}
		}
	}

	return nil
}
