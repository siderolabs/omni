// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package auth contains authentication related logic.
package auth

import "time"

const (
	// ServiceAccountMaxAllowedLifetime is the maximum allowed lifetime for a service account.
	ServiceAccountMaxAllowedLifetime = 365 * 24 * time.Hour

	// RedirectQueryParam is the name of the query parameter used to specify URL or route to redirect after authentication flow is complete.
	//
	// tsgen:RedirectQueryParam
	RedirectQueryParam = "redirect"

	// FlowQueryParam is the name of the query parameter used to specify the authentication flow.
	//
	// tsgen:AuthFlowQueryParam
	FlowQueryParam = "flow"

	// CLIAuthFlow is the name of the authentication flow used for CLI authentication.
	//
	// tsgen:CLIAuthFlow
	CLIAuthFlow = "cli"

	// FrontendAuthFlow is the name of the authentication flow used for frontend authentication.
	//
	// tsgen:FrontendAuthFlow
	FrontendAuthFlow = "frontend"

	// ProxyAuthFlow is the name of the authentication flow used for proxy authentication.
	//
	// tsgen:WorkloadProxyAuthFlow
	ProxyAuthFlow = "workload-proxy"
)
