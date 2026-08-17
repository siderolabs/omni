// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package saml

import "context"

// EnsureUser exposes ensureUser to the external test package.
func (sp *SessionProvider) EnsureUser(ctx context.Context, email string, samlLabels map[string]string) error {
	return sp.ensureUser(ctx, email, samlLabels)
}
