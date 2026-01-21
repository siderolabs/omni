// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineRequestLinkControllerName is the name of the MachineRequestLinkController.
const MachineRequestLinkControllerName = "MachineRequestLinkController"

// MachineRequestLinkController adds labels to the links if they are created by a MachineRequest.
type MachineRequestLinkController struct {
	state state.State
	generic.NamedController
}

// NewMachineRequestLinkController initializes MachineRequestLinkController.
func NewMachineRequestLinkController(state state.State) *MachineRequestLinkController {
	return &MachineRequestLinkController{
		NamedController: generic.NamedController{
			ControllerName: MachineRequestLinkControllerName,
		},
		state: state,
	}
}

// Settings implements controller.QController interface.
func (ctrl *MachineRequestLinkController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.InfraProviderNamespace,
				Type:      infra.MachineRequestStatusType,
				Kind:      controller.InputQPrimary,
			},
		},
		Concurrency: optional.Some[uint](4),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineRequestLinkController) MapInput(context.Context, *zap.Logger,
	controller.QRuntime, controller.ReducedResourceMetadata,
) ([]resource.Pointer, error) {
	return nil, nil
}

// Reconcile implements controller.QController interface.
func (ctrl *MachineRequestLinkController) Reconcile(ctx context.Context,
	_ *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	machineRequestStatus, err := safe.ReaderGet[*infra.MachineRequestStatus](ctx, r, infra.NewMachineRequestStatus(ptr.ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if machineRequestStatus.TypedSpec().Value.Id == "" {
		return nil
	}

	link := siderolink.NewLink(machineRequestStatus.TypedSpec().Value.Id, nil)

	_, err = safe.StateUpdateWithConflicts(ctx, ctrl.state, link.Metadata(), func(r *siderolink.Link) error {
		r.Metadata().Labels().Set(omni.LabelMachineRequest, machineRequestStatus.Metadata().ID())

		helpers.CopyLabels(machineRequestStatus, r, omni.LabelMachineRequestSet)

		return nil
	})
	if state.IsPhaseConflictError(err) {
		return nil
	}

	if state.IsNotFoundError(err) {
		return controller.NewRequeueError(err, time.Second*5)
	}

	return err
}
