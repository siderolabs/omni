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
	"github.com/siderolabs/gen/xerrors"
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

				schematicConfiguration, err := safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, r, machineStatus.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				installImage := options.TypedSpec().Value.InstallImage

				if clusterMachineTalosVersion == nil {
					// A free machine keeps its install image as the record of what it runs. A host missing from
					// it (enrolled before the host was tracked) is filled in from the schematic's record.
					if installImage != nil && installImage.ImageFactoryHost == "" {
						installImage.ImageFactoryHost = schematicFactoryHost(schematicConfiguration, installImage.SchematicId, installImage.TalosVersion, imageFactoryClients)
					}

					return nil
				}

				talosVersion := clusterMachineTalosVersion.TypedSpec().Value.TalosVersion
				schematicID := clusterMachineTalosVersion.TypedSpec().Value.SchematicId

				// The host is the factory that issued the schematic in use, recorded when it was ensured for the
				// target Talos version. While the record covers another schematic or version (an upgrade in
				// flight, a resource from before the record existed), the install image already published for
				// this very schematic and version keeps its host. With neither, nothing is published rather
				// than a guess: an install image naming a factory that does not know its schematic only fails
				// the pull.
				imageFactoryHost := schematicFactoryHost(schematicConfiguration, schematicID, talosVersion, imageFactoryClients)

				if imageFactoryHost == "" && installImage != nil && installImage.SchematicId == schematicID && installImage.TalosVersion == talosVersion {
					imageFactoryHost = installImage.ImageFactoryHost
				}

				if imageFactoryHost == "" && schematicID != "" {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("the image factory of schematic %q is not known yet", schematicID)
				}

				options.TypedSpec().Value.InstallImage = omni.NewInstallImage(
					machineStatus,
					talosVersion,
					schematicID,
					imageFactoryHost,
					machineStatus.TypedSpec().Value.SchematicReady(),
				)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachineTalosVersion](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.SchematicConfiguration](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithIgnoreTeardownUntil(), // keep the resource until everyone else is done with Machine
	)
}

// schematicFactoryHost returns the host of the factory that issued the schematic for the Talos version, as
// recorded on the SchematicConfiguration, or empty while the record covers another schematic ID or version, or
// names no configured factory.
func schematicFactoryHost(schematicConfiguration *omni.SchematicConfiguration, schematicID, talosVersion string, imageFactoryClients ImageFactoryClientProvider) string {
	if schematicConfiguration == nil ||
		schematicConfiguration.TypedSpec().Value.SchematicId != schematicID ||
		schematicConfiguration.TypedSpec().Value.TalosVersion != talosVersion {
		return ""
	}

	imageFactoryClient := imageFactoryClients.ForURL(schematicConfiguration.TypedSpec().Value.ImageFactoryUrl)
	if imageFactoryClient == nil {
		return ""
	}

	return imageFactoryClient.Host()
}
