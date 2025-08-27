// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"log"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
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
				return omni.NewClusterDestroyStatus(resources.DefaultNamespace, cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterDestroyStatus *omni.ClusterDestroyStatus) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, clusterDestroyStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, cluster *omni.Cluster, clusterDestroyStatus *omni.ClusterDestroyStatus) error {
				remainingMachines := 0

				msStatuses, err := r.List(ctx, omni.NewMachineSetStatus(resources.DefaultNamespace, "").Metadata(), state.WithLabelQuery(
					resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
				))
				if err != nil {
					return fmt.Errorf("failed to list control planes %w", err)
				}

				remainingMachineSetIDs := make(map[resource.ID]struct{}, len(msStatuses.Items))
				for _, status := range msStatuses.Items {
					switch status.Metadata().Phase() {
					case resource.PhaseRunning:
						if !status.Metadata().Finalizers().Has(ClusterDestroyStatusControllerName) {
							if err = r.AddFinalizer(ctx, status.Metadata(), ClusterDestroyStatusControllerName); err != nil {
								return err
							}
						}
						remainingMachineSetIDs[status.Metadata().ID()] = struct{}{}
					case resource.PhaseTearingDown:
						if status.Metadata().Finalizers().Has(ClusterDestroyStatusControllerName) {
							if len(*status.Metadata().Finalizers()) == 1 {
								log.Printf("Removing finalizer for cluster %s", status.Metadata().ID())
								if err = r.RemoveFinalizer(ctx, status.Metadata(), ClusterDestroyStatusControllerName); err != nil {
									return err
								}

								continue
							}
							remainingMachineSetIDs[status.Metadata().ID()] = struct{}{}
						}
					}
				}

				cmStatuses, err := r.List(ctx, omni.NewClusterMachineStatus(resources.DefaultNamespace, "").Metadata(),
					state.WithLabelQuery(resource.LabelEqual(
						omni.LabelCluster, cluster.Metadata().ID()),
					),
				)
				if err != nil {
					return err
				}

				incrementRemainingMachines := func(cmStatus resource.Resource) {
					if msId, ok := cmStatus.Metadata().Labels().Get(omni.LabelMachineSet); ok {
						if _, ok = remainingMachineSetIDs[msId]; ok {
							remainingMachines++
						}
					}
				}

				for _, cmStatus := range cmStatuses.Items {
					switch cmStatus.Metadata().Phase() {
					case resource.PhaseRunning:
						if !cmStatus.Metadata().Finalizers().Has(ClusterDestroyStatusControllerName) {
							if err = r.AddFinalizer(ctx, cmStatus.Metadata(), ClusterDestroyStatusControllerName); err != nil {
								return err
							}
						}
						incrementRemainingMachines(cmStatus)
					case resource.PhaseTearingDown:
						if cmStatus.Metadata().Finalizers().Has(ClusterDestroyStatusControllerName) {
							if hasOnlyDestroyStatusFinalizers(cmStatus.Metadata()) {
								if err = r.RemoveFinalizer(ctx, cmStatus.Metadata(), ClusterDestroyStatusControllerName); err != nil {
									return err
								}

								continue
							}
							incrementRemainingMachines(cmStatus)
						}
					}
				}

				if cluster.Metadata().Phase() != resource.PhaseTearingDown {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("not tearing down")
				}

				clusterDestroyStatus.TypedSpec().Value.Phase = fmt.Sprintf("Destroying: %s, %s",
					pluralize.NewClient().Pluralize("machine set", len(remainingMachineSetIDs), true),
					pluralize.NewClient().Pluralize("machine", remainingMachines, true),
				)

				return nil
			},
		},
		qtransform.WithExtraMappedInput(mappers.MapByClusterLabel[*omni.MachineSetStatus, *omni.Cluster]()),
		qtransform.WithExtraMappedInput(mappers.MapByClusterLabel[*omni.ClusterMachineStatus, *omni.Cluster]()),
		qtransform.WithIgnoreTeardownUntil(ClusterStatusControllerName),
	)
}
