// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ClusterUUIDController manages ClusterUUID resource lifecycle.
//
// ClusterUUIDController generates cluster UUID for every cluster.
type ClusterUUIDController = qtransform.QController[*omni.Cluster, *omni.ClusterUUID]

// NewClusterUUIDController initializes ClusterUUIDController.
func NewClusterUUIDController() *ClusterUUIDController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterUUID]{
			Name: "ClusterUUIDController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterUUID {
				return omni.NewClusterUUID(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterUUID *omni.ClusterUUID) *omni.Cluster {
				return omni.NewCluster(clusterUUID.Metadata().Namespace(), clusterUUID.Metadata().ID())
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, cluster *omni.Cluster, clusterUUID *omni.ClusterUUID) error {
				if clusterUUID.TypedSpec().Value.GetUuid() != "" {
					if existingUUIDLabel, _ := clusterUUID.Metadata().Labels().Get(omni.LabelClusterUUID); existingUUIDLabel != clusterUUID.TypedSpec().Value.GetUuid() {
						clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, clusterUUID.TypedSpec().Value.GetUuid())
					}

					return nil
				}

				generatedUUUID, err := uuid.NewRandom()
				if err != nil {
					return fmt.Errorf("error generating cluster UUID for cluster '%s': %w", cluster.Metadata().ID(), err)
				}

				uuidStr := generatedUUUID.String()
				clusterUUID.TypedSpec().Value.Uuid = uuidStr

				clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, uuidStr)

				return nil
			},
		},
	)
}
