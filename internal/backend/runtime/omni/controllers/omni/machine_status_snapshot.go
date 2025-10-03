// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/cosi-project/runtime/pkg/task"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/snapshot"
)

// MachineStatusSnapshotControllerName is the name of the MachineStatusSnapshotController.
const MachineStatusSnapshotControllerName = "MachineStatusSnapshotController"

// MachineStatusSnapshotController manages omni.MachineStatuses based on information from Talos API.
type MachineStatusSnapshotController struct {
	runner       *task.Runner[snapshot.InfoChan, snapshot.CollectTaskSpec]
	notifyCh     chan *omni.MachineStatusSnapshot
	siderolinkCh <-chan *omni.MachineStatusSnapshot
	powerStageCh <-chan *omni.MachineStatusSnapshot
	generic.NamedController
}

// NewMachineStatusSnapshotController initializes MachineStatusSnapshotController.
func NewMachineStatusSnapshotController(siderolinkEventsCh, powerStageEventsCh <-chan *omni.MachineStatusSnapshot) *MachineStatusSnapshotController {
	return &MachineStatusSnapshotController{
		NamedController: generic.NamedController{
			ControllerName: MachineStatusSnapshotControllerName,
		},
		notifyCh:     make(chan *omni.MachineStatusSnapshot),
		siderolinkCh: siderolinkEventsCh,
		powerStageCh: powerStageEventsCh,
		runner:       task.NewEqualRunner[snapshot.CollectTaskSpec](),
	}
}

// Settings implements controller.QController interface.
func (ctrl *MachineStatusSnapshotController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.TalosConfigType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineType,
				Kind:      controller.InputQMapped,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputExclusive,
				Type: omni.MachineStatusSnapshotType,
			},
		},
		Concurrency: optional.Some[uint](4),
		RunHook: func(ctx context.Context, _ *zap.Logger, r controller.QRuntime) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case resource := <-ctrl.siderolinkCh:
					if err := ctrl.reconcileSnapshot(ctx, r, resource); err != nil {
						return err
					}
				case resource := <-ctrl.notifyCh:
					if err := ctrl.reconcileSnapshot(ctx, r, resource); err != nil {
						return err
					}
				case resource := <-ctrl.powerStageCh:
					if err := ctrl.reconcileSnapshot(ctx, r, resource); err != nil {
						return err
					}
				}
			}
		},
		ShutdownHook: func() {
			ctrl.runner.Stop()
		},
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineStatusSnapshotController) MapInput(ctx context.Context, _ *zap.Logger,
	r controller.QRuntime, ptr controller.ReducedResourceMetadata,
) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case omni.ClusterMachineType:
		fallthrough
	case omni.MachineType:
		return []resource.Pointer{
			omni.NewMachine(resources.DefaultNamespace, ptr.ID()).Metadata(),
		}, nil
	case omni.TalosConfigType:
		machines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, ptr.ID())))
		if err != nil {
			return nil, err
		}

		res := make([]resource.Pointer, 0, machines.Len())

		machines.ForEach(func(r *omni.ClusterMachine) {
			res = append(res, omni.NewMachine(resources.DefaultNamespace, r.Metadata().ID()).Metadata())
		})

		return res, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *MachineStatusSnapshotController) Reconcile(ctx context.Context,
	logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	machine, err := safe.ReaderGet[*omni.Machine](ctx, r, omni.NewMachine(ptr.Namespace(), ptr.ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if machine.Metadata().Phase() == resource.PhaseTearingDown {
		return ctrl.reconcileTearingDown(ctx, r, logger, machine)
	}

	return ctrl.reconcileRunning(ctx, r, logger, machine)
}

func (ctrl *MachineStatusSnapshotController) reconcileRunning(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machine *omni.Machine) error {
	if !machine.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, machine.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	var talosConfig *omni.TalosConfig

	if clusterMachine != nil {
		clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
		if ok {
			talosConfig, err = safe.ReaderGetByID[*omni.TalosConfig](ctx, r, clusterName)
			if err != nil && !state.IsNotFoundError(err) {
				return err
			}
		}
	}

	if !machine.TypedSpec().Value.Connected {
		ctrl.runner.StopTask(logger, machine.Metadata().ID())
	}

	if machine.TypedSpec().Value.Connected {
		ctrl.runner.StartTask(ctx, logger, machine.Metadata().ID(), snapshot.CollectTaskSpec{
			Endpoint:    machine.TypedSpec().Value.ManagementAddress,
			TalosConfig: talosConfig,
			MachineID:   machine.Metadata().ID(),
		}, ctrl.notifyCh)
	}

	return nil
}

func (ctrl *MachineStatusSnapshotController) reconcileTearingDown(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machine *omni.Machine) error {
	ctrl.runner.StopTask(logger, machine.Metadata().ID())

	md := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, machine.Metadata().ID()).Metadata()

	ready, err := helpers.TeardownAndDestroy(ctx, r, md)
	if err != nil {
		return err
	}

	if !ready {
		return nil
	}

	return r.RemoveFinalizer(ctx, machine.Metadata(), ctrl.Name())
}

func (ctrl *MachineStatusSnapshotController) reconcileSnapshot(ctx context.Context, r controller.QRuntime, snapshot *omni.MachineStatusSnapshot) error {
	machine, err := safe.ReaderGetByID[*omni.Machine](ctx, r, snapshot.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if machine.Metadata().Phase() == resource.PhaseTearingDown {
		return nil
	}

	if err = safe.WriterModify(ctx, r, omni.NewMachineStatusSnapshot(resources.DefaultNamespace, snapshot.Metadata().ID()), func(m *omni.MachineStatusSnapshot) error {
		if snapshot.TypedSpec().Value.MachineStatus != nil { // if this is a power stage snapshot, it will not contain machine status, so we preserve the existing value
			m.TypedSpec().Value.MachineStatus = snapshot.TypedSpec().Value.MachineStatus
		}

		m.TypedSpec().Value.PowerStage = snapshot.TypedSpec().Value.PowerStage // always set the power stage

		return nil
	}); err != nil && !state.IsPhaseConflictError(err) {
		return fmt.Errorf("error modifying resource: %w", err)
	}

	return nil
}
