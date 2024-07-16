// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
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
	mapMachineIDToMachineSet := func(ctx context.Context, r controller.QRuntime, res resource.Resource, label string) ([]resource.Pointer, error) {
		id, ok := res.Metadata().Labels().Get(label)
		if !ok {
			return nil, nil
		}

		input, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, id)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, nil
			}
		}

		id, ok = input.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			return nil, nil
		}

		return []resource.Pointer{
			omni.NewMachineSet(resources.DefaultNamespace, id).Metadata(),
		}, nil
	}

	handler := &machineSetStatusHandler{}

	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineSet, *omni.MachineSetStatus]{
			Name: machineset.ControllerName,
			MapMetadataFunc: func(machineSet *omni.MachineSet) *omni.MachineSetStatus {
				return omni.NewMachineSetStatus(resources.DefaultNamespace, machineSet.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineSetStatus *omni.MachineSetStatus) *omni.MachineSet {
				return omni.NewMachineSet(resources.DefaultNamespace, machineSetStatus.Metadata().ID())
			},
			TransformExtraOutputFunc:        handler.reconcileRunning,
			FinalizerRemovalExtraOutputFunc: handler.reconcileTearingDown,
		},
		qtransform.WithConcurrency(8),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ControlPlaneStatus, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByMachineSetLabel[*omni.MachineSetNode, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByMachineSetLabel[*omni.ClusterMachineStatus, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedDestroyReadyInput(
			mappers.MapByMachineSetLabel[*omni.ClusterMachine, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByMachineSetLabel[*omni.ClusterMachineConfigStatus, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterSecrets, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.Cluster, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.TalosConfig, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByMachineSetLabel[*omni.ClusterMachineIdentity, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.LoadBalancerStatus, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			// machine to machine set, if the machine is allocated
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, machine *omni.Machine) ([]resource.Pointer, error) {
				clusterMachine, err := r.Get(ctx, omni.NewClusterMachine(resources.DefaultNamespace, machine.Metadata().ID()).Metadata())
				if err != nil {
					if state.IsNotFoundError(err) {
						return nil, nil
					}
				}

				machineSetID, ok := clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
				if !ok {
					return nil, nil
				}

				return []resource.Pointer{
					omni.NewMachineSet(resources.DefaultNamespace, machineSetID).Metadata(),
				}, nil
			},
		),
		qtransform.WithExtraMappedInput(
			// config patch to machine set if the machine is allocated, checks by different layers, if is on the cluster layer,
			// matches all machine sets
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, patch *omni.ConfigPatch) ([]resource.Pointer, error) {
				clusterName, ok := patch.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					// no cluster, map by the machine ID
					return mapMachineIDToMachineSet(ctx, r, patch, omni.LabelMachine)
				}

				// cluster machine patch
				pointers, err := mapMachineIDToMachineSet(ctx, r, patch, omni.LabelClusterMachine)
				if err != nil {
					return nil, err
				}

				if pointers != nil {
					return pointers, err
				}

				// machine set level patch
				machineSetID, ok := patch.Metadata().Labels().Get(omni.LabelMachineSet)
				if ok {
					return []resource.Pointer{
						omni.NewMachineSet(resources.DefaultNamespace, machineSetID).Metadata(),
					}, nil
				}

				// cluster level patch, find all machine sets in a cluster
				list, err := r.List(ctx, omni.NewMachineSet(resources.DefaultNamespace, "").Metadata(), state.WithLabelQuery(
					resource.LabelEqual(omni.LabelCluster, clusterName),
				))
				if err != nil {
					return nil, err
				}

				return xslices.Map(list.Items, func(r resource.Resource) resource.Pointer { return r.Metadata() }), nil
			},
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.ClusterMachineType,
				Kind: controller.OutputExclusive,
			},
			controller.Output{
				Type: omni.ClusterMachineConfigPatchesType,
				Kind: controller.OutputExclusive,
			},
		),
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

	// should run always
	machineset.ReconcileStatus(rc, machineSetStatus)

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

	clusterMachinesCount := uint32(len(rc.GetClusterMachines()))
	// no cluster machines release the finalizer
	if clusterMachinesCount == 0 && len(rc.GetMachineSetNodes()) == 0 {
		logger.Info("machineset torn down", zap.String("machineset", machineSet.Metadata().ID()))

		return nil
	}

	err = safe.WriterModify(ctx, r, omni.NewMachineSetStatus(resources.DefaultNamespace, machineSet.Metadata().ID()), func(status *omni.MachineSetStatus) error {
		status.TypedSpec().Value.Phase = specs.MachineSetPhase_Destroying
		status.TypedSpec().Value.Ready = false
		status.TypedSpec().Value.Machines = &specs.Machines{
			Total:   clusterMachinesCount,
			Healthy: 0,
		}

		return nil
	})
	if err != nil {
		return err
	}

	if _, err := handler.reconcileMachines(ctx, r, logger, rc); err != nil {
		return err
	}

	// teardown complete, ignore requeue and unlock the finalizer now
	if len(rc.GetRunningClusterMachines()) == 0 {
		return nil
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
