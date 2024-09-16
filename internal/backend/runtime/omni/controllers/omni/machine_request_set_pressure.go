// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const machineRequestSetPressureControllerName = "MachineRequestSetPressureController"

// MachineRequestSetPressureController manages MachineRequestSetPressure resource lifecycle.
//
// MachineRequestSetPressureController calculates requested machines for each machine request set.
type MachineRequestSetPressureController = qtransform.QController[*omni.MachineRequestSet, *omni.MachineRequestSetPressure]

// NewMachineRequestSetPressureController initializes MachineRequestSetPressureController.
//
//nolint:gocognit
func NewMachineRequestSetPressureController() *MachineRequestSetPressureController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineRequestSet, *omni.MachineRequestSetPressure]{
			Name: machineRequestSetPressureControllerName,
			MapMetadataFunc: func(res *omni.MachineRequestSet) *omni.MachineRequestSetPressure {
				return omni.NewMachineRequestSetPressure(res.Metadata().Namespace(), res.Metadata().ID())
			},
			UnmapMetadataFunc: func(res *omni.MachineRequestSetPressure) *omni.MachineRequestSet {
				return omni.NewMachineRequestSet(res.Metadata().Namespace(), res.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, mrs *omni.MachineRequestSet, mrsp *omni.MachineRequestSetPressure) error {
				// calculate pressure only for the machine request sets which are managed by the automated scaling
				if mrs.Metadata().Owner() != machineProvisionControllerName {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("the machine request set is not managed by the autoscaling")
				}

				mssList, err := safe.ReaderListAll[*omni.MachineSetStatus](ctx, r)
				if err != nil {
					return err
				}

				total := uint32(0)

				err = mssList.ForEachErr(func(mss *omni.MachineSetStatus) error {
					if mss.Metadata().Phase() == resource.PhaseTearingDown || mrs.Metadata().Phase() == resource.PhaseTearingDown {
						return r.RemoveFinalizer(ctx, mss.Metadata(), machineRequestSetPressureControllerName)
					}

					if mss.TypedSpec().Value.MachineAllocation == nil {
						return nil
					}

					if mss.TypedSpec().Value.MachineAllocation.Name != mrs.Metadata().ID() {
						return nil
					}

					value := mss.TypedSpec().Value.MachineAllocation.MachineCount

					if mss.TypedSpec().Value.Machines.Total > mss.TypedSpec().Value.MachineAllocation.MachineCount {
						value = mss.TypedSpec().Value.Machines.Total
					}

					total += value

					if !mss.Metadata().Finalizers().Has(machineRequestSetPressureControllerName) {
						return r.AddFinalizer(ctx, mss.Metadata(), machineRequestSetPressureControllerName)
					}

					return nil
				})
				if err != nil {
					return err
				}

				mrsp.TypedSpec().Value.RequiredMachines = total

				return nil
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, mrs *omni.MachineRequestSet) error {
				if mrs.Metadata().Owner() != machineProvisionControllerName {
					return nil
				}

				mssList, err := safe.ReaderListAll[*omni.MachineSetStatus](ctx, r)
				if err != nil {
					return err
				}

				return mssList.ForEachErr(func(mss *omni.MachineSetStatus) error {
					allocation := mss.TypedSpec().Value.MachineAllocation
					if allocation == nil || allocation.Name != mrs.Metadata().ID() {
						return nil
					}

					return r.RemoveFinalizer(ctx, mss.Metadata(), machineRequestSetPressureControllerName)
				})
			},
		},
		qtransform.WithExtraMappedInput(
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, machineSetStatus *omni.MachineSetStatus) ([]resource.Pointer, error) {
				allocationConfig := machineSetStatus.TypedSpec().Value.MachineAllocation

				if allocationConfig == nil {
					return nil, nil
				}

				return []resource.Pointer{
					omni.NewMachineRequestSet(resources.DefaultNamespace, allocationConfig.Name).Metadata(),
				}, nil
			},
		),
	)
}
