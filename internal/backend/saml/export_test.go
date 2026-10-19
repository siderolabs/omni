// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package saml

import (
	"context"

	"github.com/siderolabs/omni/internal/pkg/auth/user"
)

// MatchSAMLLabelRule is exported for testing.
var MatchSAMLLabelRule = user.MatchSAMLLabelRule

// EnsureUser exposes the user sync to the external test package.
func (sp *SessionProvider) EnsureUser(ctx context.Context, email string, samlLabels map[string]string) error {
	return sp.syncer.Sync(ctx, email, samlLabels)
}
