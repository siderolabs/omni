// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
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

	params, err := safe.StateGet[*siderolink.ConnectionParams](ctx, s.omniState, siderolink.NewConnectionParams(
		resources.DefaultNamespace,
		siderolink.ConfigID,
	).Metadata())
	if err != nil {
		return nil, fmt.Errorf("failed to get Omni connection params for the extra kernel arguments: %w", err)
	}

	customization := schematic.Customization{
		ExtraKernelArgs: append(strings.Split(params.TypedSpec().Value.Args, " "), request.ExtraKernelArgs...),
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

	schematic := schematic.Schematic{
		Customization: customization,
	}

	schematicInfo, err := s.imageFactoryClient.EnsureSchematic(ctx, schematic)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure schematic: %w", err)
	}

	pxeURL, err := config.Config.GetImageFactoryPXEBaseURL()
	if err != nil {
		return nil, err
	}

	return &management.CreateSchematicResponse{
		SchematicId: schematicInfo.FullID,
		PxeUrl:      pxeURL.JoinPath("pxe", schematicInfo.FullID).String(),
	}, nil
}
