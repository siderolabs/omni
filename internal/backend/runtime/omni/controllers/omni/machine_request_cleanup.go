// Copyright (c) 2026 Sidero Labs, Inc.
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

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	customcleanup "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/cleanup"
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
				customcleanup.NewHandler[*infra.MachineRequestStatus, *omni.MachineSetNode](
					forMachinesOfRequest(func(ctx context.Context, r controller.Runtime, req *infra.MachineRequestStatus, machineID resource.ID) error {
						return customcleanup.IDHandleFunc[*infra.MachineRequestStatus, *omni.MachineSetNode](func(*infra.MachineRequestStatus) resource.ID {
							return machineID
						}, true)(ctx, r, req)
					}),
					customcleanup.HandlerOptions{},
				),
				customcleanup.NewHandler[*infra.MachineRequestStatus, *omni.ClusterMachine](
					forMachinesOfRequest(func(ctx context.Context, r controller.Runtime, _ *infra.MachineRequestStatus, machineID resource.ID) error {
						_, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machineID)
						if err != nil {
							if state.IsNotFoundError(err) {
								return nil
							}

							return err
						}

						return xerrors.NewTaggedf[cleanup.SkipReconcileTag]("cluster machine is still present")
					}),
					customcleanup.HandlerOptions{
						NoOutputs: true,
					},
				),
				customcleanup.NewHandler[*infra.MachineRequestStatus, *siderolink.Link](
					forMachinesOfRequest(func(ctx context.Context, r controller.Runtime, _ *infra.MachineRequestStatus, machineID resource.ID) error {
						_, err := r.Teardown(ctx, siderolink.NewLink(machineID, nil).Metadata(), controller.WithOwner(""))
						if err != nil {
							if state.IsNotFoundError(err) {
								return nil
							}

							return err
						}

						return nil
					}),
					customcleanup.HandlerOptions{},
				),
			),
		},
	)
}

// forMachinesOfRequest runs the cleanup step for each machine which is bound to the machine request.
//
// The machines are found by the machine request label on their links, and not by the UUID in the status, which is reported by the infra provider.
func forMachinesOfRequest(
	step func(ctx context.Context, r controller.Runtime, req *infra.MachineRequestStatus, machineID resource.ID) error,
) customcleanup.HandlerFunc[*infra.MachineRequestStatus] {
	return func(ctx context.Context, r controller.Runtime, req *infra.MachineRequestStatus) error {
		// uncached: with a stale cache which misses the link, the cleanup would be skipped for good
		machineIDs, err := machinesOfRequest(ctx, r.ListUncached, req.Metadata().ID())
		if err != nil {
			return err
		}

		for _, machineID := range machineIDs {
			if err = step(ctx, r, req, machineID); err != nil {
				return err
			}
		}

		return nil
	}
}
