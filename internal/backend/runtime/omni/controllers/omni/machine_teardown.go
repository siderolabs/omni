// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// MachineTeardownControllerName is the name of the MachineTeardownController.
const MachineTeardownControllerName = "MachineTeardownController"

// MachineTeardownController processes additional teardown steps for a machine leaving a machine set.
type MachineTeardownController struct {
	talosClientFactory *talos.ClientFactory
	generic.NamedController
}

// NewMachineTeardownController initializes MachineTeardownController.
func NewMachineTeardownController(talosClientFactory *talos.ClientFactory) *MachineTeardownController {
	return &MachineTeardownController{
		NamedController: generic.NamedController{
			ControllerName: MachineTeardownControllerName,
		},
		talosClientFactory: talosClientFactory,
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
func (ctrl *MachineTeardownController) MapInput(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	if ptr.Type() == omni.MachineStatusSnapshotType {
		return qtransform.MapperSameID[*omni.MachineStatus]()(
			ctx,
			logger,
			r,
			ptr,
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
		if err := ctrl.resetMachine(ctx, machineStatus, logger); err != nil {
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
	machineStatus *omni.MachineStatus,
	logger *zap.Logger,
) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	c, err := ctrl.talosClientFactory.GetForMachine(ctx, machineStatus.Metadata().ID())
	if err != nil {
		if talos.IsClientNotReadyError(err) {
			logger.Info("skipping machine wipe as the machine is not reachable", zap.Error(err))

			return nil
		}

		return err
	}

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
