// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MaintenanceConfigPatchPrefix is the prefix for the system config patch that contains the maintenance config.
const MaintenanceConfigPatchPrefix = "950-maintenance-config-"

// MaintenanceConfigPatchController creates the system config patch that contains the maintenance config.
type MaintenanceConfigPatchController = qtransform.QController[*omni.MachineStatus, *omni.ConfigPatch]

// NewMaintenanceConfigPatchController initializes MaintenanceConfigPatchController.
func NewMaintenanceConfigPatchController() *MaintenanceConfigPatchController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.ConfigPatch]{
			Name: "MaintenanceConfigPatchController",
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.ConfigPatch {
				return omni.NewConfigPatch(resources.DefaultNamespace, MaintenanceConfigPatchPrefix+machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(configPatch *omni.ConfigPatch) *omni.MachineStatus {
				id := strings.TrimPrefix(configPatch.Metadata().ID(), MaintenanceConfigPatchPrefix)

				return omni.NewMachineStatus(resources.DefaultNamespace, id)
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, machineStatus *omni.MachineStatus, configPatch *omni.ConfigPatch) error {
				maintenanceConfig := machineStatus.TypedSpec().Value.GetMaintenanceConfig()
				if maintenanceConfig == nil {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("maintenance config is not ready")
				}

				configPatch.Metadata().Labels().Set(omni.LabelSystemPatch, "")
				configPatch.Metadata().Labels().Set(omni.LabelMachine, machineStatus.Metadata().ID())

				configPatch.TypedSpec().Value.Data = maintenanceConfig.GetConfig()

				return nil
			},
		},
		qtransform.WithConcurrency(2),
		qtransform.WithOutputKind(controller.OutputShared),
	)
}
