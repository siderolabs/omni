// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineTeardownControllerName is the name of the MachineTeardownController.
const MachineTeardownControllerName = "MachineTeardownController"

// MachineTeardownController processes additional teardown steps for a machine leaving a machine set.
type MachineTeardownController struct {
	generic.NamedController
}

// NewMachineTeardownController initializes MachineTeardownController.
func NewMachineTeardownController() *MachineTeardownController {
	return &MachineTeardownController{
		NamedController: generic.NamedController{
			ControllerName: MachineTeardownControllerName,
		},
	}
}

// Settings implements controller.QController interface.
func (ctrl *MachineTeardownController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineStatusType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.TalosConfigType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineStatusSnapshotType,
				Kind:      controller.InputQMapped,
			},
		},
		Concurrency: optional.Some[uint](4),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineTeardownController) MapInput(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) ([]resource.Pointer, error) {
	if ptr.Type() == omni.MachineStatusSnapshotType {
		return qtransform.MapperSameID[*omni.MachineStatusSnapshot, *omni.MachineStatus]()(
			ctx,
			logger,
			r,
			omni.NewMachineStatusSnapshot(resources.DefaultNamespace, ptr.ID()),
		)
	}

	if ptr.Type() == omni.TalosConfigType {
		return nil, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *MachineTeardownController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if machineStatus.Metadata().Phase() == resource.PhaseTearingDown {
		if err := ctrl.resetMachine(ctx, r, machineStatus, logger); err != nil {
			return err
		}

		return r.RemoveFinalizer(ctx, machineStatus.Metadata(), ctrl.Name())
	}

	if !machineStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		return r.AddFinalizer(ctx, machineStatus.Metadata(), ctrl.Name())
	}

	return nil
}

func (ctrl *MachineTeardownController) resetMachine(
	ctx context.Context,
	r controller.QRuntime,
	machineStatus *omni.MachineStatus,
	logger *zap.Logger,
) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	c, err := helpers.GetTalosClient(ctx, r, machineStatus.TypedSpec().Value.ManagementAddress, machineStatus)
	if err != nil {
		return err
	}

	defer func() {
		if e := c.Close(); e != nil {
			logger.Warn("failed to close reset-machine client", zap.Error(err))
		}
	}()

	disks, err := c.Disks(ctx)
	if err != nil {
		logger.Warn("machine wipe check failed", zap.Error(err))

		return nil
	}

	var installed bool

	for _, m := range disks.Messages {
		for _, d := range m.Disks {
			if d.SystemDisk {
				installed = true

				break
			}
		}
	}

	if !installed {
		logger.Info("skipping machine wipe as Talos is not installed")

		return nil
	}

	// try to wipe the machine without any attempts to retry it
	if err = c.Reset(ctx, false, false); err != nil {
		logger.Warn("machine wipe failed", zap.Error(err))

		return nil
	}

	logger.Info("wiped Talos on the machine")

	return nil
}
