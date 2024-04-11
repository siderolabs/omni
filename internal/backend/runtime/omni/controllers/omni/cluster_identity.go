// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterIdentityController creates the system config patch that contains the maintenance config.
type ClusterIdentityController = qtransform.QController[*omni.ClusterSecrets, *omni.ClusterIdentity]

// NewClusterIdentityController initializes ClusterIdentityController.
func NewClusterIdentityController() *ClusterIdentityController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterSecrets, *omni.ClusterIdentity]{
			Name: "ClusterIdentityController",
			MapMetadataFunc: func(clusterSecrets *omni.ClusterSecrets) *omni.ClusterIdentity {
				return omni.NewClusterIdentity(resources.DefaultNamespace, clusterSecrets.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterIdentity *omni.ClusterIdentity) *omni.ClusterSecrets {
				return omni.NewClusterSecrets(resources.DefaultNamespace, clusterIdentity.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, clusterSecrets *omni.ClusterSecrets, clusterIdentity *omni.ClusterIdentity) error {
				bundle, err := omni.ToSecretsBundle(clusterSecrets)
				if err != nil {
					return fmt.Errorf("failed to convert cluster %q secrets to bundle: %w", clusterSecrets.Metadata().ID(), err)
				}

				clusterIdentity.TypedSpec().Value.ClusterId = bundle.Cluster.ID

				clusterMachineIdentityList, err := safe.ReaderListAll[*omni.ClusterMachineIdentity](ctx, r)
				if err != nil {
					return fmt.Errorf("error listing cluster machine identities: %w", err)
				}

				nodeIDs := make([]string, 0, clusterMachineIdentityList.Len())

				for iter := clusterMachineIdentityList.Iterator(); iter.Next(); {
					clusterMachineIdentity := iter.Value()
					nodeIdentity := clusterMachineIdentity.TypedSpec().Value.NodeIdentity

					if nodeIdentity != "" {
						nodeIDs = append(nodeIDs, nodeIdentity)
					}
				}

				slices.Sort(nodeIDs)

				clusterIdentity.TypedSpec().Value.NodeIds = nodeIDs

				return nil
			},
		},
		qtransform.WithConcurrency(2),
		qtransform.WithExtraMappedInput(mappers.MapByClusterLabel[*omni.ClusterMachineIdentity, *omni.ClusterSecrets]()),
	)
}
