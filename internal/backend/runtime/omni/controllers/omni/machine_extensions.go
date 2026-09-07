// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
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
		ControllerName: MachineExtensionsControllerName,
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
		Concurrency: optional.Some[uint](1), // the configurations of a cluster write the same outputs, a parallel reconcile can overwrite a newer list with an older one
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineExtensionsController) MapInput(ctx context.Context, _ *zap.Logger,
	r controller.QRuntime, ptr controller.ReducedResourceMetadata,
) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case omni.MachineExtensionsType:
		clusterName, ok := ptr.Labels().Get(omni.LabelCluster)
		if !ok {
			return nil, nil
		}

		list, err := safe.ReaderListAll[*omni.ExtensionsConfiguration](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		if err != nil {
			return nil, err
		}

		resources := make([]resource.Pointer, 0, list.Len())

		list.ForEach(func(r *omni.ExtensionsConfiguration) {
			resources = append(resources, r.Metadata())
		})

		return resources, nil
	case omni.ClusterMachineType:
		clusterName, ok := ptr.Labels().Get(omni.LabelCluster)
		if !ok {
			return nil, fmt.Errorf("cluster machine %q doesn't have cluster label set", ptr.ID())
		}

		machineSet, ok := ptr.Labels().Get(omni.LabelMachineSet)
		if !ok {
			return nil, fmt.Errorf("cluster machine %q doesn't have machine set label set", ptr.ID())
		}

		for _, queries := range [][]resource.LabelQueryOption{
			{
				resource.LabelEqual(omni.LabelClusterMachine, ptr.ID()),
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
			matching, err := safe.ReaderListAll[*omni.ExtensionsConfiguration](ctx, r, state.WithLabelQuery(queries...))
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
func (ctrl *MachineExtensionsController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
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

	tearingDown := configuration.Metadata().Phase() == resource.PhaseTearingDown

	if !tearingDown && !configuration.Metadata().Finalizers().Has(MachineExtensionsControllerName) {
		if err = r.AddFinalizer(ctx, configuration.Metadata(), MachineExtensionsControllerName); err != nil {
			return err
		}
	}

	var configs []*omni.ExtensionsConfiguration

	if cluster, ok := configuration.Metadata().Labels().Get(omni.LabelCluster); ok {
		var configList safe.List[*omni.ExtensionsConfiguration]

		configList, err = safe.ReaderListAll[*omni.ExtensionsConfiguration](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))
		if err != nil {
			return err
		}

		configs = xslices.Filter(slices.Collect(configList.All()), func(cfg *omni.ExtensionsConfiguration) bool {
			return cfg.Metadata().Phase() == resource.PhaseRunning // a configuration being deleted must not be picked
		})
	} else if !tearingDown {
		logger.Warn("extensions configuration doesn't have cluster label set")

		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("extensions configuration doesn't have cluster label set")
	}

	for _, clusterMachine := range clusterMachines {
		config := ctrl.matchExtensionsConfiguration(clusterMachine, configs)
		if config == nil {
			continue // destroyed by the cleanup below
		}

		status := omni.NewMachineExtensions(clusterMachine.Metadata().ID())

		if err = safe.WriterModify(ctx, r, status, func(r *omni.MachineExtensions) error {
			r.Metadata().Labels().Set(omni.ExtensionsConfigurationLabel, config.Metadata().ID())

			helpers.CopyLabels(clusterMachine, r, omni.LabelCluster)

			if !ctrl.shouldRecalculateExtensions(r, configs) {
				return nil
			}

			r.TypedSpec().Value.Extensions = config.TypedSpec().Value.Extensions

			return nil
		}); err != nil {
			if state.IsPhaseConflictError(err) {
				continue // being destroyed by the cleanup below, created again on the next reconcile
			}

			return err
		}

		tracker.keep(status)
	}

	if tearingDown {
		return tracker.cleanup(ctx, withDestroyReadyCallback(func() error {
			return r.RemoveFinalizer(ctx, configuration.Metadata(), MachineExtensionsControllerName)
		}))
	}

	return tracker.cleanup(ctx)
}

// shouldRecalculateExtensions is a workaround built to preserve the existing wrong order of extensions to prevent machines from being upgraded unexpectedly.
//
// It does it by a clever trick:
//
// - If the version is zero, it means this is a create operation - calculate the extensions again.
// - If the helpers.InputResourceVersionAnnotation were set AND they have changed on the update, calculate the extensions again.
// - If the input versions are not yet set and if this is an update operation, we want to keep the existing extensions, even if they were calculated incorrectly.
func (ctrl *MachineExtensionsController) shouldRecalculateExtensions(me *omni.MachineExtensions, configs []*omni.ExtensionsConfiguration) bool {
	_, versionsWereSet := me.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
	versionsAreUpdated := helpers.UpdateInputsVersions(me, configs...)

	isCreate := me.Metadata().Version().Value() == 0
	if isCreate {
		return true
	}

	if versionsWereSet && versionsAreUpdated {
		return true
	}

	return false
}

// matchExtensionsConfiguration finds the extensions configuration which applies to the given cluster machine.
// The levels are checked in the following order:
// 1. The configuration defined for the cluster machine itself.
// 2. The configuration defined for the machine set the cluster machine belongs to.
// 3. The configuration defined for the cluster the machine belongs to.
//
// If there are multiple configurations defined for the same level, the one with the lexicographically highest ID is used.
func (ctrl *MachineExtensionsController) matchExtensionsConfiguration(cm *omni.ClusterMachine, configs []*omni.ExtensionsConfiguration) *omni.ExtensionsConfiguration {
	cmID := cm.Metadata().ID()
	msID, _ := cm.Metadata().Labels().Get(omni.LabelMachineSet)
	clusterID, _ := cm.Metadata().Labels().Get(omni.LabelCluster)

	for _, labels := range []struct {
		labelName  string
		labelValue string
	}{
		{omni.LabelClusterMachine, cmID},
		{omni.LabelMachineSet, msID},
		{omni.LabelCluster, clusterID},
	} {
		config := ctrl.matchExtensionConfigByLabel(labels.labelName, labels.labelValue, configs)
		if config != nil {
			return config
		}
	}

	return nil
}

func (ctrl *MachineExtensionsController) matchExtensionConfigByLabel(labelName, labelValue string, configs []*omni.ExtensionsConfiguration) *omni.ExtensionsConfiguration {
	configs = xslices.Filter(configs, func(cfg *omni.ExtensionsConfiguration) bool {
		val, ok := cfg.Metadata().Labels().Get(labelName)

		// special handling for the cluster label - if the config has a cluster label, it should not have machine set or cluster machine labels
		if labelName == omni.LabelCluster {
			if _, wrongLevel := cfg.Metadata().Labels().Get(omni.LabelClusterMachine); wrongLevel {
				return false
			}

			if _, wrongLevel := cfg.Metadata().Labels().Get(omni.LabelMachineSet); wrongLevel {
				return false
			}
		}

		return ok && val == labelValue
	})

	if len(configs) == 0 {
		return nil
	}

	return configs[len(configs)-1]
}

func (ctrl *MachineExtensionsController) getRelatedClusterMachines(ctx context.Context, r controller.QRuntime, configuration *omni.ExtensionsConfiguration) ([]*omni.ClusterMachine, error) {
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
