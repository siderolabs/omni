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
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

type machineStatusLabels = system.ResourceLabels[*omni.MachineStatus]

const labelEvicted = "evicted"

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
			Type:      system.ResourceLabelsType[*omni.MachineStatus](),
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
			Namespace: resources.InfraProviderNamespace,
			Type:      infra.MachineRequestType,
			Kind:      controller.InputStrong,
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
//
//nolint:gocognit,gocyclo,cyclop
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

		machineMap := make(map[resource.ID]*omni.Machine, allMachines.Len())

		err = allMachines.ForEachErr(func(machine *omni.Machine) error {
			requestName, ok := machine.Metadata().Labels().Get(omni.LabelMachineRequest)
			if ok {
				request, e := safe.ReaderGetByID[*infra.MachineRequest](ctx, r, requestName)
				if e != nil && !state.IsNotFoundError(e) {
					return e
				}

				if request == nil || request.Metadata().Phase() == resource.PhaseTearingDown {
					return nil
				}
			}

			machineMap[machine.Metadata().ID()] = machine

			return nil
		})
		if err != nil {
			return err
		}

		allMachineStatuses, err := safe.ReaderListAll[*machineStatusLabels](ctx, r)
		if err != nil {
			return err
		}

		machineStatusMap := map[resource.ID]*machineStatusLabels{}

		allMachineStatuses.ForEach(func(ms *machineStatusLabels) {
			if m, ok := machineMap[ms.Metadata().ID()]; !ok || m.Metadata().Phase() == resource.PhaseTearingDown {
				ms.Metadata().Labels().Set(labelEvicted, "")

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
		if err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
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

	if _, managed := machineSet.Metadata().Labels().Get(omni.LabelManaged); managed {
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
			allocationType: specs.MachineSetSpec_MachineAllocation_Static,
			machineCount:   3,
		}, nil
	}

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

func (ctrl *MachineSetNodeController) reconcileMachineSet(
	ctx context.Context,
	r controller.Runtime,
	machineSet *omni.MachineSet,
	allMachineStatuses safe.List[*machineStatusLabels],
	allMachineSetNodes safe.List[*omni.MachineSetNode],
	machineStatusMap map[resource.ID]*machineStatusLabels,
	visited map[resource.ID]struct{},
	logger *zap.Logger,
) (err error) {
	if machineSet.Metadata().Phase() == resource.PhaseTearingDown {
		return nil
	}

	allocation, err := ctrl.getMachineAllocation(ctx, r, machineSet)
	if err != nil {
		return err
	}

	if allocation == nil {
		return nil
	}

	visited[machineSet.Metadata().ID()] = struct{}{}

	existingMachineSetNodes := allMachineSetNodes.FilterLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()))

	switch allocation.allocationType {
	case specs.MachineSetSpec_MachineAllocation_Unlimited:
		err = ctrl.createNodes(ctx, r, machineSet, allocation, allMachineStatuses, math.MaxInt32, logger)

		return err // unlimited allocation mode does not cause any machine pressure
	case specs.MachineSetSpec_MachineAllocation_Static:
		diff := int(allocation.machineCount) - existingMachineSetNodes.Len()

		if diff == 0 {
			return nil
		}

		if diff < 0 {
			logger.Info("scaling machine set down", zap.Int("pending", -diff), zap.String("machine_set", machineSet.Metadata().ID()))

			return ctrl.deleteNodes(ctx, r, existingMachineSetNodes, machineStatusMap, -diff, logger)
		}

		logger.Info("scaling machine set up", zap.Int("pending", diff), zap.String("machine_set", machineSet.Metadata().ID()))

		return ctrl.createNodes(ctx, r, machineSet, allocation, allMachineStatuses, diff, logger)
	}

	return nil
}

//nolint:gocognit
func (ctrl *MachineSetNodeController) createNodes(
	ctx context.Context,
	r controller.Runtime,
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
		selector.Terms = append(selector.Terms, resource.LabelTerm{
			Key: omni.MachineStatusLabelAvailable,
			Op:  resource.LabelOpExists,
		}, resource.LabelTerm{
			Key: omni.MachineStatusLabelConnected,
			Op:  resource.LabelOpExists,
		}, resource.LabelTerm{
			Key: omni.MachineStatusLabelReportingEvents,
			Op:  resource.LabelOpExists,
		}, resource.LabelTerm{
			Key:    labelEvicted,
			Op:     resource.LabelOpExists,
			Invert: true,
		})

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
	r controller.Runtime,
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
