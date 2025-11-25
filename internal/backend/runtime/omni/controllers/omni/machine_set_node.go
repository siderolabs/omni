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
	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

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

// NewMachineSetNodeController creates a new MachineSetNodeController.
func NewMachineSetNodeController() *MachineSetNodeController {
	return &MachineSetNodeController{
		NamedController: generic.NamedController{
			ControllerName: MachineSetNodeControllerName,
		},
	}
}

// Settings implements QController.
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
//
//nolint:gocognit,gocyclo,cyclop
func (ctrl *MachineSetNodeController) MapInput(
	ctx context.Context, _ *zap.Logger, r controller.QRuntime, ptr controller.ReducedResourceMetadata,
) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case omni.ClusterType:
		machineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, ptr.ID())),
		)
		if err != nil {
			return nil, err
		}

		return slices.Collect(machineSets.Pointers()), nil
	case system.ResourceLabelsType[*omni.MachineStatus]():
		status, err := safe.ReaderGetByID[*machineStatusLabels](ctx, r, ptr.ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, nil
			}

			return nil, err
		}

		selector := resource.LabelQuery{
			Terms: assignableMachineStatusLabelTerms,
		}

		machineIsPossiblyAssignable := selector.Matches(*status.Metadata().Labels())

		if machineIsPossiblyAssignable {
			return ctrl.getUpscalableMachinesets(ctx, r)
		}

		return getMachineSets(ctx, r, ptr.ID())
	case omni.MachineType:
		return getMachineSets(ctx, r, ptr.ID())
	case omni.MachineClassType:
		allMachineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r)
		if err != nil {
			return nil, err
		}

		var machineSetsWithClass []resource.Pointer

		allMachineSets.ForEach(func(ms *omni.MachineSet) {
			allocation := ms.TypedSpec().Value.MachineAllocation
			if allocation != nil && allocation.Name == ptr.ID() {
				machineSetsWithClass = append(machineSetsWithClass, ms.Metadata())
			}
		})

		return machineSetsWithClass, nil
	case omni.MachineSetNodeType:
		machineSetNode, err := safe.ReaderGet[*omni.MachineSetNode](ctx, r, ptr)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, nil
			}

			return nil, err
		}

		machineSetID, ok := machineSetNode.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			return nil, nil
		}

		return []resource.Pointer{omni.NewMachineSet(resources.DefaultNamespace, machineSetID).Metadata()}, nil
	case infra.MachineRequestType:
		machines, err := safe.ReaderListAll[*omni.Machine](ctx, r,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequest, ptr.ID())),
		)
		if err != nil {
			return nil, err
		}

		var machineSets []resource.Pointer

		for machine := range machines.All() {
			ms, err := getMachineSets(ctx, r, machine.Metadata().ID())
			if err != nil {
				return nil, err
			}

			if ms != nil {
				machineSets = append(machineSets, ms...)
			}
		}

		return machineSets, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

func (ctrl *MachineSetNodeController) getAllMachineSetNodes(ctx context.Context, r controller.QRuntime, opts ...state.ListOption) (safe.List[*omni.MachineSetNode], error) {
	items, err := r.ListUncached(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.MachineSetNodeType, "", resource.VersionUndefined),
		opts...,
	)
	if err != nil {
		return safe.List[*omni.MachineSetNode]{}, err
	}

	return safe.NewList[*omni.MachineSetNode](items), nil
}

// Reconcile implements QController.
func (ctrl *MachineSetNodeController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	machineSet, err := safe.ReaderGet[*omni.MachineSet](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	allocation := omni.GetMachineAllocation(machineSet)
	if allocation == nil {
		return nil
	}

	if machineSet.Metadata().Phase() == resource.PhaseRunning {
		return ctrl.reconcileRunning(ctx, logger, r, machineSet)
	}

	return ctrl.reconcileTearingDown(ctx, r, machineSet)
}

func (ctrl *MachineSetNodeController) reconcileRunning(ctx context.Context, logger *zap.Logger, r controller.QRuntime, machineSet *omni.MachineSet) error {
	if !machineSet.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, machineSet.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		return err
	}

	if _, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked); locked {
		logger.Warn("cluster is locked, skip reconcile", zap.String("cluster", cluster.Metadata().ID()))

		return nil
	}

	machineSetNodes, err := ctrl.getAllMachineSetNodes(ctx, r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())),
	)
	if err != nil {
		return err
	}

	nodeDiff, allocation, err := ctrl.shouldScale(ctx, r, machineSet, machineSetNodes)
	if err != nil {
		return err
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

	err = ctrl.scaleMachineSet(ctx, r, machineSet, cluster, allocation, allMachineStatuses, logger, machineSetNodes, machineSetMachineStatusMap, nodeDiff)
	if err != nil {
		return err
	}

	return ctrl.destroyOrphaned(ctx, r, machineSetNodes)
}

func (ctrl *MachineSetNodeController) reconcileTearingDown(ctx context.Context, r controller.QRuntime, machineSet *omni.MachineSet) error {
	machineSetNodes, err := safe.ReaderListAll[*omni.MachineSetNode](ctx, r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())),
	)
	if err != nil {
		return err
	}

	ready, err := helpers.TeardownAndDestroyAll(ctx, r, machineSetNodes.Pointers(), controller.WithOwner(""))
	if err != nil {
		return err
	}

	if !ready {
		return nil
	}

	if machineSet.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.RemoveFinalizer(ctx, machineSet.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *MachineSetNodeController) destroyOrphaned(
	ctx context.Context,
	r controller.QRuntime,
	machineSetNodes safe.List[*omni.MachineSetNode],
) error {
	var toDestroy []resource.Pointer

	for machineSetNode := range machineSetNodes.All() {
		machine, err := safe.ReaderGetByID[*omni.Machine](ctx, r, machineSetNode.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		shouldDestroy := machine == nil || machine.Metadata().Phase() == resource.PhaseTearingDown

		if machine != nil {
			machineRequestID, ok := machine.Metadata().Labels().Get(omni.LabelMachineRequest)
			if ok {
				var machineRequest *infra.MachineRequest

				machineRequest, err = safe.ReaderGetByID[*infra.MachineRequest](ctx, r, machineRequestID)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				shouldDestroy = machineRequest == nil || machineRequest.Metadata().Phase() == resource.PhaseTearingDown
			}
		}

		if shouldDestroy {
			toDestroy = append(toDestroy, machineSetNode.Metadata())
		}
	}

	_, err := helpers.TeardownAndDestroyAll(ctx, r, slices.Values(toDestroy), controller.WithOwner(""))

	return err
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

func (ctrl *MachineSetNodeController) scaleMachineSet(
	ctx context.Context,
	r controller.QRuntime,
	machineSet *omni.MachineSet,
	cluster *omni.Cluster,
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

	return ctrl.createNodes(ctx, r, machineSet, cluster, allocation, allMachineStatuses, diff, logger)
}

func (ctrl *MachineSetNodeController) shouldScale(
	ctx context.Context,
	r controller.QRuntime,
	machineSet *omni.MachineSet,
	machineSetNodes safe.List[*omni.MachineSetNode],
) (
	nodeDiff int, allocation *allocationConfig, err error,
) {
	if machineSet.Metadata().Phase() == resource.PhaseTearingDown {
		return 0, allocation, nil
	}

	allocation, err = ctrl.getMachineAllocation(ctx, r, machineSet)
	if err != nil {
		return 0, allocation, err
	}

	if allocation == nil {
		return 0, allocation, nil
	}

	switch allocation.allocationType {
	case specs.MachineSetSpec_MachineAllocation_Unlimited:
		nodeDiff = unlimitedNodeCount
	case specs.MachineSetSpec_MachineAllocation_Static:
		nodeDiff = int(allocation.machineCount) - machineSetNodes.Len()
	}

	return nodeDiff, allocation, nil
}

//nolint:gocognit
func (ctrl *MachineSetNodeController) createNodes(
	ctx context.Context,
	r controller.QRuntime,
	machineSet *omni.MachineSet,
	cluster *omni.Cluster,
	allocation *allocationConfig,
	allMachineStatuses safe.List[*machineStatusLabels],
	count int,
	logger *zap.Logger,
) (err error) {
	created := 0

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

			msn := omni.NewMachineSetNode(resources.DefaultNamespace, id, machineSet)

			msn.Metadata().Labels().Set(omni.LabelManagedByMachineSetNodeController, "")

			if err = r.Create(ctx, msn, controller.WithCreateNoOwner()); err != nil {
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
			return r.Destroy(ctx, machineSetNode.Metadata(), controller.WithOwner(""))
		}

		return nil
	}); err != nil {
		return err
	}

	iterations := min(machinesToDestroyCount, len(usedMachineSetNodes))

	for i := range iterations {
		var (
			ready bool
			err   error
		)
		if ready, err = helpers.TeardownAndDestroy(
			ctx, r, usedMachineSetNodes[i].Metadata(),
			controller.WithOwner(""),
		); err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		if !ready {
			return nil
		}

		logger.Info("removed machine set node", zap.String("machine", usedMachineSetNodes[i].Metadata().ID()))
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

func getMachineSets(ctx context.Context, r controller.QRuntime, machineID resource.ID) ([]resource.Pointer, error) {
	machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, r, machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	machineSetID, ok := machineSetNode.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return nil, nil
	}

	return []resource.Pointer{omni.NewMachineSet(resources.DefaultNamespace, machineSetID).Metadata()}, nil
}

// getUpscalableMachinesets returns machine sets that have room to grow.
func (ctrl *MachineSetNodeController) getUpscalableMachinesets(ctx context.Context, r controller.QRuntime) ([]resource.Pointer, error) {
	allMachineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r)
	if err != nil {
		return nil, err
	}

	upscalableMachineSets := []resource.Pointer{}

	machineSetNodes, err := ctrl.getAllMachineSetNodes(ctx, r)
	if err != nil {
		return nil, err
	}

	if err := allMachineSets.ForEachErr(func(ms *omni.MachineSet) error {
		nodeDiff, _, err := ctrl.shouldScale(ctx, r, ms, machineSetNodes.FilterLabelQuery(
			resource.LabelEqual(omni.LabelMachineSet, ms.Metadata().ID()),
		))
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
