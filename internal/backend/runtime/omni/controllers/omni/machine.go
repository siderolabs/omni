// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineController creates omni.Machines based on siderolink.Link resources.
//
// MachineController plays the role of machine discovery.
type MachineController = qtransform.QController[*siderolink.Link, *omni.Machine]

// NewMachineController instantiates the machine controller.
func NewMachineController() *MachineController {
	helper := &machineControllerHelper{}

	return qtransform.NewQController(
		qtransform.Settings[*siderolink.Link, *omni.Machine]{
			Name: "MachineController",
			MapMetadataFunc: func(link *siderolink.Link) *omni.Machine {
				return omni.NewMachine(resources.DefaultNamespace, link.Metadata().ID())
			},
			UnmapMetadataFunc: func(machine *omni.Machine) *siderolink.Link {
				return siderolink.NewLink(resources.DefaultNamespace, machine.Metadata().ID(), nil)
			},
			TransformFunc: helper.transform,
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*omni.MachineRequestSet](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*infra.ProviderStatus](),
		),
		qtransform.WithExtraMappedInput(
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, infraMachine *infra.Machine) ([]resource.Pointer, error) {
				ptr := siderolink.NewLink(resources.DefaultNamespace, infraMachine.Metadata().ID(), nil).Metadata()

				return []resource.Pointer{ptr}, nil
			},
		),
		qtransform.WithConcurrency(4),
	)
}

type machineControllerHelper struct{}

func (h *machineControllerHelper) transform(ctx context.Context, r controller.Reader, _ *zap.Logger, link *siderolink.Link, machine *omni.Machine) error {
	if err := h.handleInfraProvider(ctx, r, link, machine); err != nil {
		return err
	}

	// convert SideroLink subnet to an IP address
	ipPrefix, err := netip.ParsePrefix(link.TypedSpec().Value.NodeSubnet)
	if err != nil {
		return err
	}

	helpers.CopyLabels(link, machine, omni.LabelMachineRequest, omni.LabelMachineRequestSet)

	spec := machine.TypedSpec().Value

	spec.ManagementAddress = ipPrefix.Addr().String()
	spec.Connected = link.TypedSpec().Value.Connected

	machine.Metadata().Labels().Set(omni.MachineAddressLabel, spec.ManagementAddress)

	return nil
}

// handleInfraProvider checks if the link has an infra provider ID annotation, and if so, runs the infra provider handling logic - static or provisioning (regular).
func (h *machineControllerHelper) handleInfraProvider(ctx context.Context, r controller.Reader, link *siderolink.Link, machine *omni.Machine) error {
	infraProviderID, ok := link.Metadata().Annotations().Get(omni.LabelInfraProviderID)
	if !ok { // not created by an infra provider, skip
		return nil
	}

	providerStatus, err := safe.ReaderGetByID[*infra.ProviderStatus](ctx, r, infraProviderID)
	if err != nil {
		return err
	}

	if _, isStaticProvider := providerStatus.Metadata().Labels().Get(omni.LabelIsStaticInfraProvider); isStaticProvider { // static infra provider flow, e.g., bare-metal
		return h.handleStaticInfraProvider(ctx, r, machine)
	}

	// provisioning (regular) infra provider flow, e.g., kubevirt
	return h.handleProvisioningInfraProvider(ctx, r, link, machine)
}

// handleStaticInfraProvider checks if the machine managed by a static infra provider was accepted by the user.
func (h *machineControllerHelper) handleStaticInfraProvider(ctx context.Context, r controller.Reader, machine *omni.Machine) error {
	machine.Metadata().Labels().Set(omni.LabelIsManagedByStaticInfraProvider, "")

	infraMachine, err := safe.ReaderGetByID[*infra.Machine](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	accepted := infraMachine != nil && infraMachine.TypedSpec().Value.AcceptanceStatus == specs.InfraMachineConfigSpec_ACCEPTED

	if !accepted {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"the link is created by a static infra provider, but the machine is not yet accepted",
		)
	}

	return nil
}

// handleProvisioningInfraProvider checks if the machine managed by a provisioning infra provider has the machine request and machine request set labels set.
//
// It then matches the machine request set owner to the machine provision controller, and if it matches, sets the no manual allocation label on the machine.
func (h *machineControllerHelper) handleProvisioningInfraProvider(ctx context.Context, r controller.Reader, link *siderolink.Link, machine *omni.Machine) error {
	_, ok := link.Metadata().Labels().Get(omni.LabelMachineRequest)
	if !ok {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"the link is created by the infra provider, but doesn't have the machine request label yet",
		)
	}

	machineRequestSetID, ok := link.Metadata().Labels().Get(omni.LabelMachineRequestSet)
	if !ok {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"the link is created by the infra provider, but doesn't have the machine request set label yet",
		)
	}

	machineRequestSet, err := safe.ReaderGetByID[*omni.MachineRequestSet](ctx, r, machineRequestSetID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if machineRequestSet != nil && machineRequestSet.Metadata().Owner() == machineProvisionControllerName {
		machine.Metadata().Labels().Set(omni.LabelNoManualAllocation, "")
	}

	return nil
}
