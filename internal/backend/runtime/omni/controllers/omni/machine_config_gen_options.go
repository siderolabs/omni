// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"cmp"
	"context"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineConfigGenOptionsControllerName is the name of the MachineConfigGenOptionsController.
const MachineConfigGenOptionsControllerName = "MachineConfigGenOptionsController"

//tsgen:installDiskMinSize
const installDiskMinSize = 5e9 // 5GB

// MachineConfigGenOptionsController creates a patch that configures machine install disk automatically.
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

// GenInstallConfig creates a config patch with an automatically picked install disk.
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

	if machineStatus.TypedSpec().Value.Hardware == nil {
		return
	}

	installDisk := omni.GetMachineStatusSystemDisk(machineStatus)

	if installDisk == "" {
		const transportUSB = "usb"

		candidates := machineStatus.TypedSpec().Value.Hardware.Blockdevices

		candidates = xslices.Filter(candidates, func(disk *specs.MachineStatusSpec_HardwareStatus_BlockDevice) bool {
			return !disk.Readonly && disk.Type != storage.Disk_CD.String() && disk.Size > installDiskMinSize && disk.BusPath != "/virtual"
		})

		sortFunc := func(a, b *specs.MachineStatusSpec_HardwareStatus_BlockDevice) int {
			if a.Transport == transportUSB && b.Transport != transportUSB {
				return 1
			} else if b.Transport == transportUSB && a.Transport != transportUSB {
				return -1
			}

			return cmp.Compare(a.Size, b.Size)
		}

		slices.SortFunc(candidates, sortFunc)

		if len(candidates) > 0 {
			installDisk = candidates[0].LinuxName
		}
	}

	genOptions.TypedSpec().Value.InstallDisk = installDisk
}
