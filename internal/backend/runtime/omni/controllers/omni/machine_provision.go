// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// MachineProvisionController turns MachineProvision resources into MachineRequestSets, scales them automatically on demand.
type MachineProvisionController = qtransform.QController[*omni.MachineSet, *omni.MachineRequestSet]

// MachineProvisionControllerName is the name of the MachineProvisionController.
const MachineProvisionControllerName = "MachineProvisionController"

// NewMachineProvisionController instantiates the machine controller.
//
//nolint:gocognit
func NewMachineProvisionController() *MachineProvisionController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineSet, *omni.MachineRequestSet]{
			Name: MachineProvisionControllerName,
			MapMetadataFunc: func(res *omni.MachineSet) *omni.MachineRequestSet {
				return omni.NewMachineRequestSet(res.Metadata().ID())
			},
			UnmapMetadataFunc: func(res *omni.MachineRequestSet) *omni.MachineSet {
				return omni.NewMachineSet(res.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineSet *omni.MachineSet, machineRequestSet *omni.MachineRequestSet) error {
				machineAllocation := omni.GetMachineAllocation(machineSet)
				if machineAllocation == nil {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("machine set doesn't use automatic machine allocation")
				}

				clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					return fmt.Errorf("machine set doesn't have cluster label")
				}

				cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
				if err != nil {
					return err
				}

				machineClass, err := safe.ReaderGetByID[*omni.MachineClass](ctx, r, machineAllocation.Name)
				if err != nil {
					return err
				}

				provision := machineClass.TypedSpec().Value.AutoProvision
				if provision == nil {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("autoprovision is disabled for the machine class")
				}

				machineRequestSet.TypedSpec().Value.ProviderId = provision.ProviderId
				machineRequestSet.TypedSpec().Value.KernelArgs = provision.KernelArgs
				machineRequestSet.TypedSpec().Value.MetaValues = provision.MetaValues
				machineRequestSet.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion
				machineRequestSet.TypedSpec().Value.ProviderData = provision.ProviderData
				machineRequestSet.TypedSpec().Value.GrpcTunnel = provision.GrpcTunnel

				expectMachines := machineAllocation.MachineCount

				machineSetStatus, err := safe.ReaderGetByID[*omni.MachineSetStatus](ctx, r, machineSet.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if machineSetStatus != nil && machineSetStatus.TypedSpec().Value.Machines != nil && machineSetStatus.TypedSpec().Value.Machines.Total > expectMachines {
					expectMachines = machineSetStatus.TypedSpec().Value.Machines.Total
				}

				delta := expectMachines - uint32(machineRequestSet.TypedSpec().Value.MachineCount)

				if delta == 0 {
					return nil
				}

				if delta > 0 {
					logger.Info("scale up", zap.Uint32("count", delta))
				} else {
					logger.Info("scale down", zap.Uint32("count", -delta))
				}

				machineRequestSet.TypedSpec().Value.MachineCount = int32(expectMachines)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.Cluster](
			mappers.MapByClusterLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetStatus](
			qtransform.MapperSameID[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineClass](
			qtransform.MapperFuncFromTyped[*omni.MachineClass](
				func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, res *omni.MachineClass) ([]resource.Pointer, error) {
					if res.TypedSpec().Value.AutoProvision == nil {
						return nil, nil
					}

					machineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r)
					if err != nil {
						return nil, err
					}

					resources := make([]resource.Pointer, 0, machineSets.Len())

					for machineSet := range machineSets.All() {
						allocation := omni.GetMachineAllocation(machineSet)

						if allocation == nil || allocation.Name != res.Metadata().ID() {
							continue
						}

						resources = append(resources, machineSet.Metadata())
					}

					return resources, nil
				},
			),
		),
		qtransform.WithOutputKind(controller.OutputShared),
		qtransform.WithConcurrency(4),
	)
}
