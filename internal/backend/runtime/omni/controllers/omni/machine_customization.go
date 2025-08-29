// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineExtraKernelArgsControllerName is the name of the MachineExtraKernelArgsController.
const MachineExtraKernelArgsControllerName = "MachineExtraKernelArgsController"

// MachineExtraKernelArgsController splits a single extraKernelArgs configuration resource defined for cluster/machine set
// to the MachineExtraKernelArgs resource for each machine.
type MachineExtraKernelArgsController struct {
	machineCustomizationController[*omni.ExtraKernelArgsConfiguration, *omni.MachineExtraKernelArgs]
}

// NewMachineExtraKernelArgsController initializes MachineExtraKernelArgsController.
func NewMachineExtraKernelArgsController() *MachineExtraKernelArgsController {
	return &MachineExtraKernelArgsController{
		machineCustomizationController: machineCustomizationController[*omni.ExtraKernelArgsConfiguration, *omni.MachineExtraKernelArgs]{
			NamedController: generic.NamedController{
				ControllerName: MachineExtraKernelArgsControllerName,
			},
			configLabel: omni.ExtraKernelArgsConfigurationLabel,
			newFunc: func(id resource.ID) *omni.MachineExtraKernelArgs {
				return omni.NewMachineExtraKernelArgs(resources.DefaultNamespace, id)
			},
			modifyFunc: func(i *omni.ExtraKernelArgsConfiguration, r *omni.MachineExtraKernelArgs) {
				r.TypedSpec().Value.Args = i.TypedSpec().Value.Args
			},
		},
	}
}

// MachineExtensionsControllerName is the name of the MachineExtensionsController.
const MachineExtensionsControllerName = "MachineExtensionsController"

// MachineExtensionsController splits a single extensions configuration resource defined for cluster/machine set
// to the MachineExtensions resource for each machine.
type MachineExtensionsController struct {
	machineCustomizationController[*omni.ExtensionsConfiguration, *omni.MachineExtensions]
}

// NewMachineExtensionsController initializes MachineExtensionsController.
func NewMachineExtensionsController() *MachineExtensionsController {
	return &MachineExtensionsController{
		machineCustomizationController: machineCustomizationController[*omni.ExtensionsConfiguration, *omni.MachineExtensions]{
			NamedController: generic.NamedController{
				ControllerName: MachineExtensionsControllerName,
			},
			configLabel: omni.ExtensionsConfigurationLabel,
			newFunc: func(id resource.ID) *omni.MachineExtensions {
				return omni.NewMachineExtensions(resources.DefaultNamespace, id)
			},
			modifyFunc: func(i *omni.ExtensionsConfiguration, r *omni.MachineExtensions) {
				r.TypedSpec().Value.Extensions = i.TypedSpec().Value.Extensions
			},
		},
	}
}

type machineCustomizationController[I, O generic.ResourceWithRD] struct {
	newFunc    func(resource.ID) O
	modifyFunc func(I, O)
	generic.NamedController
	configLabel string
}

// Settings implements controller.QController interface.
func (ctrl *machineCustomizationController[I, O]) Settings() controller.QSettings {
	var (
		input      I
		output     O
		inputType  = input.ResourceDefinition().Type
		outputType = output.ResourceDefinition().Type
	)

	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      inputType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      outputType,
				Kind:      controller.InputQMappedDestroyReady,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputExclusive,
				Type: outputType,
			},
		},
		Concurrency: optional.Some[uint](4),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *machineCustomizationController[I, O]) MapInput(ctx context.Context, _ *zap.Logger, r controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	res, err := r.Get(ctx, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	var (
		output     O
		outputType = output.ResourceDefinition().Type
	)

	switch ptr.Type() {
	case outputType:
		clusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return nil, nil
		}

		var list safe.List[I]

		list, err = safe.ReaderListAll[I](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		if err != nil {
			return nil, err
		}

		resources := make([]resource.Pointer, 0, list.Len())

		list.ForEach(func(r I) {
			resources = append(resources, r.Metadata())
		})

		return resources, nil
	case omni.ClusterMachineType:
		clusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return nil, fmt.Errorf("cluster machine %q doesn't have cluster label set", res.Metadata().ID())
		}

		machineSet, ok := res.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			return nil, fmt.Errorf("cluster machine %q doesn't have machine set label set", res.Metadata().ID())
		}

		for _, queries := range [][]resource.LabelQueryOption{
			{
				resource.LabelEqual(omni.LabelClusterMachine, res.Metadata().ID()),
			},
			{
				resource.LabelEqual(omni.LabelMachineSet, machineSet),
			},
			{
				resource.LabelEqual(omni.LabelCluster, clusterName),
				resource.LabelExists(omni.LabelClusterMachine, resource.NotMatches),
				resource.LabelExists(omni.LabelMachineSet, resource.NotMatches),
			},
		} {
			var matching safe.List[I]

			matching, err = safe.ReaderListAll[I](ctx, r, state.WithLabelQuery(queries...))
			if err != nil {
				return nil, err
			}

			if matching.Len() == 0 {
				continue
			}

			resources := make([]resource.Pointer, 0, matching.Len())

			for i := range matching.Len() {
				resources = append(resources, matching.Get(i).Metadata())
			}

			return resources, nil
		}

		return nil, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *machineCustomizationController[I, O]) Reconcile(ctx context.Context,
	_ *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	var (
		output     O
		outputType = output.ResourceDefinition().Type
	)

	configuration, err := safe.ReaderGetByID[I](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	tracker := trackResource(r, resources.DefaultNamespace, outputType, state.WithLabelQuery(
		resource.LabelEqual(ctrl.configLabel, configuration.Metadata().ID()),
	))

	clusterMachines, err := ctrl.getRelatedClusterMachines(ctx, r, configuration)
	if err != nil {
		return err
	}

	if configuration.Metadata().Phase() == resource.PhaseTearingDown {
		return tracker.cleanup(ctx, withDestroyReadyCallback(func() error {
			return r.RemoveFinalizer(ctx, configuration.Metadata(), ctrl.Name())
		}))
	}

	if !configuration.Metadata().Finalizers().Has(ctrl.Name()) {
		if err = r.AddFinalizer(ctx, configuration.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	for _, clusterMachine := range clusterMachines {
		res := ctrl.newFunc(clusterMachine.Metadata().ID())

		tracker.keep(res)

		if err = safe.WriterModify[O](ctx, r, res, func(r O) error {
			r.Metadata().Labels().Set(ctrl.configLabel, configuration.Metadata().ID())
			helpers.CopyLabels(clusterMachine, r, omni.LabelCluster)
			ctrl.modifyFunc(configuration, r)

			return nil
		}); err != nil {
			if state.IsPhaseConflictError(err) {
				return controller.NewRequeueError(err, time.Millisecond*100)
			}

			return err
		}
	}

	return tracker.cleanup(ctx)
}

func (ctrl *machineCustomizationController[I, O]) getRelatedClusterMachines(ctx context.Context,
	r controller.QRuntime, configuration I,
) ([]*omni.ClusterMachine, error) {
	for _, label := range []string{
		omni.LabelClusterMachine,
		omni.LabelMachineSet,
		omni.LabelCluster,
	} {
		value, found := configuration.Metadata().Labels().Get(label)
		if !found {
			continue
		}

		if label == omni.LabelClusterMachine {
			clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, value)
			if err != nil {
				if state.IsNotFoundError(err) {
					return nil, nil
				}

				return nil, err
			}

			return []*omni.ClusterMachine{clusterMachine}, nil
		}

		clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(resource.LabelEqual(label, value)))
		if err != nil {
			return nil, err
		}

		res := make([]*omni.ClusterMachine, 0, clusterMachines.Len())

		clusterMachines.ForEach(func(r *omni.ClusterMachine) {
			res = append(res, r)
		})

		return res, nil
	}

	return nil, nil
}
