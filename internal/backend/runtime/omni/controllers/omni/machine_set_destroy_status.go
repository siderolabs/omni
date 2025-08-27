// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"

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
				return omni.NewMachineSetDestroyStatus(resources.EphemeralNamespace, machineSet.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineSetDestroyStatus *omni.MachineSetDestroyStatus) *omni.MachineSet {
				return omni.NewMachineSet(resources.DefaultNamespace, machineSetDestroyStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, machineSet *omni.MachineSet, machineSetDestroyStatus *omni.MachineSetDestroyStatus) error {
				cmStatuses, err := r.List(ctx, omni.NewClusterMachineStatus(resources.DefaultNamespace, "").Metadata(),
					state.WithLabelQuery(resource.LabelEqual(
						omni.LabelMachineSet, machineSet.Metadata().ID()),
					),
				)
				if err != nil {
					return err
				}

				remainingMachines := 0
				for _, cmStatus := range cmStatuses.Items {
					switch cmStatus.Metadata().Phase() {
					case resource.PhaseRunning:
						if !cmStatus.Metadata().Finalizers().Has(MachineSetDestroyStatusControllerName) {
							if err = r.AddFinalizer(ctx, cmStatus.Metadata(), MachineSetDestroyStatusControllerName); err != nil {
								return err
							}
						}
						remainingMachines++
					case resource.PhaseTearingDown:
						if cmStatus.Metadata().Finalizers().Has(MachineSetDestroyStatusControllerName) {
							if hasOnlyDestroyStatusFinalizers(cmStatus.Metadata()) {
								if err = r.RemoveFinalizer(ctx, cmStatus.Metadata(), MachineSetDestroyStatusControllerName); err != nil {
									return err
								}

								continue
							}
							remainingMachines++
						}
					}
				}

				if machineSet.Metadata().Phase() != resource.PhaseTearingDown {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("not tearing down")
				}

				machineSetDestroyStatus.TypedSpec().Value.Phase = fmt.Sprintf("Destroying: %s", pluralize.NewClient().Pluralize("machine", remainingMachines, true))

				return nil
			},
		},
		qtransform.WithExtraMappedInput(mappers.MapByMachineSetLabel[*omni.ClusterMachineStatus, *omni.MachineSet]()),
		qtransform.WithIgnoreTeardownUntil(),
	)
}

// hasOnlyDestroyStatusFinalizers reports if ClusterMachineStatus resource has only specified DestroyStatusControllers* as finalizer.
func hasOnlyDestroyStatusFinalizers(clusterMachineStatusMD *resource.Metadata) bool {
	destroyStatusControllers := []string{ClusterDestroyStatusControllerName, MachineSetDestroyStatusControllerName}

	for _, fin := range *clusterMachineStatusMD.Finalizers() {
		if !slices.Contains(destroyStatusControllers, fin) { // there is a finalizer that is not a destroy status controller
			return false
		}
	}

	return true
}
