// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package installdisk contains the controller resolving the install disk of each machine.
package installdisk

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// StatusController resolves the install disk of each machine into MachineInstallDiskStatus:
// the automatic default from the machine's disks, overridden by the optional user selection
// in MachineInstallDiskConfig. ClusterMachineConfigController and the UI dropdowns read the
// resolution result, never the inputs.
type StatusController = qtransform.QController[*omni.MachineStatus, *omni.MachineInstallDiskStatus]

// NewStatusController initializes StatusController.
func NewStatusController() *StatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.MachineInstallDiskStatus]{
			Name: "MachineInstallDiskStatusController",
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.MachineInstallDiskStatus {
				return omni.NewMachineInstallDiskStatus(machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MachineInstallDiskStatus) *omni.MachineStatus {
				return omni.NewMachineStatus(status.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, machineStatus *omni.MachineStatus, status *omni.MachineInstallDiskStatus) error {
				config, err := safe.ReaderGetByID[*omni.MachineInstallDiskConfig](ctx, r, machineStatus.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				resolution := Resolve(machineStatus, config)

				status.TypedSpec().Value.Disk = resolution.Disk
				status.TypedSpec().Value.Candidates = resolution.Candidates
				status.TypedSpec().Value.Message = resolution.Message
				status.TypedSpec().Value.SelectionHash = resolution.SelectionHash

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.MachineInstallDiskConfig](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
	)
}
