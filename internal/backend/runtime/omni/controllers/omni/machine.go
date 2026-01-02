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
				return siderolink.NewLink(machine.Metadata().ID(), nil)
			},
			TransformFunc: helper.transform,
		},
		qtransform.WithExtraMappedInput[*omni.MachineRequestSet](
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*infra.MachineRequest](
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*infra.MachineRequestStatus](
			qtransform.MapperFuncFromTyped[*infra.MachineRequestStatus](
				func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, res *infra.MachineRequestStatus) ([]resource.Pointer, error) {
					if res.TypedSpec().Value.Id == "" {
						return nil, nil
					}

					return []resource.Pointer{
						siderolink.NewLink(res.TypedSpec().Value.Id, nil).Metadata(),
					}, nil
				},
			),
		),
		qtransform.WithExtraMappedInput[*infra.ProviderStatus](
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*infra.Machine](
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, infraMachine controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				ptr := siderolink.NewLink(infraMachine.ID(), nil).Metadata()

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

	helpers.CopyLabels(link, machine, omni.LabelMachineRequest, omni.LabelMachineRequestSet, omni.LabelInfraProviderID)
	helpers.CopyAnnotations(link, machine, omni.CreatedWithUniqueToken)

	spec := machine.TypedSpec().Value

	spec.ManagementAddress = ipPrefix.Addr().String()
	spec.Connected = link.TypedSpec().Value.Connected

	machine.Metadata().Labels().Set(omni.MachineAddressLabel, spec.ManagementAddress)

	machine.TypedSpec().Value.UseGrpcTunnel = link.TypedSpec().Value.VirtualAddrport != ""

	return nil
}

// handleInfraProvider checks if the link has an infra provider ID annotation, and if so, runs the infra provider handling logic - static or provisioning (regular).
func (h *machineControllerHelper) handleInfraProvider(ctx context.Context, r controller.Reader, link *siderolink.Link, machine *omni.Machine) error {
	infraProviderID, ok := link.Metadata().Labels().Get(omni.LabelInfraProviderID)
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
	machineRequestID, ok := link.Metadata().Labels().Get(omni.LabelMachineRequest)
	if !ok {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"the link is created by the infra provider, but doesn't have the machine request label yet",
		)
	}

	machineRequest, err := safe.ReaderGetByID[*infra.MachineRequest](ctx, r, machineRequestID)
	if err != nil {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"the link is created by the infra provider, but the machine request doesn't exist",
		)
	}

	machineRequestStatus, err := safe.ReaderGetByID[*infra.MachineRequestStatus](ctx, r, machineRequestID)
	if err != nil {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"the link is created by the infra provider, but the machine request status doesn't exist",
		)
	}

	if machineRequestStatus.TypedSpec().Value.Id == "" {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"waiting for the machine request status UUID to be populated",
		)
	}

	machineRequestSetID, ok := machineRequest.Metadata().Labels().Get(omni.LabelMachineRequestSet)
	if !ok {
		return nil
	}

	machineRequestSet, err := safe.ReaderGetByID[*omni.MachineRequestSet](ctx, r, machineRequestSetID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if machineRequestSet != nil && machineRequestSet.Metadata().Owner() == MachineProvisionControllerName {
		machine.Metadata().Labels().Set(omni.LabelNoManualAllocation, "")
	}

	machine.Metadata().Labels().Set(omni.LabelMachineRequestSet, machineRequestSetID)

	return nil
}
