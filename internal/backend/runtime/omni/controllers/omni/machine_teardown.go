// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
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
		},
		Concurrency: optional.Some[uint](4),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineTeardownController) MapInput(ctx context.Context, _ *zap.Logger, r controller.QRuntime, ptr resource.Pointer) ([]resource.Pointer, error) {
	if ptr.Type() == omni.TalosConfigType {
		statuses, err := safe.ReaderListAll[*omni.MachineStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, ptr.ID())))
		if err != nil {
			return nil, err
		}

		return safe.ToSlice(statuses, func(s *omni.MachineStatus) resource.Pointer {
			return s.Metadata()
		}), nil
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

func (ctrl *MachineTeardownController) resetMachine(ctx context.Context, r controller.QRuntime,
	machineStatus *omni.MachineStatus, logger *zap.Logger,
) error {
	client, err := ctrl.getClient(ctx, r, machineStatus)
	if err != nil {
		return err
	}

	disks, err := client.Disks(ctx)
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
	err = client.Reset(ctx, false, false)
	if err != nil {
		logger.Warn("machine wipe failed", zap.Error(err))

		return nil
	}

	logger.Info("wiped Talos on the machine")

	return nil
}

func (ctrl *MachineTeardownController) getClient(
	ctx context.Context,
	r controller.QRuntime,
	machineStatus *omni.MachineStatus,
) (*client.Client, error) {
	address := machineStatus.TypedSpec().Value.ManagementAddress
	opts := talos.GetSocketOptions(address)

	clusterName, ok := machineStatus.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return client.New(ctx,
			append(
				opts,
				client.WithTLSConfig(insecureTLSConfig),
				client.WithEndpoints(address),
			)...)
	}

	talosConfig, err := safe.ReaderGet[*omni.TalosConfig](ctx, r, omni.NewTalosConfig(resources.DefaultNamespace, clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster '%s' talosconfig not found: %w", clusterName, err)
		}

		return nil, fmt.Errorf("cluster '%s' failed to get talosconfig: %w", clusterName, err)
	}

	var endpoints []string

	if opts == nil {
		endpoints = []string{address}
	}

	config := omni.NewTalosClientConfig(talosConfig, endpoints...)
	opts = append(opts, client.WithConfig(config))

	result, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client to machine '%s': %w", machineStatus.Metadata().ID(), err)
	}

	return result, nil
}
