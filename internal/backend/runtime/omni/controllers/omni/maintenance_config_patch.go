// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"bytes"
	"context"
	_ "embed"
	"strings"
	"text/template"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/config"
)

//go:embed data/maintenance-config-patch.yaml
var maintenanceConfigPatchData string

// MaintenanceConfigPatchPrefix is the prefix for the system config patch that contains the maintenance config.
const MaintenanceConfigPatchPrefix = "950-maintenance-config-"

// MaintenanceConfigPatchController creates the system config patch that contains the maintenance config.
type MaintenanceConfigPatchController = qtransform.QController[*omni.MachineStatus, *omni.ConfigPatch]

// NewMaintenanceConfigPatchController initializes MaintenanceConfigPatchController.
func NewMaintenanceConfigPatchController(eventSinkPort int) *MaintenanceConfigPatchController {
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
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, machineStatus *omni.MachineStatus, configPatch *omni.ConfigPatch) error {
				version := strings.TrimLeft(machineStatus.TypedSpec().Value.TalosVersion, "v")
				if version == "" {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("the machine Talos version wasn't read")
				}

				if !quirks.New(version).SupportsMultidoc() {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("the machine doesn't support partial configs")
				}

				connectionParams, err := safe.ReaderGetByID[*siderolink.ConnectionParams](ctx, r, siderolink.ConfigID)
				if err != nil {
					return err
				}

				return UpdateMaintenanceConfigPatch(configPatch, machineStatus, connectionParams, eventSinkPort)
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*siderolink.ConnectionParams](),
		),
		qtransform.WithConcurrency(2),
		qtransform.WithOutputKind(controller.OutputShared),
	)
}

// UpdateMaintenanceConfigPatch generates the siderolink connection config patch from the machine status and connection params.
func UpdateMaintenanceConfigPatch(configPatch *omni.ConfigPatch, machineStatus *omni.MachineStatus, connectionParams *siderolink.ConnectionParams, eventSinkPort int) error {
	configPatch.Metadata().Labels().Set(omni.LabelSystemPatch, "")
	configPatch.Metadata().Labels().Set(omni.LabelMachine, machineStatus.Metadata().ID())

	url, err := siderolink.APIURL(connectionParams, config.Config.SiderolinkUseGRPCTunnel)
	if err != nil {
		return err
	}

	template, err := template.New("patch").Parse(maintenanceConfigPatchData)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer

	if err = template.Execute(&buffer, struct {
		APIURL        string
		EventSinkPort int
	}{
		APIURL:        url,
		EventSinkPort: eventSinkPort,
	}); err != nil {
		return err
	}

	configPatch.TypedSpec().Value.Data = buffer.String()

	return nil
}
