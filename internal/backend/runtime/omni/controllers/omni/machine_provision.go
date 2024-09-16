// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineProvisionController turns MachineProvision resources into a MachineRequestSets, scales them automatically on demand.
type MachineProvisionController = qtransform.QController[*omni.MachineClass, *omni.MachineRequestSet]

const machineProvisionControllerName = "MachineProvisionController"

// NewMachineProvisionController instanciates the machine controller.
func NewMachineProvisionController() *MachineProvisionController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineClass, *omni.MachineRequestSet]{
			Name: machineProvisionControllerName,
			MapMetadataFunc: func(res *omni.MachineClass) *omni.MachineRequestSet {
				return omni.NewMachineRequestSet(resources.DefaultNamespace, res.Metadata().ID())
			},
			UnmapMetadataFunc: func(res *omni.MachineRequestSet) *omni.MachineClass {
				return omni.NewMachineClass(resources.DefaultNamespace, res.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineClass *omni.MachineClass, machineRequestSet *omni.MachineRequestSet) error {
				provision := machineClass.TypedSpec().Value.AutoProvision
				if provision == nil {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("autoprovision is disabled")
				}

				machineRequestSet.TypedSpec().Value.ProviderId = provision.ProviderId
				machineRequestSet.TypedSpec().Value.Extensions = provision.Extensions
				machineRequestSet.TypedSpec().Value.KernelArgs = provision.KernelArgs
				machineRequestSet.TypedSpec().Value.MetaValues = provision.MetaValues
				machineRequestSet.TypedSpec().Value.TalosVersion = provision.TalosVersion

				pressure, err := safe.ReaderGetByID[*omni.MachineRequestSetPressure](ctx, r, machineClass.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				expectMachines := provision.IdleMachineCount
				if pressure != nil {
					expectMachines += pressure.TypedSpec().Value.RequiredMachines
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
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineRequestSetPressure, *omni.MachineClass](),
		),
		qtransform.WithConcurrency(4),
	)
}
