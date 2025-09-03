// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// BMCConfigController manages BMCConfig resource lifecycle.
type BMCConfigController = qtransform.QController[*omni.InfraMachineBMCConfig, *infra.BMCConfig]

// NewBMCConfigController initializes BMCConfigController.
func NewBMCConfigController() *BMCConfigController {
	name := "BMCConfigController"

	return qtransform.NewQController(
		qtransform.Settings[*omni.InfraMachineBMCConfig, *infra.BMCConfig]{
			Name: name,
			MapMetadataFunc: func(config *omni.InfraMachineBMCConfig) *infra.BMCConfig {
				return infra.NewBMCConfig(config.Metadata().ID())
			},
			UnmapMetadataFunc: func(config *infra.BMCConfig) *omni.InfraMachineBMCConfig {
				return omni.NewInfraMachineBMCConfig(config.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, userConfig *omni.InfraMachineBMCConfig, config *infra.BMCConfig) error {
				link, err := helpers.HandleInput[*siderolink.Link](ctx, r, name, userConfig)
				if err != nil {
					return err
				}

				if link == nil {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("no matching link found")
				}

				providerID, ok := link.Metadata().Annotations().Get(omni.LabelInfraProviderID)
				if !ok {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("the link is not created by an infra provider")
				}

				config.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

				config.TypedSpec().Value.Config = userConfig.TypedSpec().Value

				return nil
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, writer controller.ReaderWriter, _ *zap.Logger, userConfig *omni.InfraMachineBMCConfig) error {
				if _, err := helpers.HandleInput[*siderolink.Link](ctx, writer, name, userConfig); err != nil {
					return err
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*siderolink.Link](qtransform.MapperSameID[*omni.InfraMachineBMCConfig]()),
	)
}
