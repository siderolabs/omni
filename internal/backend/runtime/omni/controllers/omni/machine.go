// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineController creates omni.Machines based on siderolink.Link resources.
//
// MachineController plays the role of machine discovery.
type MachineController = qtransform.QController[*siderolink.Link, *omni.Machine]

// NewMachineController instanciates the machine controller.
func NewMachineController() *MachineController {
	return qtransform.NewQController(
		qtransform.Settings[*siderolink.Link, *omni.Machine]{
			Name: "MachineController",
			MapMetadataFunc: func(link *siderolink.Link) *omni.Machine {
				return omni.NewMachine(resources.DefaultNamespace, link.Metadata().ID())
			},
			UnmapMetadataFunc: func(machine *omni.Machine) *siderolink.Link {
				return siderolink.NewLink(resources.DefaultNamespace, machine.Metadata().ID(), nil)
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, link *siderolink.Link, machine *omni.Machine) error {
				var machineRequestSetID string

				if _, ok := link.Metadata().Annotations().Get(omni.LabelInfraProviderID); ok {
					_, ok = link.Metadata().Labels().Get(omni.LabelMachineRequest)
					if !ok {
						return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
							"the link is created by the infra provider, but doesn't have the machine request label yet",
						)
					}

					machineRequestSetID, ok = link.Metadata().Labels().Get(omni.LabelMachineRequestSet)
					if !ok {
						return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
							"the link is created by the infra provider, but doesn't have the machine request label yet",
						)
					}
				}

				// convert SideroLink subnet to an IP address
				ipPrefix, err := netip.ParsePrefix(link.TypedSpec().Value.NodeSubnet)
				if err != nil {
					return err
				}

				helpers.CopyLabels(link, machine, omni.LabelMachineRequest, omni.LabelMachineRequestSet)

				if machineRequestSetID != "" {
					machineRequestSet, err := safe.ReaderGetByID[*omni.MachineRequestSet](ctx, r, machineRequestSetID)
					if err != nil && !state.IsNotFoundError(err) {
						return err
					}

					if machineRequestSet != nil && machineRequestSet.Metadata().Owner() == machineProvisionControllerName {
						machine.Metadata().Labels().Set(omni.LabelNoManualAllocation, "")
					}
				}

				spec := machine.TypedSpec().Value

				spec.ManagementAddress = ipPrefix.Addr().String()
				spec.Connected = link.TypedSpec().Value.Connected

				machine.Metadata().Labels().Set(omni.MachineAddressLabel, spec.ManagementAddress)

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*omni.MachineRequestSet](),
		),
		qtransform.WithConcurrency(4),
	)
}
