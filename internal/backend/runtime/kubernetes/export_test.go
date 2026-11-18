// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"
)

// CheckAccess exposes the access check of the runtime to external tests, on a runtime which has only the Omni state.
func CheckAccess(ctx context.Context, omniState state.State, clusterID string) error {
	return (&Runtime{state: omniState}).checkAccess(ctx, clusterID)
}
