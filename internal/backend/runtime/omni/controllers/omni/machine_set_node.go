// Copyright (c) 2024 Sidero Labs, Inc.
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
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineSetNodeControllerName is the name of the MachineSetNodeController.
const MachineSetNodeControllerName = "MachineSetNodeController"

// MachineSetNodeController manages MachineSetNode resource lifecycle.
//
// MachineSetNodeController creates and deletes cluster machines, handles rolling updates.
type MachineSetNodeController struct{}

// Name implements controller.Controller interface.
func (ctrl *MachineSetNodeController) Name() string {
	return MachineSetNodeControllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineSetNodeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineSetType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineClassType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineSetNodeType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineSetRequiredMachinesType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MachineSetNodeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineSetNodeType,
			Kind: controller.OutputShared,
		},
		{
			Type: omni.MachineSetRequiredMachinesType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocognit
func (ctrl *MachineSetNodeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		list, err := safe.ReaderListAll[*omni.MachineSet](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing machine sets: %w", err)
		}

		machineSetNodes, err := r.ListUncached(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.MachineSetNodeType, "", resource.VersionUndefined))
		if err != nil {
			return err
		}

		allMachineSetNodes := safe.NewList[*omni.MachineSetNode](machineSetNodes)

		allMachines, err := safe.ReaderListAll[*omni.Machine](ctx, r)
		if err != nil {
			return err
		}

		machineMap := map[resource.ID]*omni.Machine{}

		allMachines.ForEach(func(machine *omni.Machine) {
			machineMap[machine.Metadata().ID()] = machine
		})

		allMachineStatuses, err := safe.ReaderListAll[*omni.MachineStatus](ctx, r)
		if err != nil {
			return err
		}

		machineStatusMap := map[resource.ID]*omni.MachineStatus{}

		allMachineStatuses.ForEach(func(ms *omni.MachineStatus) {
			if m, ok := machineMap[ms.Metadata().ID()]; !ok || m.Metadata().Phase() == resource.PhaseTearingDown {
				return
			}

			machineStatusMap[ms.Metadata().ID()] = ms
		})

		visited := map[resource.ID]struct{}{}

		err = list.ForEachErr(func(machineSet *omni.MachineSet) error {
			return ctrl.reconcileMachineSet(ctx, r, machineSet, allMachineStatuses, allMachineSetNodes, machineStatusMap, visited, logger)
		})
		if err != nil {
			return err
		}

		err = allMachineSetNodes.ForEachErr(func(machineSetNode *omni.MachineSetNode) error {
			if machineSetNode.Metadata().Owner() != ctrl.Name() {
				return nil
			}

			machineSet, ok := machineSetNode.Metadata().Labels().Get(omni.LabelMachineSet)
			if !ok {
				return nil
			}

			machine, machineExists := machineMap[machineSetNode.Metadata().ID()]

			if _, ok = visited[machineSet]; ok && machineExists && machine.Metadata().Phase() != resource.PhaseTearingDown {
				return nil
			}

			var ready bool

			ready, err = r.Teardown(ctx, machineSetNode.Metadata())
			if err != nil {
				return err
			}

			if !ready {
				return nil
			}

			return r.Destroy(ctx, machineSetNode.Metadata())
		})
		if err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *MachineSetNodeController) reconcileMachineSet(
	ctx context.Context,
	r controller.Runtime,
	machineSet *omni.MachineSet,
	allMachineStatuses safe.List[*omni.MachineStatus],
	allMachineSetNodes safe.List[*omni.MachineSetNode],
	machineStatusMap map[resource.ID]*omni.MachineStatus,
	visited map[resource.ID]struct{},
	logger *zap.Logger,
) error {
	requiredAdditionalMachines, err := ctrl.reconcileMachineSetNodes(ctx, r, machineSet, allMachineStatuses, allMachineSetNodes, machineStatusMap, visited, logger)
	if err != nil {
		return err
	}

	if machineSet.TypedSpec().Value.MachineClass == nil || machineSet.Metadata().Phase() == resource.PhaseTearingDown {
		var ready bool

		if ready, err = r.Teardown(ctx, omni.NewMachineSetRequiredMachines(resources.DefaultNamespace, machineSet.Metadata().ID()).Metadata()); err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if ready {
			if err = r.Destroy(ctx, omni.NewMachineSetRequiredMachines(resources.DefaultNamespace, machineSet.Metadata().ID()).Metadata()); err != nil && !state.IsNotFoundError(err) {
				return err
			}
		}

		return nil
	}

	if err = ctrl.saveRequiredAdditionalMachines(ctx, r, machineSet, requiredAdditionalMachines); err != nil && !state.IsPhaseConflictError(err) {
		return err
	}

	return nil
}

func (ctrl *MachineSetNodeController) reconcileMachineSetNodes(
	ctx context.Context,
	r controller.Runtime,
	machineSet *omni.MachineSet,
	allMachineStatuses safe.List[*omni.MachineStatus],
	allMachineSetNodes safe.List[*omni.MachineSetNode],
	machineStatusMap map[resource.ID]*omni.MachineStatus,
	visited map[resource.ID]struct{},
	logger *zap.Logger,
) (requiredAdditionalMachines int, err error) {
	spec := machineSet.TypedSpec()

	if spec.Value.MachineClass == nil || machineSet.Metadata().Phase() == resource.PhaseTearingDown {
		return 0, nil
	}

	visited[machineSet.Metadata().ID()] = struct{}{}

	machineClassCfg := spec.Value.MachineClass

	var machineClass *omni.MachineClass

	machineClass, err = safe.ReaderGet[*omni.MachineClass](ctx, r, omni.NewMachineClass(resources.DefaultNamespace, machineClassCfg.Name).Metadata())
	if err != nil {
		return 0, err
	}

	existingMachineSetNodes := allMachineSetNodes.FilterLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()))

	switch machineClassCfg.AllocationType {
	case specs.MachineSetSpec_MachineClass_Unlimited:
		_, err = ctrl.createNodes(ctx, r, machineSet, machineClass, allMachineStatuses, math.MaxInt32, logger)

		return 0, err // unlimited allocation mode does not cause any machine pressure
	case specs.MachineSetSpec_MachineClass_Static:
		diff := int(machineClassCfg.MachineCount) - existingMachineSetNodes.Len()

		if diff == 0 {
			return 0, nil
		}

		if diff < 0 {
			logger.Info("scaling machine set down", zap.Int("pending", -diff), zap.String("machine_set", machineSet.Metadata().ID()))

			return 0, ctrl.deleteNodes(ctx, r, existingMachineSetNodes, machineStatusMap, -diff, logger)
		}

		logger.Info("scaling machine set up", zap.Int("pending", diff), zap.String("machine_set", machineSet.Metadata().ID()))

		return ctrl.createNodes(ctx, r, machineSet, machineClass, allMachineStatuses, diff, logger)
	}

	return 0, nil
}

func (ctrl *MachineSetNodeController) saveRequiredAdditionalMachines(ctx context.Context, r controller.Runtime, machineSet *omni.MachineSet, numRequired int) error {
	return safe.WriterModify(ctx, r, omni.NewMachineSetRequiredMachines(resources.DefaultNamespace, machineSet.Metadata().ID()),
		func(res *omni.MachineSetRequiredMachines) error {
			helpers.CopyAllLabels(machineSet, res)

			machineClass := machineSet.TypedSpec().Value.MachineClass

			if machineClass != nil {
				res.Metadata().Labels().Set(omni.LabelMachineClassName, machineClass.Name)
			} else {
				res.Metadata().Labels().Delete(omni.LabelMachineClassName)
			}

			res.TypedSpec().Value.RequiredAdditionalMachines = uint32(numRequired)

			return nil
		},
	)
}

//nolint:gocognit
func (ctrl *MachineSetNodeController) createNodes(
	ctx context.Context,
	r controller.Runtime,
	machineSet *omni.MachineSet,
	machineClass *omni.MachineClass,
	allMachineStatuses safe.List[*omni.MachineStatus],
	count int,
	logger *zap.Logger,
) (requiredAdditionalMachines int, err error) {
	selectors, err := labels.ParseSelectors(machineClass.TypedSpec().Value.MatchLabels)
	if err != nil {
		return 0, err
	}

	created := 0

	clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return 0, fmt.Errorf("failed to get cluster name of the machine set %q", machineSet.Metadata().ID())
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return 0, nil
		}

		return 0, err
	}

	clusterVersion, err := semver.Parse(cluster.TypedSpec().Value.TalosVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to parse talos version of the cluster %w", err)
	}

	for _, selector := range selectors {
		selector.Terms = append(selector.Terms, resource.LabelTerm{
			Key: omni.MachineStatusLabelAvailable,
			Op:  resource.LabelOpExists,
		}, resource.LabelTerm{
			Key: omni.MachineStatusLabelConnected,
			Op:  resource.LabelOpExists,
		}, resource.LabelTerm{
			Key: omni.MachineStatusLabelReportingEvents,
			Op:  resource.LabelOpExists,
		})

		availableMachineClassMachines := allMachineStatuses.FilterLabelQuery(resource.RawLabelQuery(selector))

		for i := range availableMachineClassMachines.Len() {
			machine := availableMachineClassMachines.Get(i)

			var machineVersion semver.Version

			machineVersion, err = semver.Parse(strings.TrimPrefix(machine.TypedSpec().Value.TalosVersion, "v"))
			if err != nil {
				continue
			}

			// do not try to allocate the machine if it's Talos major or minor version is greater than cluster Talos version
			// this way we don't allow downgrading the machines while allocating them
			if machineVersion.Major > clusterVersion.Major || machineVersion.Minor > clusterVersion.Minor {
				continue
			}

			// do not try to allocate the machine if it's running Talos from an ISO or PXE and it's major and minor version do not match.
			installed := omni.GetMachineStatusSystemDisk(machine) != ""

			if !installed && (machineVersion.Major != clusterVersion.Major || machineVersion.Minor != clusterVersion.Minor) {
				continue
			}

			id := machine.Metadata().ID()

			if err := r.Create(ctx, omni.NewMachineSetNode(resources.DefaultNamespace, id, machineSet)); err != nil {
				if state.IsConflictError(err) {
					continue
				}

				return 0, err
			}

			logger.Info("created machine set node", zap.String("machine", id))

			created++
			if created == count {
				return 0, nil
			}
		}
	}

	return count - created, nil
}

func (ctrl *MachineSetNodeController) deleteNodes(
	ctx context.Context,
	r controller.Runtime,
	machineSetNodes safe.List[*omni.MachineSetNode],
	machineStatuses map[string]*omni.MachineStatus,
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
			return err
		}

		logger.Info("removed machine set node", zap.String("machine", usedMachineSetNodes[i].Metadata().ID()))

		if !ready {
			return nil
		}

		if err = r.Destroy(ctx, usedMachineSetNodes[i].Metadata()); err != nil {
			return err
		}
	}

	return nil
}

func getSortFunction(machineStatuses map[resource.ID]*omni.MachineStatus) func(a, b *omni.MachineSetNode) int {
	return func(a, b *omni.MachineSetNode) int {
		ms1, ok1 := machineStatuses[a.Metadata().ID()]
		ms2, ok2 := machineStatuses[b.Metadata().ID()]

		if !ok1 && ok2 {
			return -1
		}

		if ok1 && !ok2 {
			return 1
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
