// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// MachineSetStatusController manages MachineSetStatus resource lifecycle.
//
// MachineSetStatusController creates and deletes cluster machines, handles rolling updates.
type MachineSetStatusController = qtransform.QController[*omni.MachineSet, *omni.MachineSetStatus]

const requeueInterval = time.Second * 30

// NewMachineSetStatusController creates new MachineSetStatusController.
func NewMachineSetStatusController() *MachineSetStatusController {
	handler := &machineSetStatusHandler{}

	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineSet, *omni.MachineSetStatus]{
			Name: machineset.ControllerName,
			MapMetadataFunc: func(machineSet *omni.MachineSet) *omni.MachineSetStatus {
				return omni.NewMachineSetStatus(machineSet.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineSetStatus *omni.MachineSetStatus) *omni.MachineSet {
				return omni.NewMachineSet(machineSetStatus.Metadata().ID())
			},
			TransformExtraOutputFunc:        handler.reconcileRunning,
			FinalizerRemovalExtraOutputFunc: handler.reconcileTearingDown,
		},
		qtransform.WithConcurrency(16),
		qtransform.WithExtraMappedInput[*omni.ControlPlaneStatus](
			qtransform.MapperSameID[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedDestroyReadyInput[*omni.ClusterMachine](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfigStatus](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.MachinePendingUpdates](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.Cluster](
			mappers.MapClusterResourceToLabeledResources[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.TalosConfig](
			mappers.MapClusterResourceToLabeledResources[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineIdentity](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.LoadBalancerStatus](
			mappers.MapClusterResourceToLabeledResources[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*machineStatusLabels](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.Machine](
			// machine to machine set, if the machine is allocated
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, machine controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				machineStatus, err := safe.ReaderGetByID[*machineStatusLabels](ctx, r, machine.ID())
				if err != nil {
					if state.IsNotFoundError(err) {
						return nil, nil
					}
				}

				machineSetID, ok := machineStatus.Metadata().Labels().Get(omni.LabelMachineSet)
				if !ok {
					return nil, nil
				}

				return []resource.Pointer{
					omni.NewMachineSet(machineSetID).Metadata(),
				}, nil
			},
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.ClusterMachineType,
				Kind: controller.OutputExclusive,
			},
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.MachineSetConfigStatusType,
				Kind: controller.OutputExclusive,
			},
		),
		qtransform.WithConcurrency(8),
	)
}

type machineSetStatusHandler struct{}

func (handler *machineSetStatusHandler) reconcileRunning(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	machineSet *omni.MachineSet, machineSetStatus *omni.MachineSetStatus,
) error {
	clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	if ok {
		logger = logger.With(zap.String("cluster", clusterName))
	}

	logger = logger.With(zap.String("machineset", machineSet.Metadata().ID()))

	rc, err := machineset.BuildReconciliationContext(ctx, r, machineSet)
	if err != nil {
		return err
	}

	if err = safe.WriterModify(ctx, r, omni.NewMachineSetConfigStatus(machineSet.Metadata().ID()),
		func(machineSetConfigStatus *omni.MachineSetConfigStatus) error {
			// should run always
			machineset.ReconcileStatus(rc, machineSetStatus, machineSetConfigStatus)

			return nil
		},
	); err != nil {
		return err
	}

	requeue, err := handler.reconcileMachines(ctx, r, logger, rc)
	if err != nil {
		return err
	}

	if requeue {
		return controller.NewRequeueInterval(requeueInterval)
	}

	return nil
}

func (handler *machineSetStatusHandler) reconcileTearingDown(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, machineSet *omni.MachineSet) error {
	rc, err := machineset.BuildReconciliationContext(ctx, r, machineSet)
	if err != nil {
		return err
	}

	updateStatus := func(clusterMachinesCount uint32) error {
		mss := omni.NewMachineSetStatus(machineSet.Metadata().ID())
		notFoundErr := errors.New("not found")

		modifyErr := safe.WriterModify(ctx, r, mss, func(status *omni.MachineSetStatus) error {
			if status.Metadata().Version().Value() == 0 {
				return notFoundErr
			}

			status.TypedSpec().Value.Phase = specs.MachineSetPhase_Destroying
			status.TypedSpec().Value.Ready = false
			status.TypedSpec().Value.Machines = &specs.Machines{
				Total:   clusterMachinesCount,
				Healthy: 0,
			}

			return nil
		}, controller.WithExpectedPhaseAny())
		if modifyErr != nil && !errors.Is(modifyErr, notFoundErr) {
			return modifyErr
		}

		modifyErr = safe.WriterModify(ctx, r, omni.NewMachineSetConfigStatus(machineSet.Metadata().ID()),
			func(status *omni.MachineSetConfigStatus) error {
				status.TypedSpec().Value.ShouldResetGraceful = false

				return nil
			},
			controller.WithExpectedPhaseAny(),
		)
		if modifyErr != nil && !errors.Is(modifyErr, notFoundErr) {
			return modifyErr
		}

		return nil
	}

	clusterMachinesCount := uint32(len(rc.GetClusterMachines()))
	// no cluster machines release the finalizer
	if clusterMachinesCount == 0 && len(rc.GetMachineSetNodes()) == 0 {
		logger.Info("machineset torn down", zap.String("machineset", machineSet.Metadata().ID()))

		if err = updateStatus(0); err != nil {
			return err
		}

		return nil
	}

	if err = updateStatus(clusterMachinesCount); err != nil {
		return err
	}

	if _, err = handler.reconcileMachines(ctx, r, logger, rc); err != nil {
		return err
	}

	// teardown complete, ignore requeue and unlock the finalizer now
	if len(rc.GetRunningClusterMachines()) == 0 {
		if err = updateStatus(0); err != nil {
			return err
		}

		return nil
	}

	ready, err := helpers.TeardownAndDestroy(ctx, r, omni.NewMachineSetConfigStatus(machineSet.Metadata().ID()).Metadata())
	if err != nil {
		return err
	}

	if !ready {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine set config status is not destroyed yet")
	}

	return controller.NewRequeueErrorf(requeueInterval, "the machine set still has cluster machines")
}

func (handler *machineSetStatusHandler) reconcileMachines(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *machineset.ReconciliationContext) (bool, error) {
	if err := machineset.UpdateFinalizers(ctx, r, rc); err != nil {
		return false, err
	}

	// return requeue as separate flag and return requeue in the end of the function
	return machineset.ReconcileMachines(ctx, r, logger, rc)
}
