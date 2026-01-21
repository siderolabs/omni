// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// SiderolinkAPIConfigController turns siderolink config resource into a SiderolinkAPIConfig.
type SiderolinkAPIConfigController = qtransform.QController[*siderolink.Config, *siderolink.APIConfig]

// SiderolinkAPIConfigControllerName is the name of the connection params controller.
const SiderolinkAPIConfigControllerName = "SiderolinkAPIConfigController"

// NewSiderolinkAPIConfigController instanciates the connection params controller.
func NewSiderolinkAPIConfigController(services *config.Services) *SiderolinkAPIConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*siderolink.Config, *siderolink.APIConfig]{
			Name: SiderolinkAPIConfigControllerName,
			MapMetadataFunc: func(*siderolink.Config) *siderolink.APIConfig {
				return siderolink.NewAPIConfig()
			},
			UnmapMetadataFunc: func(*siderolink.APIConfig) *siderolink.Config {
				return siderolink.NewConfig()
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, _ *siderolink.Config, params *siderolink.APIConfig) error {
				spec := params.TypedSpec().Value

				spec.MachineApiAdvertisedUrl = services.MachineAPI.URL()
				spec.WireguardAdvertisedEndpoint = services.Siderolink.WireGuard.AdvertisedEndpoint
				spec.EnforceGrpcTunnel = services.Siderolink.UseGRPCTunnel
				spec.LogsPort = int32(services.Siderolink.LogServerPort)
				spec.EventsPort = int32(services.Siderolink.EventSinkPort)

				return nil
			},
		},
	)
}
