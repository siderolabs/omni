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

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// BMCConfigController manages BMCConfig resource lifecycle.
type BMCConfigController = qtransform.QController[*omni.InfraMachineBMCConfig, *infra.BMCConfig]

// NewBMCConfigController initializes BMCConfigController.
func NewBMCConfigController() *BMCConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.InfraMachineBMCConfig, *infra.BMCConfig]{
			Name: "BMCConfigController",
			MapMetadataFunc: func(config *omni.InfraMachineBMCConfig) *infra.BMCConfig {
				return infra.NewBMCConfig(config.Metadata().ID())
			},
			UnmapMetadataFunc: func(config *infra.BMCConfig) *omni.InfraMachineBMCConfig {
				return omni.NewInfraMachineBMCConfig(config.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, userConfig *omni.InfraMachineBMCConfig, config *infra.BMCConfig) error {
				link, err := safe.ReaderGetByID[*siderolink.Link](ctx, r, userConfig.Metadata().ID())
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("no matching link found")
					}

					return err
				}

				providerID, ok := link.Metadata().Labels().Get(omni.LabelInfraProviderID)
				if !ok {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("the link is not created by an infra provider")
				}

				config.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

				config.TypedSpec().Value.Config = userConfig.TypedSpec().Value

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*siderolink.Link](qtransform.MapperSameID[*omni.InfraMachineBMCConfig]()),
	)
}
