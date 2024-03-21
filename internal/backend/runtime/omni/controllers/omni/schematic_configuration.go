// Copyright (c) 2024 Sidero Labs, Inc.
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
	"github.com/siderolabs/image-factory/pkg/schematic"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// SchematicConfigurationController generates schematic configuration from the extensions list.
// Ensures that the schematic ID exists in the image factory by calling the image factory API.
type SchematicConfigurationController = qtransform.QController[*omni.ExtensionsConfiguration, *omni.SchematicConfiguration]

// NewSchematicConfigurationController initializes SchematicConfigurationController.
func NewSchematicConfigurationController(imageFactoryClient *imagefactory.Client) *SchematicConfigurationController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ExtensionsConfiguration, *omni.SchematicConfiguration]{
			Name: "SchematicConfigurationController",
			MapMetadataFunc: func(extensionsConfiguration *omni.ExtensionsConfiguration) *omni.SchematicConfiguration {
				return omni.NewSchematicConfiguration(resources.DefaultNamespace, extensionsConfiguration.Metadata().ID())
			},
			UnmapMetadataFunc: func(schematicConfiguration *omni.SchematicConfiguration) *omni.ExtensionsConfiguration {
				return omni.NewExtensionsConfiguration(resources.DefaultNamespace, schematicConfiguration.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger,
				extensionsConfiguration *omni.ExtensionsConfiguration, schematicConfiguration *omni.SchematicConfiguration,
			) error {
				config := schematic.Schematic{
					Customization: schematic.Customization{
						SystemExtensions: schematic.SystemExtensions{
							OfficialExtensions: extensionsConfiguration.TypedSpec().Value.Extensions,
						},
					},
				}

				id, err := config.ID()
				if err != nil {
					return err
				}

				if _, err := safe.ReaderGetByID[*omni.Schematic](ctx, r, id); err != nil {
					if !state.IsNotFoundError(err) {
						return err
					}

					if _, err = imageFactoryClient.EnsureSchematic(ctx, config); err != nil {
						if e := updateStatus(ctx, r, extensionsConfiguration, nil); e != nil {
							return e
						}

						return err
					}
				}

				helpers.CopyAllLabels(extensionsConfiguration, schematicConfiguration)

				schematicConfiguration.TypedSpec().Value.SchematicId = id

				return updateStatus(ctx, r, extensionsConfiguration, nil)
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*omni.Schematic](),
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.ExtensionsConfigurationStatusType,
				Kind: controller.OutputExclusive,
			},
		),
		qtransform.WithOutputKind(controller.OutputShared),
	)
}

func updateStatus(ctx context.Context, r controller.ReaderWriter, extensionsConfiguration *omni.ExtensionsConfiguration, err error) error {
	status := omni.NewExtensionsConfigurationStatus(resources.DefaultNamespace, extensionsConfiguration.Metadata().ID())

	return safe.WriterModify(ctx, r, status, func(res *omni.ExtensionsConfigurationStatus) error {
		helpers.CopyAllLabels(extensionsConfiguration, res)

		if err != nil {
			res.TypedSpec().Value.Error = err.Error()
			res.TypedSpec().Value.Phase = specs.ExtensionsConfigurationStatusSpec_Failed

			return nil //nolint:nilerr
		}

		res.TypedSpec().Value.Error = ""
		res.TypedSpec().Value.Phase = specs.ExtensionsConfigurationStatusSpec_Ready
		res.TypedSpec().Value.Extensions = extensionsConfiguration.TypedSpec().Value.Extensions

		return nil
	})
}
