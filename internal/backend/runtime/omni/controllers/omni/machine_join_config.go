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
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// MachineJoinConfigController is the controller that generates per-machine siderolink JoinConfigs.
type MachineJoinConfigController = qtransform.QController[*omni.Machine, *siderolinkres.MachineJoinConfig]

// NewMachineJoinConfigController initializes MachineJoinConfigController.
func NewMachineJoinConfigController() *MachineJoinConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Machine, *siderolinkres.MachineJoinConfig]{
			Name: "MachineJoinConfigController",
			MapMetadataFunc: func(provider *omni.Machine) *siderolinkres.MachineJoinConfig {
				return siderolinkres.NewMachineJoinConfig(provider.Metadata().ID())
			},
			UnmapMetadataFunc: func(config *siderolinkres.MachineJoinConfig) *omni.Machine {
				return omni.NewMachine(resources.DefaultNamespace, config.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, machine *omni.Machine, machineJoinConfig *siderolinkres.MachineJoinConfig) error {
				tokenUsage, err := safe.ReaderGetByID[*siderolinkres.JoinTokenUsage](ctx, r, machine.Metadata().ID())
				if err != nil {
					return err
				}

				parsedTokenUsage, err := jointoken.Parse(tokenUsage.TypedSpec().Value.TokenId)
				if err != nil {
					return err
				}

				siderolinkAPIConfig, err := safe.ReaderGetByID[*siderolinkres.APIConfig](ctx, r, siderolinkres.ConfigID)
				if err != nil {
					return err
				}

				joinOptions, err := siderolink.NewJoinOptions(
					siderolink.WithJoinToken(tokenUsage.TypedSpec().Value.TokenId),
					siderolink.WithEventSinkPort(int(siderolinkAPIConfig.TypedSpec().Value.EventsPort)),
					siderolink.WithLogServerPort(int(siderolinkAPIConfig.TypedSpec().Value.LogsPort)),
					siderolink.WithMachineAPIURL(siderolinkAPIConfig.TypedSpec().Value.MachineApiAdvertisedUrl),
					siderolink.WithMachine(machine),
					siderolink.WithVersion(parsedTokenUsage.Version),
				)
				if err != nil {
					return err
				}

				config, err := joinOptions.RenderJoinConfig()
				if err != nil {
					return err
				}

				kernelArgs := joinOptions.GetKernelArgs()

				machineJoinConfig.TypedSpec().Value.Config = &specs.JoinConfig{
					Config:     string(config),
					KernelArgs: kernelArgs,
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(qtransform.MapperNone[*siderolinkres.APIConfig]()),
		qtransform.WithExtraMappedInput(qtransform.MapperSameID[*siderolinkres.JoinTokenUsage, *omni.Machine]()),
		qtransform.WithConcurrency(4),
	)
}
