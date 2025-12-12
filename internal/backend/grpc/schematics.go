// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// CreateSchematic implements ManagementServer.
func (s *managementServer) CreateSchematic(ctx context.Context, request *management.CreateSchematicRequest) (*management.CreateSchematicResponse, error) {
	// creating a schematic is equivalent to creating a machine
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return nil, err
	}

	baseKernelArgs, tunnelEnabled, err := s.getBaseKernelArgs(ctx, request.SiderolinkGrpcTunnelMode, request.JoinToken)
	if err != nil {
		return nil, err
	}

	slices.Sort(request.Extensions)
	request.Extensions = slices.Compact(request.Extensions)

	customization := schematic.Customization{
		ExtraKernelArgs: append(baseKernelArgs, request.ExtraKernelArgs...),
		SystemExtensions: schematic.SystemExtensions{
			OfficialExtensions: request.Extensions,
		},
	}

	if request.Bootloader != management.SchematicBootloader_BOOT_AUTO {
		customization.Bootloader = profile.BootloaderKind(request.Bootloader)
	}

	for key, value := range request.MetaValues {
		if !meta.CanSetMetaKey(int(key)) {
			return nil, status.Errorf(codes.InvalidArgument, "meta key %s is not allowed to be set in the schematic, as it's reserved by Talos", runtime.MetaKeyTagToID(uint8(key)))
		}

		customization.Meta = append(customization.Meta, schematic.MetaValue{
			Key:   uint8(key),
			Value: value,
		})

		// validate that labels meta has the correct format
		if key == meta.LabelsMeta {
			var labels *meta.ImageLabels

			labels, err = meta.ParseLabels([]byte(value))
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "failed to parse machine labels: %s", err)
			}

			if labels.Labels == nil {
				return nil, status.Errorf(codes.InvalidArgument, "machine labels are empty")
			}

			if labels.LegacyLabels != nil {
				return nil, status.Errorf(codes.InvalidArgument, "'initialMachineLabels' is deprecated")
			}
		}
	}

	slices.SortFunc(customization.Meta, func(a, b schematic.MetaValue) int {
		switch {
		case a.Key < b.Key:
			return 1
		case a.Key > b.Key:
			return -1
		}

		return 0
	})

	schematicRequest := schematic.Schematic{
		Customization: customization,
	}

	s.logger.Info("ensure schematic", zap.Reflect("schematic", schematicRequest))

	schematicRequest.Overlay, err = s.getOverlay(ctx, request)
	if err != nil {
		return nil, err
	}

	schematicInfo, err := s.imageFactoryClient.EnsureSchematic(ctx, schematicRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure schematic: %w", err)
	}

	schematicYml, err := yaml.Marshal(schematicRequest)
	if err != nil {
		return nil, err
	}

	return &management.CreateSchematicResponse{
		SchematicId:       schematicInfo.FullID,
		SchematicYml:      string(schematicYml),
		GrpcTunnelEnabled: tunnelEnabled,
	}, nil
}

func (s *managementServer) getOverlay(ctx context.Context, req *management.CreateSchematicRequest) (schematic.Overlay, error) {
	if !quirks.New(req.TalosVersion).SupportsOverlay() {
		return schematic.Overlay{}, nil
	}

	if req.Overlay != nil {
		options := map[string]any{}

		if req.Overlay.Options != "" {
			if err := yaml.Unmarshal([]byte(req.Overlay.Options), &options); err != nil {
				return schematic.Overlay{}, err
			}
		}

		return schematic.Overlay{
			Name:    req.Overlay.Name,
			Image:   req.Overlay.Image,
			Options: options,
		}, nil
	}

	if req.MediaId == "" {
		return schematic.Overlay{}, nil
	}

	media, err := safe.ReaderGetByID[*omni.InstallationMedia](ctx, s.omniState, req.MediaId)
	if err != nil {
		return schematic.Overlay{}, err
	}

	if media.TypedSpec().Value.Overlay == "" {
		return schematic.Overlay{}, nil
	}

	overlays, err := s.imageFactoryClient.OverlaysVersions(ctx, req.TalosVersion)
	if err != nil {
		return schematic.Overlay{}, fmt.Errorf("failed to get overlays list for the Talos version: %w", err)
	}

	index := slices.IndexFunc(overlays, func(value client.OverlayInfo) bool {
		return value.Name == media.TypedSpec().Value.Overlay
	})

	if index == -1 {
		return schematic.Overlay{}, fmt.Errorf("failed to find overlay with name %q", media.TypedSpec().Value.Overlay)
	}

	overlay := overlays[index]

	return schematic.Overlay{
		Name:  overlay.Name,
		Image: overlay.Image,
	}, nil
}

func (s *managementServer) getBaseKernelArgs(
	ctx context.Context,
	grpcTunnelMode management.CreateSchematicRequest_SiderolinkGRPCTunnelMode,
	joinToken string,
) (args []string, tunnelEnabled bool, err error) {
	siderolinkAPIConfig, err := safe.StateGetByID[*siderolinkres.APIConfig](ctx, s.omniState, siderolinkres.ConfigID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get Omni connection params for the extra kernel arguments: %w", err)
	}

	// If the tunnel is enabled instance-wide or in the request, the final state is enabled
	grpcTunnelEnabled := grpcTunnelMode == management.CreateSchematicRequest_ENABLED

	if joinToken == "" {
		var defaultToken *siderolinkres.DefaultJoinToken

		defaultToken, err = safe.StateGetByID[*siderolinkres.DefaultJoinToken](ctx, s.omniState, siderolinkres.DefaultJoinTokenID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get Omni connection params for the extra kernel arguments: %w", err)
		}

		joinToken = defaultToken.TypedSpec().Value.TokenId
	}

	opts, err := siderolink.NewJoinOptions(
		siderolink.WithJoinToken(joinToken),
		siderolink.WithGRPCTunnel(grpcTunnelEnabled),
		siderolink.WithMachineAPIURL(siderolinkAPIConfig.TypedSpec().Value.MachineApiAdvertisedUrl),
		siderolink.WithEventSinkPort(int(siderolinkAPIConfig.TypedSpec().Value.EventsPort)),
		siderolink.WithLogServerPort(int(siderolinkAPIConfig.TypedSpec().Value.LogsPort)),
	)
	if err != nil {
		return nil, false, err
	}

	return opts.GetKernelArgs(), grpcTunnelEnabled, nil
}
