// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineConfigGenOptionsControllerName is the name of the MachineConfigGenOptionsController.
const MachineConfigGenOptionsControllerName = "MachineConfigGenOptionsController"

//tsgen:installDiskMinSize
const installDiskMinSize = 5e9 // 5GB

// MachineConfigGenOptionsController creates a patch that configures machine install disk automatically.
type MachineConfigGenOptionsController = qtransform.QController[*omni.MachineStatus, *omni.MachineConfigGenOptions]

// NewMachineConfigGenOptionsController initializes MachineConfigGenOptionsController.
func NewMachineConfigGenOptionsController() *MachineConfigGenOptionsController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.MachineConfigGenOptions]{
			Name: MachineConfigGenOptionsControllerName,
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.MachineConfigGenOptions {
				return omni.NewMachineConfigGenOptions(resources.DefaultNamespace, machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineConfigGenOptions *omni.MachineConfigGenOptions) *omni.MachineStatus {
				return omni.NewMachineStatus(resources.DefaultNamespace, machineConfigGenOptions.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, machineStatus *omni.MachineStatus, options *omni.MachineConfigGenOptions) error {
				clusterMachineTalosVersion, err := safe.ReaderGetByID[*omni.ClusterMachineTalosVersion](ctx, r, machineStatus.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				extraKernelArgsRes, err := safe.ReaderGetByID[*omni.MachineExtraKernelArgs](ctx, r, machineStatus.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				configStatus, err := safe.ReaderGetByID[*omni.ClusterMachineConfigStatus](ctx, r, machineStatus.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				securityState := machineStatus.TypedSpec().Value.SecurityState
				if securityState == nil {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("security state is not known yet")
				}

				var (
					extraKernelArgs        []string
					desiredExtraKernelArgs []string
				)

				if extraKernelArgsRes != nil {
					extraKernelArgs = extraKernelArgsRes.TypedSpec().Value.Args

					// alwaysInclude is used to track whether extra kernel args were defined explicitly at least once in this cluster machine's lifetime.
					//
					// If they were, from that moment on, we need to work with the "full" schematic ID to decide if we want to upgrade the machine or not.
					options.TypedSpec().Value.AlwaysIncludeKernelArgs = true
				}

				if configStatus != nil {
					desiredExtraKernelArgs = configStatus.TypedSpec().Value.ExtraKernelArgs
				}

				extraKernelArgsInSync := slices.Equal(extraKernelArgs, desiredExtraKernelArgs)
				if !extraKernelArgsInSync {
					logger.Info("kernel args out of sync, skipping install image update until they get in sync")
				}

				GenInstallConfig(machineStatus, clusterMachineTalosVersion, options, extraKernelArgs, extraKernelArgsInSync)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachineTalosVersion](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineExtraKernelArgs](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfigStatus](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithIgnoreTeardownUntil(), // keep the resource until everyone else is done with Machine
	)
}

// GenInstallConfig creates a config patch with an automatically picked install disk.
//
// InstallImage will be updated only if the kernel args are in sync, i.e., they are applied via the ClusterMachineConfig, and can be observed on the ClusterMachineConfigStatus.
//
// This ensures that any changes in the schematic are held back until the machine config has the correct .machine.install.extraKernelArgs.
//
// By doing so, we trigger a schematic upgrade only after updating the kernel args, making them take effect (in non-UKI systems).
func GenInstallConfig(machineStatus *omni.MachineStatus, clusterMachineTalosVersion *omni.ClusterMachineTalosVersion,
	genOptions *omni.MachineConfigGenOptions, extraKernelArgs []string, kernelArgsInSync bool,
) {
	if clusterMachineTalosVersion != nil && kernelArgsInSync {
		if genOptions.TypedSpec().Value.InstallImage == nil {
			genOptions.TypedSpec().Value.InstallImage = &specs.MachineConfigGenOptionsSpec_InstallImage{}
		}

		genOptions.TypedSpec().Value.InstallImage.SchematicId = clusterMachineTalosVersion.TypedSpec().Value.SchematicId
		genOptions.TypedSpec().Value.InstallImage.TalosVersion = clusterMachineTalosVersion.TypedSpec().Value.TalosVersion
		genOptions.TypedSpec().Value.InstallImage.SchematicInitialized = machineStatus.TypedSpec().Value.Schematic != nil

		if genOptions.TypedSpec().Value.InstallImage.SchematicInitialized {
			genOptions.TypedSpec().Value.InstallImage.SchematicInvalid = machineStatus.TypedSpec().Value.GetSchematic().GetInvalid()
		}

		genOptions.TypedSpec().Value.InstallImage.SecurityState = machineStatus.TypedSpec().Value.SecurityState
		genOptions.TypedSpec().Value.InstallImage.Platform = machineStatus.TypedSpec().Value.GetPlatformMetadata().GetPlatform()
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
	genOptions.TypedSpec().Value.ExtraKernelArgs = extraKernelArgs
}
