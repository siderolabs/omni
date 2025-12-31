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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterDestroyStatusController manages ClusterDestroyStatus resource.
//
// ClusterDestroyStatusController aggregates the cluster state based on the cluster machines states.
type ClusterDestroyStatusController = qtransform.QController[*omni.Cluster, *omni.ClusterDestroyStatus]

// ClusterDestroyStatusControllerName is the name of the ClusterDestroyStatusController.
const ClusterDestroyStatusControllerName = "ClusterDestroyStatusController"

// NewClusterDestroyStatusController initializes ClusterDestroyStatusController.
//
//nolint:gocognit,gocyclo,cyclop
func NewClusterDestroyStatusController() *ClusterDestroyStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterDestroyStatus]{
			Name: ClusterDestroyStatusControllerName,
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterDestroyStatus {
				return omni.NewClusterDestroyStatus(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterDestroyStatus *omni.ClusterDestroyStatus) *omni.Cluster {
				return omni.NewCluster(clusterDestroyStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, cluster *omni.Cluster, clusterDestroyStatus *omni.ClusterDestroyStatus) error {
				if cluster.Metadata().Phase() != resource.PhaseTearingDown {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("not tearing down")
				}

				msStatuses, err := r.List(ctx, omni.NewMachineSetStatus("").Metadata(), state.WithLabelQuery(
					resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
				))
				if err != nil {
					return fmt.Errorf("failed to list control planes %w", err)
				}

				cmStatuses, err := r.List(ctx, omni.NewClusterMachineStatus("").Metadata(),
					state.WithLabelQuery(resource.LabelEqual(
						omni.LabelCluster, cluster.Metadata().ID()),
					),
				)
				if err != nil {
					return err
				}

				clusterDestroyStatus.TypedSpec().Value.Phase = fmt.Sprintf("Destroying: %s, %s",
					pluralize.NewClient().Pluralize("machine set", len(msStatuses.Items), true),
					pluralize.NewClient().Pluralize("machine", len(cmStatuses.Items), true),
				)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.MachineSetStatus](mappers.MapByClusterLabel[*omni.Cluster]()),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](mappers.MapByClusterLabel[*omni.Cluster]()),
		qtransform.WithIgnoreTeardownUntil(ClusterStatusControllerName),
	)
}
