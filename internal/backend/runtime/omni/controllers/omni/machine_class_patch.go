// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineClassPatchControllerName is the name of the MachineClassPatchController.
const MachineClassPatchControllerName = "MachineClassPatchController"

// MachineClassPatchController builds mapping MachineSetNode -> MachineClass.
type MachineClassPatchController struct {
	generic.NamedController
}

// NewMachineClassPatchController initializes MachineClassPatchController.
func NewMachineClassPatchController() *MachineClassPatchController {
	return &MachineClassPatchController{
		NamedController: generic.NamedController{
			ControllerName: MachineClassPatchControllerName,
		},
	}
}

// Settings implements controller.QController interface.
func (ctrl *MachineClassPatchController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ConfigPatchType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineClassType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineStatusType,
				Kind:      controller.InputQMapped,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputShared,
				Type: omni.ConfigPatchType,
			},
		},
		Concurrency: optional.Some[uint](8),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineClassPatchController) MapInput(ctx context.Context, _ *zap.Logger,
	r controller.QRuntime, ptr resource.Pointer,
) ([]resource.Pointer, error) {
	_, err := r.Get(ctx, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}
	}

	switch ptr.Type() {
	case omni.MachineClassType:
		patches, err := r.List(ctx, omni.NewConfigPatch(resources.DefaultNamespace, "").Metadata(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineClass, ptr.ID())))
		if err != nil {
			return nil, err
		}

		return xslices.Map(patches.Items, func(r resource.Resource) resource.Pointer {
			return r.Metadata()
		}), nil
	case omni.MachineStatusType:
		machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, ptr.ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, nil
			}

			return nil, err
		}

		// ignore not allocated machines
		if _, ok := machineStatus.Metadata().Labels().Get(omni.LabelCluster); !ok {
			return nil, nil
		}

		patches, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, r, state.WithLabelQuery(resource.LabelExists(omni.LabelMachineClass)))
		if err != nil {
			return nil, err
		}

		res := make([]resource.Pointer, 0, patches.Len())

		patches.ForEach(func(r *omni.ConfigPatch) {
			res = append(res, r.Metadata())
		})

		return res, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *MachineClassPatchController) Reconcile(ctx context.Context,
	_ *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	configPatch, err := safe.ReaderGetByID[*omni.ConfigPatch](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if configPatch.Metadata().Owner() != "" {
		return nil
	}

	machineClassName, ok := configPatch.Metadata().Labels().Get(omni.LabelMachineClass)
	if !ok {
		return nil
	}

	machineClass, err := helpers.HandleInput[*omni.MachineClass](ctx, r, MachineClassPatchControllerName, configPatch,
		helpers.WithID(machineClassName),
	)
	if err != nil {
		return err
	}

	tracker := trackResource(r, resources.DefaultNamespace, omni.ConfigPatchType, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelConfigPatchClass, configPatch.Metadata().ID()),
	))

	allMachineStatuses, err := safe.ReaderListAll[*omni.MachineStatus](ctx, r)
	if err != nil {
		return err
	}

	if err = allMachineStatuses.ForEachErr(func(res *omni.MachineStatus) error {
		if res.Metadata().Phase() == resource.PhaseTearingDown {
			if err = r.RemoveFinalizer(ctx, res.Metadata(), MachineClassPatchControllerName); err != nil {
				return err
			}
		}

		if !res.Metadata().Finalizers().Has(MachineClassPatchControllerName) {
			return r.AddFinalizer(ctx, res.Metadata(), MachineClassPatchControllerName)
		}

		return nil
	}); err != nil {
		return err
	}

	if configPatch.Metadata().Phase() == resource.PhaseTearingDown || machineClass == nil {
		return tracker.cleanup(ctx, withDestroyReadyCallback(func() error {
			return r.RemoveFinalizer(ctx, configPatch.Metadata(), MachineClassPatchControllerName)
		}))
	}

	if !configPatch.Metadata().Finalizers().Has(MachineClassPatchControllerName) {
		if err = r.AddFinalizer(ctx, configPatch.Metadata(), MachineClassPatchControllerName); err != nil {
			return err
		}
	}

	queries, err := labels.ParseSelectors(machineClass.TypedSpec().Value.MatchLabels)
	if err != nil {
		return err
	}

	for _, query := range queries {
		query.Terms = append(query.Terms, resource.LabelTerm{
			Key: omni.LabelCluster,
			Op:  resource.LabelOpExists,
		})

		machineStatuses := allMachineStatuses.FilterLabelQuery(
			resource.RawLabelQuery(query),
		)

		if err != nil {
			return err
		}

		if err = machineStatuses.ForEachErr(func(machineStatus *omni.MachineStatus) error {
			patch := omni.NewConfigPatch(resources.DefaultNamespace, configPatch.Metadata().ID()+"-"+machineStatus.Metadata().ID())

			tracker.keep(patch)

			return safe.WriterModify(ctx, r, patch, func(r *omni.ConfigPatch) error {
				r.TypedSpec().Value.Data = configPatch.TypedSpec().Value.Data

				r.Metadata().Labels().Set(omni.LabelConfigPatchClass, configPatch.Metadata().ID())
				r.Metadata().Labels().Set(omni.LabelClusterMachineClassPatch, machineStatus.Metadata().ID())
				r.Metadata().Labels().Set(omni.LabelSystemPatch, "")
				r.Metadata().Annotations().Set(omni.ConfigPatchDescription, fmt.Sprintf("This patch is automatically generated from %q", configPatch.Metadata().ID()))

				helpers.CopyLabels(machineStatus, r, omni.LabelCluster)

				return nil
			})
		}); err != nil {
			return err
		}
	}

	return tracker.cleanup(ctx)
}
