// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterEndpointController manages endpoints for each Cluster.
type ClusterEndpointController = qtransform.QController[*omni.Cluster, *omni.ClusterEndpoint]

// NewClusterEndpointController initializes ClusterEndpointController.
func NewClusterEndpointController() *ClusterEndpointController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterEndpoint]{
			Name: "ClusterEndpointController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterEndpoint {
				return omni.NewClusterEndpoint(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterEndpoint *omni.ClusterEndpoint) *omni.Cluster {
				return omni.NewCluster(clusterEndpoint.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, cluster *omni.Cluster, clusterEndpoint *omni.ClusterEndpoint) error {
				items, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r,
					state.WithLabelQuery(
						resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
						resource.LabelExists(omni.LabelControlPlaneRole),
					),
				)
				if err != nil {
					return fmt.Errorf("error listing cluster machine statuses: %w", err)
				}

				clusterEndpoint.TypedSpec().Value.ManagementAddresses = nil

				for val := range items.All() {
					if val.TypedSpec().Value.ManagementAddress == "" {
						continue
					}

					clusterEndpoint.TypedSpec().Value.ManagementAddresses = append(
						clusterEndpoint.TypedSpec().Value.ManagementAddresses,
						val.TypedSpec().Value.ManagementAddress,
					)
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			// reconcile on controlplane MachineSet changes
			mappers.MapByClusterLabelOnlyControlplane[*omni.Cluster](),
		),
	)
}
