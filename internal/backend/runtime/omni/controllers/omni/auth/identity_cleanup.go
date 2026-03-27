// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	customcleanup "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/cleanup"
)

// IdentityCleanupController cleans up IdentityLastActive resources when the corresponding Identity is removed.
type IdentityCleanupController = cleanup.Controller[*authres.Identity]

// NewIdentityCleanupController returns a new IdentityCleanup controller.
func NewIdentityCleanupController() *IdentityCleanupController {
	return cleanup.NewController(
		cleanup.Settings[*authres.Identity]{
			Name:    "IdentityCleanupController",
			Handler: &customcleanup.SameIDHandler[*authres.Identity, *authres.IdentityLastActive]{},
		},
	)
}
