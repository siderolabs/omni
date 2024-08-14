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
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterKubernetesNodesController creates the ClusterKubernetesNodes resource by watching the ClusterMachineIdentity resources.
//
// Its primary input is ClusterUUID since it is a resource that does not get updated during the whole lifetime of a cluster.
// This is something we want here, as we are only interested in the extra input - the ClusterMachineIdentity resources.
type ClusterKubernetesNodesController = qtransform.QController[*omni.ClusterUUID, *omni.ClusterKubernetesNodes]

// NewClusterKubernetesNodesController initializes ClusterKubernetesNodesController.
func NewClusterKubernetesNodesController() *ClusterKubernetesNodesController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterUUID, *omni.ClusterKubernetesNodes]{
			Name: "ClusterKubernetesNodesController",
			MapMetadataFunc: func(clusterUUID *omni.ClusterUUID) *omni.ClusterKubernetesNodes {
				return omni.NewClusterKubernetesNodes(resources.DefaultNamespace, clusterUUID.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterKubernetesNodes *omni.ClusterKubernetesNodes) *omni.ClusterUUID {
				return omni.NewClusterUUID(clusterKubernetesNodes.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, clusterUUID *omni.ClusterUUID, clusterKubernetesNodes *omni.ClusterKubernetesNodes) error {
				identityList, err := safe.ReaderListAll[*omni.ClusterMachineIdentity](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterUUID.Metadata().ID())))
				if err != nil {
					return fmt.Errorf("error listing cluster machine identities: %w", err)
				}

				if identityList.Len() == 0 {
					clusterKubernetesNodes.TypedSpec().Value.Nodes = nil

					return nil
				}

				nodes := make([]string, 0, identityList.Len())

				for identity := range identityList.All() {
					if identity.Metadata().Phase() == resource.PhaseTearingDown {
						continue
					}

					nodes = append(nodes, identity.TypedSpec().Value.Nodename)
				}

				if len(nodes) == 0 {
					clusterKubernetesNodes.TypedSpec().Value.Nodes = nil

					return nil
				}

				slices.Sort(nodes)

				clusterKubernetesNodes.TypedSpec().Value.Nodes = nodes

				return nil
			},
		},
		qtransform.WithConcurrency(2),
		qtransform.WithExtraMappedInput(mappers.MapByClusterLabel[*omni.ClusterMachineIdentity, *omni.ClusterUUID]()),
	)
}
