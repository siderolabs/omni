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
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// MachineClassStatusController manages MachineClassStatus resource lifecycle.
//
// MachineClassStatusController generates cluster UUID for every cluster.
type MachineClassStatusController = qtransform.QController[*omni.MachineClass, *omni.MachineClassStatus]

// NewMachineClassStatusController initializes MachineClassStatusController.
func NewMachineClassStatusController() *MachineClassStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineClass, *omni.MachineClassStatus]{
			Name: "MachineClassStatusController",
			MapMetadataFunc: func(mc *omni.MachineClass) *omni.MachineClassStatus {
				return omni.NewMachineClassStatus(mc.Metadata().Namespace(), mc.Metadata().ID())
			},
			UnmapMetadataFunc: func(mcs *omni.MachineClassStatus) *omni.MachineClass {
				return omni.NewMachineClass(mcs.Metadata().Namespace(), mcs.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, mc *omni.MachineClass, mcs *omni.MachineClassStatus) error {
				msrmList, err := safe.ReaderListAll[*omni.MachineSetRequiredMachines](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineClassName, mc.Metadata().ID())))
				if err != nil {
					return err
				}

				total := uint32(0)

				msrmList.ForEach(func(msrm *omni.MachineSetRequiredMachines) {
					if msrm.Metadata().Phase() != resource.PhaseTearingDown {
						total += msrm.TypedSpec().Value.RequiredAdditionalMachines
					}
				})

				mcs.TypedSpec().Value.RequiredAdditionalMachines = total

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			mappers.MapByMachineClassNameLabel[*omni.MachineSetRequiredMachines, *omni.MachineClass](),
		),
	)
}
