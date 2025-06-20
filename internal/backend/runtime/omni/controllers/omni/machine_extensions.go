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

// MachineExtensionsControllerName is the name of the MachineExtensionsController.
const MachineExtensionsControllerName = "MachineExtensionsController"

// MachineExtensionsController splits a single extensions configuration resource defined for cluster/machine set
// to the MachineExtensions resource for each machine.
type MachineExtensionsController struct {
	generic.NamedController
}

// NewMachineExtensionsController initializes MachineExtensionsController.
func NewMachineExtensionsController() *MachineExtensionsController {
	return &MachineExtensionsController{
		NamedController: generic.NamedController{
			ControllerName: MachineExtensionsControllerName,
		},
	}
}

// Settings implements controller.QController interface.
func (ctrl *MachineExtensionsController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ExtensionsConfigurationType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineExtensionsType,
				Kind:      controller.InputQMappedDestroyReady,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputExclusive,
				Type: omni.MachineExtensionsType,
			},
		},
		Concurrency: optional.Some[uint](4),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineExtensionsController) MapInput(ctx context.Context, _ *zap.Logger,
	r controller.QRuntime, ptr resource.Pointer,
) ([]resource.Pointer, error) {
	res, err := r.Get(ctx, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	switch ptr.Type() {
	case omni.MachineExtensionsType:
		clusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return nil, nil
		}

		var list safe.List[*omni.ExtensionsConfiguration]

		list, err = safe.ReaderListAll[*omni.ExtensionsConfiguration](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		if err != nil {
			return nil, err
		}

		resources := make([]resource.Pointer, 0, list.Len())

		list.ForEach(func(r *omni.ExtensionsConfiguration) {
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
			var matching safe.List[*omni.ExtensionsConfiguration]

			matching, err = safe.ReaderListAll[*omni.ExtensionsConfiguration](ctx, r, state.WithLabelQuery(queries...))
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
func (ctrl *MachineExtensionsController) Reconcile(ctx context.Context,
	_ *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	configuration, err := safe.ReaderGetByID[*omni.ExtensionsConfiguration](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	tracker := trackResource(r, resources.DefaultNamespace, omni.MachineExtensionsType, state.WithLabelQuery(
		resource.LabelEqual(omni.ExtensionsConfigurationLabel, configuration.Metadata().ID()),
	))

	clusterMachines, err := ctrl.getRelatedClusterMachines(ctx, r, configuration)
	if err != nil {
		return err
	}

	if configuration.Metadata().Phase() == resource.PhaseTearingDown {
		return tracker.cleanup(ctx, withDestroyReadyCallback(func() error {
			return r.RemoveFinalizer(ctx, configuration.Metadata(), MachineExtensionsControllerName)
		}))
	}

	if !configuration.Metadata().Finalizers().Has(MachineExtensionsControllerName) {
		if err = r.AddFinalizer(ctx, configuration.Metadata(), MachineExtensionsControllerName); err != nil {
			return err
		}
	}

	for _, clusterMachine := range clusterMachines {
		status := omni.NewMachineExtensions(resources.DefaultNamespace, clusterMachine.Metadata().ID())

		tracker.keep(status)

		if err = safe.WriterModify[*omni.MachineExtensions](ctx, r, status, func(r *omni.MachineExtensions) error {
			r.TypedSpec().Value.Extensions = configuration.TypedSpec().Value.Extensions
			r.Metadata().Labels().Set(omni.ExtensionsConfigurationLabel, configuration.Metadata().ID())

			helpers.CopyLabels(clusterMachine, r, omni.LabelCluster)

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

func (ctrl *MachineExtensionsController) getRelatedClusterMachines(ctx context.Context,
	r controller.QRuntime, configuration *omni.ExtensionsConfiguration,
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
