// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// InfraProviderCombinedStatusController merges infra.Provider and infra.ProviderHealthStatus into a single status resource.
type InfraProviderCombinedStatusController = qtransform.QController[*infra.Provider, *omni.InfraProviderCombinedStatus]

// NewInfraProviderCombinedStatusController initializes InfraProviderCombinedStatusController.
func NewInfraProviderCombinedStatusController() *InfraProviderCombinedStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*infra.Provider, *omni.InfraProviderCombinedStatus]{
			Name: "InfraProviderStatusController",
			MapMetadataFunc: func(res *infra.Provider) *omni.InfraProviderCombinedStatus {
				return omni.NewInfraProviderCombinedStatus(res.Metadata().ID())
			},
			UnmapMetadataFunc: func(res *omni.InfraProviderCombinedStatus) *infra.Provider {
				return infra.NewProvider(res.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, provider *infra.Provider, infraProviderStatus *omni.InfraProviderCombinedStatus) error {
				providerHealthStatus, err := safe.ReaderGetByID[*infra.ProviderHealthStatus](ctx, r, provider.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				providerStatus, err := safe.ReaderGetByID[*infra.ProviderStatus](ctx, r, provider.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				infraProviderStatus.TypedSpec().Value.Health = &specs.InfraProviderCombinedStatusSpec_Health{}

				if providerHealthStatus != nil {
					lastHeartbeatTime := providerHealthStatus.TypedSpec().Value.LastHeartbeatTimestamp.AsTime()

					infraProviderStatus.TypedSpec().Value.Health.Connected = time.Since(lastHeartbeatTime) < constants.InfraProviderHealthCheckInterval+time.Second

					infraProviderStatus.TypedSpec().Value.Health.Error = providerHealthStatus.TypedSpec().Value.Error
				}

				if providerStatus != nil {
					infraProviderStatus.TypedSpec().Value.Description = providerStatus.TypedSpec().Value.Description
					infraProviderStatus.TypedSpec().Value.Icon = providerStatus.TypedSpec().Value.Icon
					infraProviderStatus.TypedSpec().Value.Name = providerStatus.TypedSpec().Value.Name

					infraProviderStatus.TypedSpec().Value.Health.Initialized = true
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*infra.ProviderHealthStatus](
			qtransform.MapperSameID[*infra.Provider](),
		),
		qtransform.WithExtraMappedInput[*infra.ProviderStatus](
			qtransform.MapperSameID[*infra.Provider](),
		),
		qtransform.WithOutputKind(controller.OutputShared),
	)
}
