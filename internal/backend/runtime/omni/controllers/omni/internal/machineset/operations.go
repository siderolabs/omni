// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/set"
)

// Operation is a single operation which alters the machine set.
type Operation interface {
	Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error
}

// Create the cluster machine.
type Create struct {
	ID string
}

// Apply implements Operation interface.
func (c *Create) Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error {
	clusterMachine := omni.NewClusterMachine(c.ID)

	machineSet := rc.GetMachineSet()

	helpers.CopyLabels(machineSet, clusterMachine, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole)

	clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())

	var err error

	clusterMachine.TypedSpec().Value.KubernetesVersion, err = rc.GetKubernetesVersion()
	if err != nil {
		return err
	}

	logger.Info("create cluster machine", zap.String("machine", c.ID))

	return r.Create(ctx, clusterMachine)
}

// Teardown the cluster machine.
type Teardown struct {
	Quota *ChangeQuota
	ID    string
}

// Apply implements Operation interface.
func (d *Teardown) Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error {
	if !d.Quota.Use(opDelete) {
		logger.Info("teardown is waiting for quota", zap.String("machine", d.ID), zap.Strings("pending", set.Values(rc.GetTearingDownMachines())))

		return nil
	}

	logger.Info("teardown cluster machine", zap.String("machine", d.ID))

	if _, err := r.Teardown(ctx, omni.NewClusterMachine(d.ID).Metadata()); err != nil {
		return fmt.Errorf(
			"error tearing down machine %q in cluster %q: %w",
			d.ID,
			rc.GetClusterName(),
			err,
		)
	}

	return nil
}

// Destroy cleans up the cluster machines without finalizers.
type Destroy struct {
	ID string
}

// Apply implements Operation interface.
func (d *Destroy) Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error {
	clusterMachine, ok := rc.GetClusterMachine(d.ID)
	if !ok {
		return fmt.Errorf("machine with id %q doesn't exist in the state", d.ID)
	}

	if clusterMachine.Metadata().Phase() == resource.PhaseRunning {
		return nil
	}

	if !clusterMachine.Metadata().Finalizers().Empty() {
		return nil
	}

	if err := r.Destroy(ctx, clusterMachine.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// release the Machine finalizer
	if err := r.RemoveFinalizer(
		ctx,
		omni.NewMachine(clusterMachine.Metadata().ID()).Metadata(),
		ControllerName,
	); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error removing finalizer from machine %q: %w", clusterMachine.Metadata().ID(), err)
	}

	logger.Info("deleted the machine",
		zap.String("machine", clusterMachine.Metadata().ID()),
	)

	return nil
}
