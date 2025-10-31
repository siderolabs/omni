// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// ConnectionParamsController turns siderolink config resource into a ConnectionParams.
type ConnectionParamsController = qtransform.QController[*siderolinkres.Config, *siderolinkres.ConnectionParams] //nolint:staticcheck

// ConnectionParamsControllerName is the name of the connection params controller.
const ConnectionParamsControllerName = "ConnectionParamsController"

// NewConnectionParamsController instanciates the connection params controller.
func NewConnectionParamsController() *ConnectionParamsController {
	return qtransform.NewQController(
		qtransform.Settings[*siderolinkres.Config, *siderolinkres.ConnectionParams]{ //nolint:staticcheck
			Name: ConnectionParamsControllerName,
			MapMetadataFunc: func(res *siderolinkres.Config) *siderolinkres.ConnectionParams { //nolint:staticcheck
				return siderolinkres.NewConnectionParams(resources.DefaultNamespace, res.Metadata().ID()) //nolint:staticcheck
			},
			UnmapMetadataFunc: func(*siderolinkres.ConnectionParams) *siderolinkres.Config { //nolint:staticcheck
				return siderolinkres.NewConfig()
			},
			//nolint:staticcheck
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, siderolinkConfig *siderolinkres.Config, params *siderolinkres.ConnectionParams) error {
				spec := params.TypedSpec().Value

				spec.ApiEndpoint = config.Config.Services.MachineAPI.URL()
				spec.WireguardEndpoint = siderolinkConfig.TypedSpec().Value.AdvertisedEndpoint
				spec.UseGrpcTunnel = config.Config.Services.Siderolink.UseGRPCTunnel
				spec.LogsPort = int32(config.Config.Services.Siderolink.LogServerPort)
				spec.EventsPort = int32(config.Config.Services.Siderolink.EventSinkPort)

				defaultJoinToken, err := safe.ReaderGetByID[*siderolinkres.DefaultJoinToken](ctx, r, siderolinkres.DefaultJoinTokenID)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return err
				}

				spec.JoinToken = defaultJoinToken.TypedSpec().Value.TokenId

				opts, err := siderolink.NewJoinOptions(
					siderolink.WithMachineAPIURL(spec.ApiEndpoint),
					siderolink.WithEventSinkPort(int(spec.EventsPort)),
					siderolink.WithLogServerPort(int(spec.LogsPort)),
					siderolink.WithJoinToken(spec.JoinToken),
					siderolink.WithGRPCTunnel(spec.UseGrpcTunnel),
				)
				if err != nil {
					return err
				}

				spec.Args = strings.Join(opts.GetKernelArgs(), " ")

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*siderolinkres.DefaultJoinToken](
			func(context.Context, *zap.Logger, controller.QRuntime, controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				return []resource.Pointer{
					siderolinkres.NewConfig().Metadata(),
				}, nil
			},
		),
	)
}
