// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package kernelargs contains controllers related to kernel args status.
package kernelargs

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// StatusController reflects the status of a machine that is a member of a cluster.
type StatusController = qtransform.QController[*omni.MachineStatus, *omni.KernelArgsStatus]

// NewStatusController initializes StatusController.
func NewStatusController() *StatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.KernelArgsStatus]{
			Name: "KernelArgsStatusController",
			MapMetadataFunc: func(ms *omni.MachineStatus) *omni.KernelArgsStatus {
				return omni.NewKernelArgsStatus(ms.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.KernelArgsStatus) *omni.MachineStatus {
				return omni.NewMachineStatus(status.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, ms *omni.MachineStatus, status *omni.KernelArgsStatus) error {
				helpers.SyncLabels(ms, status, omni.LabelCluster, omni.LabelMachineSet)

				status.TypedSpec().Value.UnmetConditions = nil
				status.TypedSpec().Value.CurrentCmdline = ms.TypedSpec().Value.KernelCmdline

				if ms.TypedSpec().Value.Schematic == nil {
					status.TypedSpec().Value.UnmetConditions = append(status.TypedSpec().Value.UnmetConditions, "Schematic information is not yet known")
				} else {
					status.TypedSpec().Value.CurrentArgs = kernelargs.FilterExtras(ms.TypedSpec().Value.Schematic.KernelArgs)
				}

				if omni.GetMachineStatusSystemDisk(ms) == "" {
					status.TypedSpec().Value.UnmetConditions = append(status.TypedSpec().Value.UnmetConditions, "Talos is not installed, kernel args cannot be updated yet")
				}

				securityState := ms.TypedSpec().Value.SecurityState
				talosVersion := ms.TypedSpec().Value.TalosVersion

				if securityState == nil && talosVersion == "" {
					status.TypedSpec().Value.UnmetConditions = append(status.TypedSpec().Value.UnmetConditions,
						"Cannot determine if kernel args update is supported: SecurityState and TalosVersion are not yet set")

					return nil
				}

				updateSupported, err := kernelargs.UpdateSupported(ms, func() (*omni.ClusterMachineConfig, error) {
					return safe.ReaderGetByID[*omni.ClusterMachineConfig](ctx, r, ms.Metadata().ID())
				})
				if err != nil {
					return err
				}

				if !updateSupported {
					status.TypedSpec().Value.UnmetConditions = append(status.TypedSpec().Value.UnmetConditions,
						"Unsupported: machine is not booted with UKI and Talos version is < 1.12 or GrubUseUKICmdline is false")
				}

				kernelArgs, err := safe.ReaderGetByID[*omni.KernelArgs](ctx, r, ms.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				args, argsOk, err := kernelargs.Calculate(ms, kernelArgs)
				if err != nil {
					return err
				}

				if !argsOk {
					status.TypedSpec().Value.UnmetConditions = append(status.TypedSpec().Value.UnmetConditions, "Extra kernel args are not yet initialized")
				}

				status.TypedSpec().Value.Args = kernelargs.FilterExtras(args)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.KernelArgs](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfig](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
	)
}
