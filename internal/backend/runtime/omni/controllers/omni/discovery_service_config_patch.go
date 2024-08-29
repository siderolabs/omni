// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// DiscoveryServiceConfigPatchPrefix is the prefix for the system config patch that contains the discovery service endpoint config.
const DiscoveryServiceConfigPatchPrefix = "950-discovery-service-"

// DiscoveryServiceConfigPatchController creates the system config patch that contains the discovery service endpoint config.
//
// It sets the cluster discovery service endpoint to the embedded discovery service if the feature is available for the Omni instance and enabled for the cluster.
type DiscoveryServiceConfigPatchController = qtransform.QController[*omni.ClusterStatus, *omni.ConfigPatch]

// NewDiscoveryServiceConfigPatchController initializes DiscoveryServiceConfigPatchController.
func NewDiscoveryServiceConfigPatchController(embeddedDiscoveryServicePort int) *DiscoveryServiceConfigPatchController {
	getPatch := sync.OnceValues(func() ([]byte, error) {
		var buf bytes.Buffer

		encoder := yaml.NewEncoder(&buf)

		encoder.SetIndent(2)

		if err := encoder.Encode(map[string]any{
			"cluster": map[string]any{
				"discovery": map[string]any{
					"registries": map[string]any{
						"service": map[string]any{
							"endpoint": "http://" + net.JoinHostPort(siderolink.ListenHost, strconv.Itoa(embeddedDiscoveryServicePort)),
						},
					},
				},
			},
		}); err != nil {
			return nil, fmt.Errorf("failed to encode patch: %w", err)
		}

		return buf.Bytes(), nil
	})

	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterStatus, *omni.ConfigPatch]{
			Name: "DiscoveryServiceConfigPatchController",
			MapMetadataFunc: func(cluster *omni.ClusterStatus) *omni.ConfigPatch {
				return omni.NewConfigPatch(resources.DefaultNamespace, DiscoveryServiceConfigPatchPrefix+cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(configPatch *omni.ConfigPatch) *omni.ClusterStatus {
				id := strings.TrimPrefix(configPatch.Metadata().ID(), DiscoveryServiceConfigPatchPrefix)

				return omni.NewClusterStatus(resources.DefaultNamespace, id)
			},
			TransformExtraOutputFunc: func(_ context.Context, _ controller.ReaderWriter, _ *zap.Logger, cluster *omni.ClusterStatus, configPatch *omni.ConfigPatch) error {
				configPatch.Metadata().Labels().Set(omni.LabelSystemPatch, "")
				configPatch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

				if !cluster.TypedSpec().Value.UseEmbeddedDiscoveryService {
					// not enabled, remove the patch
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("embedded discovery service is disabled for cluster")
				}

				patchYAML, err := getPatch()
				if err != nil {
					return err
				}

				return configPatch.TypedSpec().Value.SetUncompressedData(patchYAML)
			},
		},
		qtransform.WithConcurrency(2),
		qtransform.WithOutputKind(controller.OutputShared),
	)
}
