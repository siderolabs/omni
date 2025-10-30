// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// LinkCleanupController manages LinkCleanup resource lifecycle.
type LinkCleanupController = cleanup.Controller[*siderolink.Link]

// NewLinkCleanupController returns a new LinkCleanup controller.
// Removes the corresponding JoinTokenUsage resource when the link is removed.
func NewLinkCleanupController() *LinkCleanupController {
	return cleanup.NewController(
		cleanup.Settings[*siderolink.Link]{
			Name: "LinkCleanupController",
			Handler: cleanup.Combine(
				&helpers.SameIDHandler[*siderolink.Link, *siderolink.JoinTokenUsage]{},
				&helpers.SameIDHandler[*siderolink.Link, *siderolink.NodeUniqueToken]{},
				&helpers.SameIDHandler[*siderolink.Link, *omni.MachineLabels]{},
			),
		},
	)
}
