// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// MachineCleanupController manages MachineCleanup resource lifecycle.
type MachineCleanupController = cleanup.Controller[*omni.Machine]

// NewMachineCleanupController returns a new MachineCleanup controller.
// This controller should remove all MachineSetNodes for a tearing down machine.
// If the MachineSetNode is owned by some controller, it is skipped.
func NewMachineCleanupController() *MachineCleanupController {
	return cleanup.NewController(
		cleanup.Settings[*omni.Machine]{
			Name:    "MachineCleanupController",
			Handler: &helpers.SameIDHandler[*omni.Machine, *omni.MachineSetNode]{},
		},
	)
}
