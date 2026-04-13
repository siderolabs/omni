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

// PublicKeyCleanupController cleans up PublicKeyLastActive resources when the corresponding PublicKey is removed.
type PublicKeyCleanupController = cleanup.Controller[*authres.PublicKey]

// NewPublicKeyCleanupController returns a new PublicKeyCleanup controller.
func NewPublicKeyCleanupController() *PublicKeyCleanupController {
	return cleanup.NewController(
		cleanup.Settings[*authres.PublicKey]{
			Name:    "PublicKeyCleanupController",
			Handler: &customcleanup.SameIDHandler[*authres.PublicKey, *authres.PublicKeyLastActive]{},
		},
	)
}
