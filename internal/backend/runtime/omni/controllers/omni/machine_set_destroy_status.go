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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// MachineSetDestroyStatusController manages MachineSetDestroyStatus resource.
//
// MachineSetDestroyStatusController aggregates the machineset state based on the cluster machines states.
type MachineSetDestroyStatusController = qtransform.QController[*omni.MachineSet, *omni.MachineSetDestroyStatus]

// MachineSetDestroyStatusControllerName is the name of the MachineSetDestroyStatusController.
const MachineSetDestroyStatusControllerName = "MachineSetDestroyStatusController"

// NewMachineSetDestroyStatusController initializes MachineSetDestroyStatusController.
//
//nolint:gocognit
func NewMachineSetDestroyStatusController() *MachineSetDestroyStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineSet, *omni.MachineSetDestroyStatus]{
			Name: MachineSetDestroyStatusControllerName,
			MapMetadataFunc: func(machineSet *omni.MachineSet) *omni.MachineSetDestroyStatus {
				return omni.NewMachineSetDestroyStatus(machineSet.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineSetDestroyStatus *omni.MachineSetDestroyStatus) *omni.MachineSet {
				return omni.NewMachineSet(machineSetDestroyStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, machineSet *omni.MachineSet, machineSetDestroyStatus *omni.MachineSetDestroyStatus) error {
				if machineSet.Metadata().Phase() != resource.PhaseTearingDown {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("not tearing down")
				}

				cmStatuses, err := r.List(ctx, omni.NewClusterMachineStatus("").Metadata(),
					state.WithLabelQuery(resource.LabelEqual(
						omni.LabelMachineSet, machineSet.Metadata().ID()),
					),
				)
				if err != nil {
					return err
				}

				machineSetDestroyStatus.TypedSpec().Value.Phase = fmt.Sprintf("Destroying: %s", pluralize.NewClient().Pluralize("machine", len(cmStatuses.Items), true))

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](mappers.MapByMachineSetLabel[*omni.MachineSet]()),
		qtransform.WithIgnoreTeardownUntil(),
	)
}
