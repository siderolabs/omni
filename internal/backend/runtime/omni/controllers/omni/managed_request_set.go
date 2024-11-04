// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ManagedRequestSetController creates omni.ClusterSecrets for each input omni.Cluster.
//
// ManagedRequestSetController generates and stores cluster wide secrets.
type ManagedRequestSetController = qtransform.QController[*omni.MachineSet, *omni.MachineRequestSet]

// ProviderConfig defines the infra provider configuration for the managed control planes.
type ProviderConfig struct {
	ID   string
	Data string
}

// NewManagedRequestSetController instantiates the talosconfig controller.
func NewManagedRequestSetController(defaultProvider ProviderConfig) *ManagedRequestSetController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineSet, *omni.MachineRequestSet]{
			Name: "ManagedRequestSetController",
			MapMetadataOptionalFunc: func(machineSet *omni.MachineSet) optional.Optional[*omni.MachineRequestSet] {
				if machineSet.Metadata().Owner() != managedControlPlaneControllerName {
					return optional.None[*omni.MachineRequestSet]()
				}

				return optional.Some(omni.NewMachineRequestSet(resources.DefaultNamespace, machineSet.Metadata().ID()))
			},
			UnmapMetadataFunc: func(machineRequestSet *omni.MachineRequestSet) *omni.MachineSet {
				return omni.NewMachineSet(resources.DefaultNamespace, machineRequestSet.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, machineSet *omni.MachineSet, machineRequestSet *omni.MachineRequestSet) error {
				clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					return errors.New("cluster name label is missing from the machine set")
				}

				cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
				if err != nil {
					return err
				}

				machineRequestSet.TypedSpec().Value.MachineCount = 3
				machineRequestSet.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
				machineRequestSet.TypedSpec().Value.ProviderId = defaultProvider.ID
				machineRequestSet.TypedSpec().Value.ProviderData = defaultProvider.Data

				machineRequestSet.Metadata().Labels().Set(omni.LabelNoManualAllocation, "")

				return nil
			},
		},
		qtransform.WithOutputKind(controller.OutputShared),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.Cluster, *omni.MachineSet](),
		),
	)
}
