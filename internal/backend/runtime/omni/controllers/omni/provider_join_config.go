// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// ProviderJoinConfigController is the controller that generates one global join config for the infra provider.
// This join config won't have the machine request ID inside.
type ProviderJoinConfigController = qtransform.QController[*infra.Provider, *siderolinkres.ProviderJoinConfig]

// NewProviderJoinConfigController initializes ProviderJoinConfigController.
func NewProviderJoinConfigController() *ProviderJoinConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*infra.Provider, *siderolinkres.ProviderJoinConfig]{
			Name: "ProviderJoinConfigController",
			MapMetadataFunc: func(provider *infra.Provider) *siderolinkres.ProviderJoinConfig {
				return siderolinkres.NewProviderJoinConfig(provider.Metadata().ID())
			},
			UnmapMetadataFunc: func(config *siderolinkres.ProviderJoinConfig) *infra.Provider {
				return infra.NewProvider(config.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, provider *infra.Provider, providerJoinConfig *siderolinkres.ProviderJoinConfig) error {
				siderolinkAPIConfig, err := safe.ReaderGetByID[*siderolinkres.APIConfig](ctx, r, siderolinkres.ConfigID)
				if err != nil {
					return err
				}

				if providerJoinConfig.TypedSpec().Value.JoinToken == "" {
					providerJoinConfig.TypedSpec().Value.JoinToken, err = jointoken.Generate()
					if err != nil {
						return err
					}

					logger.Info("generated join token for the infra provider")
				}

				joinOptions, err := siderolink.NewJoinOptions(
					siderolink.WithJoinToken(providerJoinConfig.TypedSpec().Value.JoinToken),
					siderolink.WithEventSinkPort(int(siderolinkAPIConfig.TypedSpec().Value.EventsPort)),
					siderolink.WithLogServerPort(int(siderolinkAPIConfig.TypedSpec().Value.LogsPort)),
					siderolink.WithMachineAPIURL(siderolinkAPIConfig.TypedSpec().Value.MachineApiAdvertisedUrl),
					siderolink.WithProvider(provider),
				)
				if err != nil {
					return err
				}

				config, err := joinOptions.RenderJoinConfig()
				if err != nil {
					return err
				}

				kernelArgs := joinOptions.GetKernelArgs()

				providerJoinConfig.TypedSpec().Value.Config = &specs.JoinConfig{
					Config:     string(config),
					KernelArgs: kernelArgs,
				}

				providerJoinConfig.Metadata().Labels().Set(omni.LabelInfraProviderID, provider.Metadata().ID())

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*siderolinkres.APIConfig](qtransform.MapperNone()),
		qtransform.WithConcurrency(4),
	)
}
