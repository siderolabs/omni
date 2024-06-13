// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package token_test

import (
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/siderolabs/omni/internal/backend/oidc/external"
)

type mockTokenRequest struct{}

func (mockTokenRequest) GetAudience() []string {
	return []string{"test"}
}

func (mockTokenRequest) GetScopes() []string {
	return []string{
		oidc.ScopeOpenID,
		external.ScopeClusterPrefix + "cluster1",
	}
}

func (mockTokenRequest) GetSubject() string {
	return "some@example.com"
}
