// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// ConnectionParamsController turns siderolink config resource into a ConnectionParams.
type ConnectionParamsController = qtransform.QController[*siderolink.Config, *siderolink.ConnectionParams]

// ConnectionParamsControllerName is the name of the connection params controller.
const ConnectionParamsControllerName = "ConnectionParamsController"

// NewConnectionParamsController instanciates the connection params controller.
func NewConnectionParamsController() *ConnectionParamsController {
	return qtransform.NewQController(
		qtransform.Settings[*siderolink.Config, *siderolink.ConnectionParams]{
			Name: ConnectionParamsControllerName,
			MapMetadataFunc: func(res *siderolink.Config) *siderolink.ConnectionParams {
				return siderolink.NewConnectionParams(resources.DefaultNamespace, res.Metadata().ID())
			},
			UnmapMetadataFunc: func(*siderolink.ConnectionParams) *siderolink.Config {
				return siderolink.NewConfig(resources.DefaultNamespace)
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, siderolinkConfig *siderolink.Config, params *siderolink.ConnectionParams) error {
				spec := params.TypedSpec().Value

				spec.ApiEndpoint = config.Config.SideroLinkAPIURL
				spec.WireguardEndpoint = siderolinkConfig.TypedSpec().Value.AdvertisedEndpoint
				spec.UseGrpcTunnel = config.Config.SiderolinkUseGRPCTunnel
				spec.LogsPort = int32(config.Config.LogServerPort)
				spec.EventsPort = int32(config.Config.EventSinkPort)

				defaultJoinToken, err := safe.ReaderGetByID[*auth.DefaultJoinToken](ctx, r, auth.DefaultJoinTokenID)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return err
				}

				spec.JoinToken = defaultJoinToken.TypedSpec().Value.TokenId

				url, err := siderolink.APIURL(params)
				if err != nil {
					return err
				}

				spec.Args = fmt.Sprintf("%s=%s %s=%s %s=tcp://%s",
					constants.KernelParamSideroLink,
					url,
					constants.KernelParamEventsSink,
					net.JoinHostPort(
						siderolinkConfig.TypedSpec().Value.ServerAddress,
						strconv.Itoa(config.Config.EventSinkPort),
					),
					constants.KernelParamLoggingKernel,
					net.JoinHostPort(
						siderolinkConfig.TypedSpec().Value.ServerAddress,
						strconv.Itoa(config.Config.LogServerPort),
					),
				)

				logger.Info(fmt.Sprintf("add this kernel argument to a Talos instance you would like to connect: %s", spec.Args))

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			func(context.Context, *zap.Logger, controller.QRuntime, *auth.DefaultJoinToken) ([]resource.Pointer, error) {
				return []resource.Pointer{
					siderolink.NewConfig(resources.DefaultNamespace).Metadata(),
				}, nil
			},
		),
		qtransform.WithExtraMappedInput(
			func(context.Context, *zap.Logger, controller.QRuntime, *auth.JoinToken) ([]resource.Pointer, error) {
				return []resource.Pointer{
					siderolink.NewConfig(resources.DefaultNamespace).Metadata(),
				}, nil
			},
		),
	)
}
