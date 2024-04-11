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
)

// MachineSetNodeController manages MachineSetNode resource lifecycle.
//
// MachineSetNodeController creates and deletes cluster machines, handles rolling updates.
type MachineSetNodeController struct{}

// Name implements controller.Controller interface.
func (ctrl *MachineSetNodeController) Name() string {
	return "MachineSetNodeController"
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
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MachineSetNodeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineSetNodeType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
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

		allMachineSetNodes, err := safe.ReaderListAll[*omni.MachineSetNode](ctx, r)
		if err != nil {
			return err
		}

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

			return r.Destroy(ctx, machineSetNode.Metadata())
		})
		if err != nil {
			return err
		}
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
	var err error

	spec := machineSet.TypedSpec()
	if spec.Value.MachineClass == nil || machineSet.Metadata().Phase() == resource.PhaseTearingDown {
		return nil
	}

	visited[machineSet.Metadata().ID()] = struct{}{}

	machineClassCfg := spec.Value.MachineClass

	var machineClass *omni.MachineClass

	machineClass, err = safe.ReaderGet[*omni.MachineClass](ctx, r, omni.NewMachineClass(resources.DefaultNamespace, machineClassCfg.Name).Metadata())
	if err != nil {
		return err
	}

	existingMachineSetNodes := allMachineSetNodes.FilterLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()))

	switch machineClassCfg.AllocationType {
	case specs.MachineSetSpec_MachineClass_Unlimited:
		return ctrl.createNodes(ctx, r, machineSet, machineClass, allMachineStatuses, math.MaxInt32)
	case specs.MachineSetSpec_MachineClass_Static:
		diff := int(machineClassCfg.MachineCount) - existingMachineSetNodes.Len()

		if diff == 0 {
			return nil
		}

		if diff < 0 {
			logger.Info("scaling machine set down", zap.Int("pending", -diff), zap.String("machine_set", machineSet.Metadata().ID()))

			return ctrl.deleteNodes(ctx, r, existingMachineSetNodes, machineStatusMap, -diff)
		}

		logger.Info("scaling machine set up", zap.Int("pending", diff), zap.String("machine_set", machineSet.Metadata().ID()))

		return ctrl.createNodes(ctx, r, machineSet, machineClass, allMachineStatuses, diff)
	}

	return nil
}

func (ctrl *MachineSetNodeController) createNodes(
	ctx context.Context,
	r controller.Runtime,
	machineSet *omni.MachineSet,
	machineClass *omni.MachineClass,
	allMachineStatuses safe.List[*omni.MachineStatus],
	count int,
) error {
	selectors, err := labels.ParseSelectors(machineClass.TypedSpec().Value.MatchLabels)
	if err != nil {
		return err
	}

	created := 0

	for _, selector := range selectors {
		selector.Terms = append(selector.Terms, resource.LabelTerm{
			Key: omni.MachineStatusLabelAvailable,
			Op:  resource.LabelOpExists,
		})

		availableMachineClassMachines := allMachineStatuses.FilterLabelQuery(resource.RawLabelQuery(selector))

		for i := range availableMachineClassMachines.Len() {
			id := availableMachineClassMachines.Get(i).Metadata().ID()

			if err := r.Create(ctx, omni.NewMachineSetNode(resources.DefaultNamespace, id, machineSet)); err != nil {
				if state.IsConflictError(err) {
					continue
				}

				return err
			}

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
	r controller.Runtime,
	machineSetNodes safe.List[*omni.MachineSetNode],
	machineStatuses map[string]*omni.MachineStatus,
	machinesToDestroyCount int,
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
