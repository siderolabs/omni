// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/image-factory/pkg/client"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// TalosExtensionsController creates omni.TalosExtensions for each omni.TalosVersion.
//
// TalosExtensionsController generates information about Talos extensions available for a Talos version.
type TalosExtensionsController = qtransform.QController[*omni.TalosVersion, *omni.TalosExtensions]

// NewTalosExtensionsController instantiates the TalosExtensions controller.
func NewTalosExtensionsController() *TalosExtensionsController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.TalosVersion, *omni.TalosExtensions]{
			Name: "TalosExtensionsController",
			MapMetadataFunc: func(version *omni.TalosVersion) *omni.TalosExtensions {
				return omni.NewTalosExtensions(resources.DefaultNamespace, version.Metadata().ID())
			},
			UnmapMetadataFunc: func(r *omni.TalosExtensions) *omni.TalosVersion {
				return omni.NewTalosVersion(resources.DefaultNamespace, r.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, _ controller.Reader, _ *zap.Logger, version *omni.TalosVersion, extensionsResource *omni.TalosExtensions) error {
				factoryClient, err := client.New(config.Config.ImageFactoryBaseURL)
				if err != nil {
					return fmt.Errorf("failed to get image factory client %w", err)
				}

				versions, err := factoryClient.Versions(ctx)
				if err != nil {
					return fmt.Errorf("failed to get existing image factory Talos versions %w", err)
				}

				hasVersion := slices.Index(versions, version.Metadata().ID()) != -1

				// skip fetching Talos extensions for a version which isn't registered in the image factory
				if !hasVersion {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("version %q is not registered in the image factory", version.Metadata().ID())
				}

				extensionsVersions, err := factoryClient.ExtensionsVersions(ctx, version.TypedSpec().Value.Version)
				if err != nil {
					return controller.NewRequeueErrorf(time.Minute*5, "failed to get extension versions from the image factory %w", err)
				}

				extensionsResource.TypedSpec().Value.Items = make([]*specs.TalosExtensionsSpec_Info, 0, len(extensionsVersions))

				for _, ev := range extensionsVersions {
					info := &specs.TalosExtensionsSpec_Info{
						Name:        ev.Name,
						Digest:      ev.Digest,
						Ref:         ev.Ref,
						Author:      ev.Author,
						Description: ev.Description,
					}

					_, info.Version, _ = strings.Cut(ev.Ref, ":")

					extensionsResource.TypedSpec().Value.Items = append(extensionsResource.TypedSpec().Value.Items, info)
				}

				return nil
			},
		},
	)
}
