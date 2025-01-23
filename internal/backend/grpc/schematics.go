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
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// CreateSchematic implements ManagementServer.
func (s *managementServer) CreateSchematic(ctx context.Context, request *management.CreateSchematicRequest) (*management.CreateSchematicResponse, error) {
	// creating a schematic is equivalent to creating a machine
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return nil, err
	}

	baseKernelArgs, err := s.getBaseKernelArgs(ctx, request.SiderolinkGrpcTunnelMode)
	if err != nil {
		return nil, err
	}

	customization := schematic.Customization{
		ExtraKernelArgs: append(baseKernelArgs, request.ExtraKernelArgs...),
		SystemExtensions: schematic.SystemExtensions{
			OfficialExtensions: request.Extensions,
		},
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
				return nil, status.Errorf(codes.InvalidArgument, "'machineInitialLabels' is deprecated")
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

	media, err := safe.ReaderGetByID[*omni.InstallationMedia](ctx, s.omniState, request.MediaId)
	if err != nil {
		return nil, err
	}

	supportsOverlays := quirks.New(request.TalosVersion).SupportsOverlay()

	if media.TypedSpec().Value.Overlay != "" && supportsOverlays {
		var overlays []client.OverlayInfo

		overlays, err = s.imageFactoryClient.Client.OverlaysVersions(ctx, request.TalosVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to get overlays list for the Talos version: %w", err)
		}

		index := slices.IndexFunc(overlays, func(value client.OverlayInfo) bool {
			return value.Name == media.TypedSpec().Value.Overlay
		})

		if index == -1 {
			return nil, fmt.Errorf("failed to find overlay with name %q", media.TypedSpec().Value.Overlay)
		}

		overlay := overlays[index]

		schematicRequest.Overlay = schematic.Overlay{
			Name:  overlay.Name,
			Image: overlay.Image,
		}
	}

	s.logger.Info("ensure schematic", zap.Reflect("schematic", schematicRequest))

	schematicInfo, err := s.imageFactoryClient.EnsureSchematic(ctx, schematicRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure schematic: %w", err)
	}

	pxeURL, err := config.Config.GetImageFactoryPXEBaseURL()
	if err != nil {
		return nil, err
	}

	filename := media.TypedSpec().Value.GenerateFilename(!supportsOverlays, request.SecureBoot, false)

	return &management.CreateSchematicResponse{
		SchematicId: schematicInfo.FullID,
		PxeUrl:      pxeURL.JoinPath("pxe", schematicInfo.FullID, request.TalosVersion, filename).String(),
	}, nil
}

func (s *managementServer) getBaseKernelArgs(ctx context.Context, grpcTunnelMode management.CreateSchematicRequest_SiderolinkGRPCTunnelMode) ([]string, error) {
	params, err := safe.StateGet[*siderolink.ConnectionParams](ctx, s.omniState, siderolink.NewConnectionParams(
		resources.DefaultNamespace,
		siderolink.ConfigID,
	).Metadata())
	if err != nil {
		return nil, fmt.Errorf("failed to get Omni connection params for the extra kernel arguments: %w", err)
	}

	switch grpcTunnelMode {
	case management.CreateSchematicRequest_AUTO:
		return siderolink.KernelArgs(params), nil
	case management.CreateSchematicRequest_ENABLED:
		return siderolink.KernelArgsWithGRPCRTunnelMode(params, true)
	case management.CreateSchematicRequest_DISABLED:
		return siderolink.KernelArgsWithGRPCRTunnelMode(params, false)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid GRPC tunnel mode: %s", grpcTunnelMode)
	}
}
