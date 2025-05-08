// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

// Questions for pr
// 1. which data is stored in the spec and which in the labels?

type machineStatusLabels = system.ResourceLabels[*omni.MachineStatus]

const labelEvicted = "evicted"

const unlimitedNodeCount = math.MaxInt32

// assignableMachineStatusLabelTerms are the terms that have to be met in order for a machine to be considered for a machineSet.
var assignableMachineStatusLabelTerms = []resource.LabelTerm{
	{
		Key: omni.MachineStatusLabelAvailable,
		Op:  resource.LabelOpExists,
	}, {
		Key: omni.MachineStatusLabelReadyToUse,
		Op:  resource.LabelOpExists,
	}, {
		Key: omni.MachineStatusLabelReportingEvents,
		Op:  resource.LabelOpExists,
	}, {
		Key:    labelEvicted,
		Op:     resource.LabelOpExists,
		Invert: true,
	},
}

// MachineSetNodeControllerName is the name of the MachineSetNodeController.
const MachineSetNodeControllerName = "MachineSetNodeController"

// MachineSetNodeController manages MachineSetNode resource lifecycle.
//
// MachineSetNodeController creates and deletes cluster machines, handles rolling updates.
type MachineSetNodeController struct {
	generic.NamedController
}

func NewMachineSetNodeController() *MachineSetNodeController {
	return &MachineSetNodeController{
		NamedController: generic.NamedController{
			ControllerName: MachineSetNodeControllerName,
		},
	}
}

func (ctrl *MachineSetNodeController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineSetType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      system.ResourceLabelsType[*omni.MachineStatus](),
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineClassType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineSetNodeType,
				Kind:      controller.InputQMappedDestroyReady,
			},
			{
				Namespace: resources.InfraProviderNamespace,
				Type:      infra.MachineRequestType,
				Kind:      controller.InputQMapped,
			},
		},
		Outputs: []controller.Output{
			{
				Type: omni.MachineSetNodeType,
				Kind: controller.OutputShared,
			},
		},
		Concurrency: optional.Some(uint(4)),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineSetNodeController) MapInput(
	ctx context.Context, _ *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case omni.ClusterType:
		machineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, ptr.ID())),
		)
		if err != nil {
			return nil, err
		}

		res := []resource.Pointer{}

		machineSets.ForEach(func(ms *omni.MachineSet) {
			res = append(res, ms.Metadata())
		})

		return res, nil
	case omni.MachineType, system.ResourceLabelsType[*omni.MachineStatus]():
		machineSet, err := getMachineSetOfMachine(ctx, r, ptr.ID())
		if err != nil {
			return nil, err
		}

		if machineSet != nil {
			return machineSet, nil
		}

		// machine is not part of a machine set. Check if machine is allocatable, if so find machineSets that would be interested in the machine.
		status, err := safe.ReaderGetByID[*machineStatusLabels](ctx, r, ptr.ID())
		if err != nil {
			return nil, err
		}

		selector := resource.LabelQuery{
			Terms: assignableMachineStatusLabelTerms,
		}

		machineIsPossiblyAssignable := selector.Matches(*status.Metadata().Labels())

		if machineIsPossiblyAssignable {
			return ctrl.getUpscalableMachinesets(ctx, r)
		}

		return nil, nil
	case omni.MachineClassType:
		allMachineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r)
		if err != nil {
			return nil, err
		}

		var machineSetsWithClass []resource.Pointer

		allMachineSets.ForEach(func(ms *omni.MachineSet) {
			allocation := ms.TypedSpec().Value.MachineAllocation
			if allocation.Source == specs.MachineSetSpec_MachineAllocation_MachineClass && allocation.Name == ptr.ID() {
				machineSetsWithClass = append(machineSetsWithClass, ms.Metadata())
			}
		})

		return machineSetsWithClass, nil
	case omni.MachineSetNodeType:
		machineSetNode, err := safe.ReaderGet[*omni.MachineSetNode](ctx, r, ptr)
		if err != nil {
			return nil, err
		}

		machineSetId, ok := machineSetNode.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			// maybe should log a warning. Ask in pr
			return nil, nil
		}

		return []resource.Pointer{omni.NewMachineSet(resources.DefaultNamespace, machineSetId).Metadata()}, nil
	case infra.MachineRequestType:
		machineRequest, err := safe.ReaderGet[*infra.MachineRequest](ctx, r, ptr)
		if err != nil {
			return nil, err
		}

		machineId, ok := machineRequest.Metadata().Labels().Get(omni.LabelMachine)
		if !ok {
			return nil, nil
		}

		return getMachineSetOfMachine(ctx, r, machineId)
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

func (ctrl *MachineSetNodeController) destroyOrphaned(
	ctx context.Context,
	r controller.QRuntime,
	machineSetNodes safe.List[*omni.MachineSetNode],
) error {
	return machineSetNodes.ForEachErr(func(machineSetNode *omni.MachineSetNode) error {
		machine, err := safe.ReaderGetByID[*omni.Machine](ctx, r, machineSetNode.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		if machine.Metadata().Phase() != resource.PhaseTearingDown {
			return nil
		}

		machineRequestID, ok := machine.Metadata().Labels().Get(omni.LabelMachineRequest)
		if ok {
			request, e := safe.ReaderGetByID[*infra.MachineRequest](ctx, r, machineRequestID)
			if e != nil && !state.IsNotFoundError(e) {
				return e
			}

			if request.Metadata().Phase() == resource.PhaseTearingDown {
				return nil
			}
		}

		var ready bool

		ready, err = r.Teardown(ctx, machineSetNode.Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		if !ready {
			return nil
		}

		err = r.Destroy(ctx, machineSetNode.Metadata())
		if err != nil && state.IsNotFoundError(err) {
			return err
		}

		return nil
	})
}

type allocationConfig struct {
	selectors      resource.LabelQueries
	machineCount   uint32
	allocationType specs.MachineSetSpec_MachineAllocation_Type
	manual         bool
}

func (ctrl *MachineSetNodeController) getMachineAllocation(ctx context.Context, r controller.Reader, machineSet *omni.MachineSet) (*allocationConfig, error) {
	var (
		selectors         resource.LabelQueries
		machineAllocation = omni.GetMachineAllocation(machineSet)
	)

	if machineAllocation == nil {
		return nil, nil //nolint:nilnil
	}

	machineClass, err := safe.ReaderGet[*omni.MachineClass](ctx, r, omni.NewMachineClass(resources.DefaultNamespace, machineAllocation.Name).Metadata())
	if err != nil {
		return nil, err
	}

	if machineClass.TypedSpec().Value.AutoProvision != nil {
		selectors = append(selectors, resource.LabelQuery{
			Terms: []resource.LabelTerm{
				{
					Key:   omni.LabelMachineRequestSet,
					Op:    resource.LabelOpEqual,
					Value: []string{machineSet.Metadata().ID()},
				},
			},
		})

		return &allocationConfig{
			selectors:      selectors,
			allocationType: machineAllocation.AllocationType,
			machineCount:   machineAllocation.MachineCount,
		}, nil
	}

	selectors, err = labels.ParseSelectors(machineClass.TypedSpec().Value.MatchLabels)
	if err != nil {
		return nil, err
	}

	return &allocationConfig{
		selectors:      selectors,
		allocationType: machineAllocation.AllocationType,
		machineCount:   machineAllocation.MachineCount,
		manual:         true,
	}, nil
}

func (ctrl *MachineSetNodeController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	machineSet, err := safe.ReaderGet[*omni.MachineSet](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	nodeDiff, allocation, machineSetNodes, err := ctrl.shouldScale(ctx, r, machineSet)
	if err != nil {
		return err
	}

	if nodeDiff == 0 {
		return nil
	}

	allMachineStatuses, err := safe.ReaderListAll[*machineStatusLabels](ctx, r)
	if err != nil {
		return err
	}

	machineSetMachineStatusMap := map[resource.ID]*machineStatusLabels{}

	machineSetNodes.ForEach(func(msn *omni.MachineSetNode) {
		machineStatus, ok := allMachineStatuses.Find(func(msl *machineStatusLabels) bool { return msl.Metadata().ID() == msn.Metadata().ID() })
		if !ok || machineStatus.Metadata().Phase() == resource.PhaseTearingDown {
			return
		}

		machineSetMachineStatusMap[msn.Metadata().ID()] = machineStatus
	})

	err = ctrl.scale(ctx, r, machineSet, allocation, allMachineStatuses, logger, machineSetNodes, machineSetMachineStatusMap, nodeDiff)
	if err != nil {
		return err
	}

	return ctrl.destroyOrphaned(ctx, r, machineSetNodes)
}

func (ctrl *MachineSetNodeController) scale(
	ctx context.Context,
	r controller.QRuntime,
	machineSet *omni.MachineSet,
	allocation *allocationConfig,
	allMachineStatuses safe.List[*machineStatusLabels],
	logger *zap.Logger,
	existingMachineSetNodes safe.List[*omni.MachineSetNode],
	machineSetMachineStatusMap map[resource.ID]*machineStatusLabels,
	diff int,
) error {
	if diff == 0 {
		return nil
	}

	logFields := []zap.Field{zap.String("machine_set", machineSet.Metadata().ID())}

	if diff < 0 {
		logFields = append(logFields, zap.Int("pending", -diff))

		logger.Info("scaling machine set down", logFields...)

		return ctrl.deleteNodes(ctx, r, existingMachineSetNodes, machineSetMachineStatusMap, -diff, logger)
	}

	// don't scare users with big number
	if diff != unlimitedNodeCount {
		logFields = append(logFields, zap.Int("pending", diff))
	}

	logger.Info("scaling machine set up", logFields...)

	return ctrl.createNodes(ctx, r, machineSet, allocation, allMachineStatuses, diff, logger)
}

func (ctrl *MachineSetNodeController) shouldScale(ctx context.Context, r controller.QRuntime, machineSet *omni.MachineSet) (
	nodeDiff int, allocation *allocationConfig, machineSetNodes safe.List[*omni.MachineSetNode], err error,
) {
	if machineSet.Metadata().Phase() == resource.PhaseTearingDown {
		return 0, allocation, machineSetNodes, nil
	}

	allocation, err = ctrl.getMachineAllocation(ctx, r, machineSet)
	if err != nil {
		return 0, allocation, machineSetNodes, err
	}

	if allocation == nil {
		return 0, allocation, machineSetNodes, nil
	}

	machineSetNodes, err = safe.ReaderListAll[*omni.MachineSetNode](
		ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())))
	if err != nil {
		return 0, allocation, machineSetNodes, err
	}

	switch allocation.allocationType {
	case specs.MachineSetSpec_MachineAllocation_Unlimited:
		nodeDiff = unlimitedNodeCount
	case specs.MachineSetSpec_MachineAllocation_Static:
		nodeDiff = int(allocation.machineCount) - machineSetNodes.Len()
	}

	return nodeDiff, allocation, machineSetNodes, nil
}

//nolint:gocognit
func (ctrl *MachineSetNodeController) createNodes(
	ctx context.Context,
	r controller.QRuntime,
	machineSet *omni.MachineSet,
	allocation *allocationConfig,
	allMachineStatuses safe.List[*machineStatusLabels],
	count int,
	logger *zap.Logger,
) (err error) {
	created := 0

	clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("failed to get cluster name of the machine set %q", machineSet.Metadata().ID())
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	clusterVersion, err := semver.Parse(cluster.TypedSpec().Value.TalosVersion)
	if err != nil {
		return fmt.Errorf("failed to parse talos version of the cluster %w", err)
	}

	for _, selector := range allocation.selectors {
		selector.Terms = append(selector.Terms, assignableMachineStatusLabelTerms...)

		if allocation.manual {
			selector.Terms = append(selector.Terms, resource.LabelTerm{
				Key:    omni.LabelNoManualAllocation,
				Op:     resource.LabelOpExists,
				Invert: true,
			})
		}

		availableMachineClassMachines := allMachineStatuses.FilterLabelQuery(resource.RawLabelQuery(selector))

		for i := range availableMachineClassMachines.Len() {
			machine := availableMachineClassMachines.Get(i)

			var machineVersion semver.Version

			version, ok := machine.Metadata().Labels().Get(omni.MachineStatusLabelTalosVersion)
			if !ok {
				continue
			}

			machineVersion, err = semver.Parse(strings.TrimPrefix(version, "v"))
			if err != nil {
				continue
			}

			// do not try to allocate the machine if it's Talos major or minor version is greater than cluster Talos version
			// this way we don't allow downgrading the machines while allocating them
			if machineVersion.Major > clusterVersion.Major || machineVersion.Minor > clusterVersion.Minor {
				continue
			}

			// do not try to allocate the machine if it's running Talos from an ISO or PXE and it's major and minor version do not match.
			_, installed := machine.Metadata().Labels().Get(omni.MachineStatusLabelInstalled)

			if !installed && (machineVersion.Major != clusterVersion.Major || machineVersion.Minor != clusterVersion.Minor) {
				continue
			}

			id := machine.Metadata().ID()

			if err := r.Create(ctx, omni.NewMachineSetNode(resources.DefaultNamespace, id, machineSet)); err != nil {
				if state.IsConflictError(err) {
					continue
				}

				return err
			}

			logger.Info("created machine set node", zap.String("machine", id))

			created++
			if created == count {
				return nil
			}
		}
	}

	return nil
}

func (ctrl *MachineSetNodeController) deleteNodes(
	ctx context.Context,
	r controller.QRuntime,
	machineSetNodes safe.List[*omni.MachineSetNode],
	machineStatuses map[string]*machineStatusLabels,
	machinesToDestroyCount int,
	logger *zap.Logger,
) error {
	usedMachineSetNodes, err := safe.Map(machineSetNodes, func(m *omni.MachineSetNode) (*omni.MachineSetNode, error) {
		return m, nil
	})
	if err != nil {
		return err
	}

	// filter only running used machines
	xslices.FilterInPlace(usedMachineSetNodes, func(r *omni.MachineSetNode) bool {
		return r.Metadata().Phase() == resource.PhaseRunning
	})

	slices.SortStableFunc(usedMachineSetNodes, getSortFunction(machineStatuses))

	// destroy all machines which are currently in tearing down phase and have no finalizers
	if err = machineSetNodes.ForEachErr(func(machineSetNode *omni.MachineSetNode) error {
		if machineSetNode.Metadata().Phase() == resource.PhaseRunning {
			return nil
		}

		machinesToDestroyCount--
		if machineSetNode.Metadata().Finalizers().Empty() {
			return r.Destroy(ctx, machineSetNode.Metadata())
		}

		return nil
	}); err != nil {
		return err
	}

	iterations := len(usedMachineSetNodes)
	if machinesToDestroyCount < iterations {
		iterations = machinesToDestroyCount
	}

	for i := range iterations {
		var (
			ready bool
			err   error
		)

		if ready, err = r.Teardown(ctx, usedMachineSetNodes[i].Metadata()); err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		logger.Info("removed machine set node", zap.String("machine", usedMachineSetNodes[i].Metadata().ID()))

		if !ready {
			return nil
		}

		if err = r.Destroy(ctx, usedMachineSetNodes[i].Metadata()); err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}
	}

	return nil
}

func getSortFunction(machineStatuses map[resource.ID]*machineStatusLabels) func(a, b *omni.MachineSetNode) int {
	return func(a, b *omni.MachineSetNode) int {
		ms1, ok1 := machineStatuses[a.Metadata().ID()]
		ms2, ok2 := machineStatuses[b.Metadata().ID()]

		if !ok1 && ok2 {
			return -1
		}

		if ok1 && !ok2 {
			return 1
		}

		if !ok1 && !ok2 {
			return 0
		}

		_, disconnected1 := ms1.Metadata().Labels().Get(omni.MachineStatusLabelDisconnected)
		_, disconnected2 := ms2.Metadata().Labels().Get(omni.MachineStatusLabelDisconnected)

		if disconnected1 && !disconnected2 {
			return -1
		}

		if !disconnected1 && disconnected2 {
			return 1
		}

		return a.Metadata().Created().Compare(b.Metadata().Created())
	}
}

func getMachineSetOfMachine(ctx context.Context, r controller.QRuntime, machineID resource.ID) ([]resource.Pointer, error) {
	machine, err := safe.ReaderGetByID[*omni.Machine](ctx, r, machineID)
	if err != nil {
		return nil, err
	}

	machineSetId, ok := machine.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return nil, nil
	}

	return []resource.Pointer{omni.NewMachineSet(resources.DefaultNamespace, machineSetId).Metadata()}, nil
}

// getUpscalableMachinesets returns machine sets that have room to grow.
func (ctrl *MachineSetNodeController) getUpscalableMachinesets(ctx context.Context, r controller.QRuntime) ([]resource.Pointer, error) {
	allMachineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r)
	if err != nil {
		return nil, err
	}

	upscalableMachineSets := []resource.Pointer{}

	if err := allMachineSets.ForEachErr(func(ms *omni.MachineSet) error {
		nodeDiff, _, _, err := ctrl.shouldScale(ctx, r, ms)
		if err != nil {
			return err
		}
		// machineSet has room to grow and could potentially use more machines
		if nodeDiff > 0 {
			upscalableMachineSets = append(upscalableMachineSets, ms.Metadata())
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return upscalableMachineSets, nil
}
