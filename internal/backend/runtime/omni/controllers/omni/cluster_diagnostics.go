// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterDiagnosticsController manages ClusterDiagnostics resource lifecycle.
type ClusterDiagnosticsController = qtransform.QController[*omni.ClusterUUID, *omni.ClusterDiagnostics]

// NewClusterDiagnosticsController initializes ClusterDiagnosticsController.
func NewClusterDiagnosticsController() *ClusterDiagnosticsController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterUUID, *omni.ClusterDiagnostics]{
			Name: "ClusterDiagnosticsController",
			MapMetadataFunc: func(uuid *omni.ClusterUUID) *omni.ClusterDiagnostics {
				return omni.NewClusterDiagnostics(resources.DefaultNamespace, uuid.Metadata().ID())
			},
			UnmapMetadataFunc: func(diagnostics *omni.ClusterDiagnostics) *omni.ClusterUUID {
				return omni.NewClusterUUID(diagnostics.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, uuid *omni.ClusterUUID, diagnostics *omni.ClusterDiagnostics) error {
				machineStatusList, err := safe.ReaderListAll[*omni.MachineStatus](
					ctx, r,
					state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, uuid.Metadata().ID())),
				)
				if err != nil {
					return err
				}

				var nodes []*specs.ClusterDiagnosticsSpec_Node

				for machineStatus := range machineStatusList.All() {
					numDiagnostics := len(machineStatus.TypedSpec().Value.Diagnostics)

					if numDiagnostics > 0 {
						nodes = append(nodes, &specs.ClusterDiagnosticsSpec_Node{
							Id:             machineStatus.Metadata().ID(),
							NumDiagnostics: uint32(numDiagnostics),
						})
					}
				}

				if len(nodes) == 0 {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("no diagnostic information available on this cluster")
				}

				slices.SortFunc(nodes, func(a, b *specs.ClusterDiagnosticsSpec_Node) int {
					return strings.Compare(a.Id, b.Id)
				})

				diagnostics.TypedSpec().Value.Nodes = nodes

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.MachineStatus, *omni.ClusterUUID](),
		),
	)
}
