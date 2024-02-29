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
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
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
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, link *siderolink.Link, machine *omni.Machine) error {
				// convert SideroLink subnet to an IP address
				ipPrefix, err := netip.ParsePrefix(link.TypedSpec().Value.NodeSubnet)
				if err != nil {
					return err
				}

				spec := machine.TypedSpec().Value

				spec.ManagementAddress = ipPrefix.Addr().String()
				spec.Connected = link.TypedSpec().Value.Connected

				machine.Metadata().Labels().Set(omni.MachineAddressLabel, spec.ManagementAddress)

				return nil
			},
		},
		qtransform.WithConcurrency(4),
	)
}
