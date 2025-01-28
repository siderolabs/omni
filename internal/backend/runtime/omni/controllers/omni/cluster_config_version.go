// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ClusterConfigVersionController manages config version for each cluster.
type ClusterConfigVersionController = qtransform.QController[*omni.Cluster, *omni.ClusterConfigVersion]

// NewClusterConfigVersionController initializes ClusterConfigVersionController.
func NewClusterConfigVersionController() *ClusterConfigVersionController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterConfigVersion]{
			Name: "ClusterConfigVersionController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterConfigVersion {
				return omni.NewClusterConfigVersion(resources.DefaultNamespace, cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterConfigVersion *omni.ClusterConfigVersion) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, clusterConfigVersion.Metadata().ID())
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, cluster *omni.Cluster, clusterVersion *omni.ClusterConfigVersion) error {
				if cluster.TypedSpec().Value.TalosVersion != "" {
					if clusterVersion.TypedSpec().Value.Version == "" {
						clusterVersion.TypedSpec().Value.Version = "v" + cluster.TypedSpec().Value.TalosVersion
					}
				}

				return nil
			},
		},
	)
}
