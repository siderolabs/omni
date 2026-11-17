// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
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
//
// It is the only place which binds a machine to a machine request. The machine UUID in the MachineRequestStatus is reported
// by the infra provider, so the link is labeled only when it carries the same infra provider ID.
// The other controllers find the machine of a machine request through that label, see machinesOfRequest.
type MachineRequestLinkController struct {
	state state.State
	generic.NamedController
}

// NewMachineRequestLinkController initializes MachineRequestLinkController.
func NewMachineRequestLinkController(state state.State) *MachineRequestLinkController {
	return &MachineRequestLinkController{
		ControllerName: MachineRequestLinkControllerName,
		state:          state,
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
	logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
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

	// the machine request set is taken from the machine request, which is created by Omni, and not from the status
	machineRequest, err := safe.StateGetByID[*infra.MachineRequest](ctx, ctrl.state, machineRequestStatus.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	_, err = safe.StateUpdateWithConflicts(ctx, ctrl.state, link.Metadata(), func(r *siderolink.Link) error {
		// a link which is bound already keeps its machine request, and is not checked again
		boundTo, bound := r.Metadata().Labels().Get(omni.LabelMachineRequest)

		switch {
		case bound && boundTo != machineRequestStatus.Metadata().ID():
			return errBoundToAnotherMachineRequest
		case !bound && !sameInfraProvider(machineRequestStatus, r):
			return errMachineOfAnotherProvider
		}

		r.Metadata().Labels().Set(omni.LabelMachineRequest, machineRequestStatus.Metadata().ID())

		if machineRequest != nil && sameInfraProvider(machineRequest, machineRequestStatus) {
			helpers.CopyLabels(machineRequest, r, omni.LabelMachineRequestSet)
		}

		return nil
	})
	if state.IsPhaseConflictError(err) {
		return nil
	}

	if errors.Is(err, errMachineOfAnotherProvider) || errors.Is(err, errBoundToAnotherMachineRequest) {
		logger.Warn("ignoring the machine reported in the machine request status", zap.String("machine", link.Metadata().ID()), zap.Error(err))

		return nil
	}

	if state.IsNotFoundError(err) {
		return controller.NewRequeueError(err, time.Second*5)
	}

	return err
}

var (
	errMachineOfAnotherProvider     = errors.New("machine of another infra provider")
	errBoundToAnotherMachineRequest = errors.New("machine is bound to another machine request")
)

// sameInfraProvider reports whether both resources carry the same infra provider ID.
func sameInfraProvider(a, b resource.Resource) bool {
	providerA, okA := a.Metadata().Labels().Get(omni.LabelInfraProviderID)
	providerB, okB := b.Metadata().Labels().Get(omni.LabelInfraProviderID)

	return okA && okB && providerA != "" && providerA == providerB
}

// machinesOfRequest returns the IDs of the machines which are bound to the machine request, i.e., whose links carry its label.
//
// The list function is the cached or the uncached one of the controller runtime.
func machinesOfRequest(
	ctx context.Context,
	list func(context.Context, resource.Kind, ...state.ListOption) (resource.List, error),
	machineRequestID resource.ID,
) ([]resource.ID, error) {
	links, err := list(ctx, siderolink.NewLink("", nil).Metadata(), state.WithLabelQuery(
		resource.LabelEqual(omni.LabelMachineRequest, machineRequestID),
	))
	if err != nil {
		return nil, err
	}

	return xslices.Map(links.Items, func(link resource.Resource) resource.ID { return link.Metadata().ID() }), nil
}
