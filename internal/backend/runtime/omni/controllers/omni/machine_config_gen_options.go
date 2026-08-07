// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineConfigGenOptionsControllerName is the name of the MachineConfigGenOptionsController.
const MachineConfigGenOptionsControllerName = "MachineConfigGenOptionsController"

// MachineConfigGenOptionsController maintains the machine config generation inputs, currently the install image.
type MachineConfigGenOptionsController = qtransform.QController[*omni.MachineStatus, *omni.MachineConfigGenOptions]

// NewMachineConfigGenOptionsController initializes MachineConfigGenOptionsController.
func NewMachineConfigGenOptionsController(imageFactoryClients ImageFactoryClientProvider) *MachineConfigGenOptionsController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.MachineConfigGenOptions]{
			Name: MachineConfigGenOptionsControllerName,
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.MachineConfigGenOptions {
				return omni.NewMachineConfigGenOptions(machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineConfigGenOptions *omni.MachineConfigGenOptions) *omni.MachineStatus {
				return omni.NewMachineStatus(machineConfigGenOptions.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, machineStatus *omni.MachineStatus, options *omni.MachineConfigGenOptions) error {
				clusterMachineTalosVersion, err := safe.ReaderGetByID[*omni.ClusterMachineTalosVersion](ctx, r, machineStatus.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				var (
					talosVersion string
					schematicID  string
				)

				if clusterMachineTalosVersion != nil {
					talosVersion = clusterMachineTalosVersion.TypedSpec().Value.TalosVersion
					schematicID = clusterMachineTalosVersion.TypedSpec().Value.SchematicId
				}

				imageFactoryClient, err := imageFactoryClients.ForTalosVersion(ctx, talosVersion)
				if err != nil {
					return err
				}

				imageFactoryHost := imageFactoryClient.Host()

				// Migration code: do not change image factory URL if it was already set in the options and the Talos version and schematic ID match the cluster machine Talos version.
				// Image factory URL will only be upgraded when the Talos version or schematic ID changes, or when the image factory URL is empty.
				if options.TypedSpec().Value.InstallImage != nil && clusterMachineTalosVersion != nil &&
					(options.TypedSpec().Value.InstallImage.TalosVersion == talosVersion &&
						options.TypedSpec().Value.InstallImage.SchematicId == schematicID) {
					imageFactoryHost = options.TypedSpec().Value.InstallImage.ImageFactoryHost
				}

				GenInstallConfig(machineStatus, clusterMachineTalosVersion, options, imageFactoryHost)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachineTalosVersion](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithIgnoreTeardownUntil(), // keep the resource until everyone else is done with Machine
	)
}

// GenInstallConfig fills the install image of the machine config generation options.
func GenInstallConfig(machineStatus *omni.MachineStatus, clusterMachineTalosVersion *omni.ClusterMachineTalosVersion, genOptions *omni.MachineConfigGenOptions, imageFactoryHost string) {
	if clusterMachineTalosVersion != nil {
		genOptions.TypedSpec().Value.InstallImage = omni.NewInstallImage(
			machineStatus,
			clusterMachineTalosVersion.TypedSpec().Value.TalosVersion,
			clusterMachineTalosVersion.TypedSpec().Value.SchematicId,
			imageFactoryHost,
			machineStatus.TypedSpec().Value.SchematicReady(),
		)
	}
}
