// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cluster

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ConfigVersionController manages config version for each cluster.
type ConfigVersionController struct {
	*qtransform.QController[*omni.Cluster, *omni.ClusterConfigVersion]
}

// NewConfigVersionController initializes ConfigVersionController.
func NewConfigVersionController() *ConfigVersionController {
	ctrl := &ConfigVersionController{}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterConfigVersion]{
			Name: "ClusterConfigVersionController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterConfigVersion {
				return omni.NewClusterConfigVersion(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterConfigVersion *omni.ClusterConfigVersion) *omni.Cluster {
				return omni.NewCluster(clusterConfigVersion.Metadata().ID())
			},
			TransformFunc: ctrl.transform,
		},
	)

	return ctrl
}

func (*ConfigVersionController) transform(_ context.Context, _ controller.Reader, _ *zap.Logger, cluster *omni.Cluster, clusterVersion *omni.ClusterConfigVersion) error {
	if cluster.TypedSpec().Value.TalosVersion != "" {
		if clusterVersion.TypedSpec().Value.Version == "" {
			clusterVersion.TypedSpec().Value.Version = "v" + cluster.TypedSpec().Value.TalosVersion
		}
	}

	return nil
}
