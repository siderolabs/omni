// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

// MachineRequestSetStatusControllerName is the name of the MachineRequestSetStatusController.
const MachineRequestSetStatusControllerName = "MachineRequestSetStatusController"

// MachineRequestSetStatusController creates machine requests for the machine pools.
type MachineRequestSetStatusController = qtransform.QController[*omni.MachineRequestSet, *omni.MachineRequestSetStatus]

// NewMachineRequestSetStatusController instantiates the MachineRequestSetStatusController.
func NewMachineRequestSetStatusController() *MachineRequestSetStatusController {
	h := &machineRequestSetStatusHandler{}

	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineRequestSet, *omni.MachineRequestSetStatus]{
			Name: MachineRequestSetStatusControllerName,
			MapMetadataFunc: func(pool *omni.MachineRequestSet) *omni.MachineRequestSetStatus {
				return omni.NewMachineRequestSetStatus(pool.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MachineRequestSetStatus) *omni.MachineRequestSet {
				return omni.NewMachineRequestSet(status.Metadata().ID())
			},
			TransformExtraOutputFunc:        h.reconcileRunning,
			FinalizerRemovalExtraOutputFunc: h.reconcileTearingDown,
		},
		qtransform.WithExtraMappedDestroyReadyInput[*infra.MachineRequest](
			qtransform.MapExtractLabelValue[*omni.MachineRequestSet](omni.LabelMachineRequestSet),
		),
		qtransform.WithExtraMappedInput[*machineStatusLabels](
			mapMachineToMachineRequest,
		),
		qtransform.WithExtraMappedInput[*infra.Provider](
			func(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				machineRequestSets, err := safe.ReaderListAll[*omni.MachineRequestSet](ctx, r)
				if err != nil {
					return nil, err
				}

				ptrs := make([]resource.Pointer, 0, machineRequestSets.Len())
				for machineRequestSet := range machineRequestSets.All() {
					if machineRequestSet.TypedSpec().Value.ProviderId == ptr.ID() {
						ptrs = append(ptrs, machineRequestSet.Metadata())
					}
				}

				return ptrs, nil
			},
		),
		qtransform.WithExtraOutputs(controller.Output{
			Type: infra.MachineRequestType,
			Kind: controller.OutputShared,
		}),
		qtransform.WithConcurrency(16),
	)
}

type machineRequestSetStatusHandler struct{}

func (h *machineRequestSetStatusHandler) reconcileRunning(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, machineRequestSet *omni.MachineRequestSet,
	_ *omni.MachineRequestSetStatus,
) error {
	machineStatuses, err := safe.ReaderListAll[*machineStatusLabels](ctx, r, state.WithLabelQuery(resource.LabelExists(omni.LabelMachineRequest)))
	if err != nil {
		return err
	}

	machineRequests, err := safe.ReaderListAll[*infra.MachineRequest](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSet.Metadata().ID())))
	if err != nil {
		return err
	}

	requests := make([]*infra.MachineRequest, 0, machineRequests.Len())

	// delete tearing down requests
	// delete requests when machines are tearing down
	err = machineRequests.ForEachErr(func(request *infra.MachineRequest) error {
		var machine *machineStatusLabels

		list := machineStatuses.FilterLabelQuery(resource.LabelEqual(omni.LabelMachineRequest, request.Metadata().ID()))
		if list.Len() > 0 {
			machine = list.Get(0)

			request.Metadata().Labels().Set(omni.LabelMachine, machine.Metadata().ID())
		}

		if machine != nil {
			if machine.Metadata().Phase() == resource.PhaseTearingDown {
				if err = r.RemoveFinalizer(ctx, machine.Metadata(), MachineRequestSetStatusControllerName); err != nil {
					return err
				}

				logger.Info("delete machine request after the machine link is torn down", zap.String("request_id", request.Metadata().ID()), zap.String("machine", machine.Metadata().ID()))

				return deleteMachineRequest(ctx, r, request, machine)
			}
		}

		if request.Metadata().Phase() == resource.PhaseTearingDown {
			return deleteMachineRequest(ctx, r, request, machine)
		}

		requests = append(requests, request)

		if machine != nil && !machine.Metadata().Finalizers().Has(MachineRequestSetStatusControllerName) {
			return r.AddFinalizer(ctx, machine.Metadata(), MachineRequestSetStatusControllerName)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return h.reconcileRequests(ctx, r, machineRequestSet, requests, machineStatuses)
}

func (h *machineRequestSetStatusHandler) reconcileRequests(ctx context.Context, r controller.ReaderWriter, machineRequestSet *omni.MachineRequestSet,
	machineRequests []*infra.MachineRequest, machineStatusList safe.List[*machineStatusLabels],
) error {
	machineStatuses := toMap(machineStatusList)

	diff := int(machineRequestSet.TypedSpec().Value.MachineCount) - len(machineRequests)
	if diff < 0 {
		return scaleDown(ctx, r, machineRequests, machineStatuses, -diff)
	}

	return h.scaleUp(ctx, r, machineRequestSet, diff)
}

func (h *machineRequestSetStatusHandler) scaleUp(ctx context.Context, r controller.ReaderWriter, machineRequestSet *omni.MachineRequestSet, count int) error {
	for range count {
		for range 100 {
			alias := rand.String(6)

			if err := safe.WriterModify(ctx, r, infra.NewMachineRequest(machineRequestSet.Metadata().ID()+"-"+alias), func(request *infra.MachineRequest) error {
				var err error

				request.TypedSpec().Value.TalosVersion = machineRequestSet.TypedSpec().Value.TalosVersion

				request.TypedSpec().Value.Extensions = machineRequestSet.TypedSpec().Value.Extensions
				request.TypedSpec().Value.KernelArgs = machineRequestSet.TypedSpec().Value.KernelArgs
				request.TypedSpec().Value.MetaValues = machineRequestSet.TypedSpec().Value.MetaValues
				request.TypedSpec().Value.ProviderData = machineRequestSet.TypedSpec().Value.ProviderData
				request.TypedSpec().Value.GrpcTunnel = machineRequestSet.TypedSpec().Value.GrpcTunnel

				request.Metadata().Labels().Set(omni.LabelInfraProviderID, machineRequestSet.TypedSpec().Value.ProviderId)
				request.Metadata().Labels().Set(omni.LabelMachineRequestSet, machineRequestSet.Metadata().ID())

				return err
			}); err != nil {
				if state.IsConflictError(err) {
					continue
				}

				return err
			}

			break
		}
	}

	return nil
}

func scaleDown(ctx context.Context, r controller.ReaderWriter, machineRequests []*infra.MachineRequest, machineStatuses map[resource.ID]*machineStatusLabels, count int) error {
	inUse := make(map[resource.ID]struct{}, len(machineStatuses))
	isCp := make(map[resource.ID]struct{}, len(machineStatuses))

	for _, res := range machineRequests {
		machineID, ok := res.Metadata().Labels().Get(omni.LabelMachine)
		if !ok {
			continue
		}

		machine, ok := machineStatuses[machineID]
		if !ok {
			continue
		}

		_, ok = machine.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			continue
		}

		inUse[res.Metadata().ID()] = struct{}{}

		if _, ok = machine.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
			isCp[res.Metadata().ID()] = struct{}{}
		}
	}

	compareFlags := func(flags map[resource.ID]struct{}, a, b *infra.MachineRequest) int {
		_, aflag := flags[a.Metadata().ID()]
		_, bflag := flags[b.Metadata().ID()]

		if aflag && !bflag {
			return 1
		}

		if bflag && !aflag {
			return -1
		}

		return 0
	}

	// sort by in use first, then if both are in use compare by the role, control planes should go last
	// the last check is by the creation time
	slices.SortFunc(machineRequests, func(a, b *infra.MachineRequest) int {
		if val := compareFlags(inUse, a, b); val != 0 {
			return val
		}

		if val := compareFlags(isCp, a, b); val != 0 {
			return val
		}

		return a.Metadata().Created().Compare(b.Metadata().Created())
	})

	for i, request := range machineRequests {
		if i >= count {
			return nil
		}

		var machine *machineStatusLabels

		machineID, ok := request.Metadata().Labels().Get(omni.LabelMachine)
		if ok {
			machine = system.NewResourceLabels[*omni.MachineStatus](machineID)
		}

		if err := deleteMachineRequest(ctx, r, request, machine); err != nil {
			return err
		}
	}

	return nil
}

func deleteMachineRequest(ctx context.Context, r controller.ReaderWriter, request *infra.MachineRequest, machine *machineStatusLabels) error {
	deleted, err := helpers.TeardownAndDestroy(ctx, r, request.Metadata())
	if err != nil {
		return err
	}

	if !deleted {
		return nil
	}

	if machine != nil {
		return r.RemoveFinalizer(ctx, machine.Metadata(), MachineRequestSetStatusControllerName)
	}

	return nil
}

func (h *machineRequestSetStatusHandler) reconcileTearingDown(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, machineRequestSet *omni.MachineRequestSet) error {
	machineRequests, err := safe.ReaderListAll[*infra.MachineRequest](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSet.Metadata().ID())))
	if err != nil {
		return err
	}

	for machineRequest := range machineRequests.All() {
		if err = h.handleInfraProviderDeletion(ctx, r, machineRequest); err != nil {
			return err
		}
	}

	destroyReady, err := helpers.TeardownAndDestroyAll(ctx, r, machineRequests.Pointers())
	if err != nil {
		return err
	}

	labels, err := safe.ReaderListAll[*machineStatusLabels](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSet.Metadata().ID())))
	if err != nil {
		return err
	}

	err = labels.ForEachErr(func(res *machineStatusLabels) error {
		return r.RemoveFinalizer(ctx, res.Metadata(), MachineRequestSetStatusControllerName)
	})
	if err != nil {
		return err
	}

	if !destroyReady {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("the machine request set still has tearing down machine requests")
	}

	return nil
}

func (h *machineRequestSetStatusHandler) handleInfraProviderDeletion(ctx context.Context, r controller.ReaderWriter, machine *infra.MachineRequest) error {
	infraProviderID, ok := machine.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if !ok {
		return nil
	}

	infraProvider, err := safe.ReaderGetByID[*infra.Provider](ctx, r, infraProviderID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// Remove all finalizers from all MachineRequests if the infra provider is being deleted or not found.
	if infraProvider != nil && infraProvider.Metadata().Phase() == resource.PhaseRunning {
		return nil
	}

	if err = safe.WriterModify[*infra.MachineRequest](ctx, r, machine, func(res *infra.MachineRequest) error {
		for _, finalizer := range *machine.Metadata().Finalizers() {
			if finalizer != clusterMachineRequestStatusControllerName {
				res.Metadata().Finalizers().Remove(finalizer)
			}
		}

		return nil
	}, controller.WithExpectedPhaseAny()); err != nil {
		return err
	}

	return nil
}

func mapMachineToMachineRequest(ctx context.Context, _ *zap.Logger, r controller.QRuntime, machine controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	machineRequest, ok := machine.Labels().Get(omni.LabelMachineRequest)
	if !ok {
		return nil, nil
	}

	request, err := safe.ReaderGetByID[*infra.MachineRequest](ctx, r, machineRequest)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	machineRequestSetName, ok := request.Metadata().Labels().Get(omni.LabelMachineRequestSet)
	if !ok {
		return nil, nil
	}

	return []resource.Pointer{
		omni.NewMachineRequestSet(machineRequestSetName).Metadata(),
	}, nil
}

func toMap[T resource.Resource](items safe.List[T]) map[resource.ID]T {
	res := make(map[resource.ID]T, items.Len())

	items.ForEach(func(t T) {
		res[t.Metadata().ID()] = t
	})

	return res
}
