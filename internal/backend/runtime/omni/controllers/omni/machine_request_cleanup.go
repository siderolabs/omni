// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineRequestStatusCleanupController manages MachineRequestStatusStatus resource lifecycle.
type MachineRequestStatusCleanupController = cleanup.Controller[*infra.MachineRequestStatus]

// NewMachineRequestStatusCleanupController returns a new MachineRequestStatusCleanup controller.
// This controller should remove all links for a tearing down machine request.
func NewMachineRequestStatusCleanupController() *MachineRequestStatusCleanupController {
	return cleanup.NewController(
		cleanup.Settings[*infra.MachineRequestStatus]{
			Name: "MachineRequestStatusCleanupController",
			Handler: cleanup.Combine(
				helpers.NewCustomHandler[*infra.MachineRequestStatus, *omni.MachineSetNode](
					helpers.MapID[*infra.MachineRequestStatus, *omni.MachineSetNode](func(req *infra.MachineRequestStatus) resource.ID {
						return req.TypedSpec().Value.Id
					}, true),
					false,
				),
				helpers.NewCustomHandler[*infra.MachineRequestStatus, *omni.ClusterMachine](
					func(ctx context.Context, r controller.Runtime, req *infra.MachineRequestStatus, _ string) error {
						_, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, req.TypedSpec().Value.Id)
						if err != nil {
							if state.IsNotFoundError(err) {
								return nil
							}

							return err
						}

						return xerrors.NewTaggedf[cleanup.SkipReconcileTag]("cluster machine is still present")
					},
					true,
				),
				helpers.NewCustomHandler[*infra.MachineRequestStatus, *siderolink.Link](
					func(ctx context.Context, r controller.Runtime, req *infra.MachineRequestStatus, _ string) error {
						_, err := r.Teardown(ctx, siderolink.NewLink(resources.DefaultNamespace, req.TypedSpec().Value.Id, nil).Metadata(), controller.WithOwner(""))
						if err != nil {
							if state.IsNotFoundError(err) {
								return nil
							}

							return err
						}

						return nil
					},
					false,
				),
			),
		},
	)
}
