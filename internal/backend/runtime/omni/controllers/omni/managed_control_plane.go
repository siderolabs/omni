// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ManagedControlPlaneController creates omni.ClusterSecrets for each input omni.Cluster.
//
// ManagedControlPlaneController generates and stores cluster wide secrets.
type ManagedControlPlaneController = qtransform.QController[*omni.Cluster, *omni.MachineSet]

const managedControlPlaneControllerName = "ManagedControlPlaneController"

// NewManagedControlPlaneController instantiates the talosconfig controller.
func NewManagedControlPlaneController() *ManagedControlPlaneController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.MachineSet]{
			Name: managedControlPlaneControllerName,
			MapMetadataOptionalFunc: func(cluster *omni.Cluster) optional.Optional[*omni.MachineSet] {
				if !omni.GetManagedEnabled(cluster) {
					return optional.None[*omni.MachineSet]()
				}

				return optional.Some(omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(cluster.Metadata().ID())))
			},
			UnmapMetadataFunc: func(machineSet *omni.MachineSet) *omni.Cluster {
				cluster, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					panic("missing cluster label on the machine set")
				}

				return omni.NewCluster(resources.DefaultNamespace, cluster)
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, cluster *omni.Cluster, machineSet *omni.MachineSet) error {
				machineSet.Metadata().Labels().Set(omni.LabelManaged, "")
				machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
				machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

				return nil
			},
		},
		qtransform.WithOutputKind(controller.OutputShared),
	)
}
