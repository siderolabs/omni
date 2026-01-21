// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// InfraMachineRegistrationController creates infra.InfraMachineRegistrations based on siderolink.Link resources.
type InfraMachineRegistrationController = qtransform.QController[*siderolink.Link, *infra.MachineRegistration]

// NewInfraMachineRegistrationController instantiates the controller.
func NewInfraMachineRegistrationController() *InfraMachineRegistrationController {
	return qtransform.NewQController(
		qtransform.Settings[*siderolink.Link, *infra.MachineRegistration]{
			Name: "InfraMachineRegistrationController",
			MapMetadataOptionalFunc: func(link *siderolink.Link) optional.Optional[*infra.MachineRegistration] {
				if _, ok := link.Metadata().Labels().Get(omni.LabelInfraProviderID); !ok {
					return optional.None[*infra.MachineRegistration]()
				}

				return optional.Some(infra.NewMachineRegistration(link.Metadata().ID()))
			},
			UnmapMetadataFunc: func(machine *infra.MachineRegistration) *siderolink.Link {
				return siderolink.NewLink(machine.Metadata().ID(), nil)
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, link *siderolink.Link, machine *infra.MachineRegistration) error {
				providerID, ok := link.Metadata().Labels().Get(omni.LabelInfraProviderID)
				if !ok {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("the link doesn't have the provider ID label")
				}

				helpers.CopyAllLabels(link, machine)

				machine.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

				return nil
			},
		},
	)
}
