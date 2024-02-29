// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

//tsgen:installDiskMinSize
const installDiskMinSize = 5e9 // 5GB

// MachineConfigGenOptionsController creates a patch that configures machine install disk automatically.
type MachineConfigGenOptionsController = qtransform.QController[*omni.MachineStatus, *omni.MachineConfigGenOptions]

// NewMachineConfigGenOptionsController initializes MachineConfigGenOptionsController.
func NewMachineConfigGenOptionsController() *MachineConfigGenOptionsController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.MachineConfigGenOptions]{
			Name: "MachineConfigGenOptionsController",
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.MachineConfigGenOptions {
				return omni.NewMachineConfigGenOptions(resources.DefaultNamespace, machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineConfigGenOptions *omni.MachineConfigGenOptions) *omni.MachineStatus {
				return omni.NewMachineStatus(resources.DefaultNamespace, machineConfigGenOptions.Metadata().ID())
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, machineStatus *omni.MachineStatus, options *omni.MachineConfigGenOptions) error {
				GenInstallConfig(machineStatus, options)

				return nil
			},
		},
		qtransform.WithIgnoreTeardownUntil(), // keep the resource until everyone else is done with Machine
	)
}

// GenInstallConfig creates a config patch with an automatically picked install disk.
func GenInstallConfig(machineStatus *omni.MachineStatus, configPatch *omni.MachineConfigGenOptions) {
	if machineStatus.TypedSpec().Value.Hardware == nil {
		return
	}

	installDisk := omni.GetMachineStatusSystemDisk(machineStatus)

	diskSize := ^uint64(0)

	if installDisk == "" {
		for _, disk := range machineStatus.TypedSpec().Value.Hardware.Blockdevices {
			if disk.Size >= installDiskMinSize && disk.Size < diskSize {
				installDisk = disk.LinuxName

				diskSize = disk.Size
			}
		}
	}

	configPatch.TypedSpec().Value.InstallDisk = installDisk
}
