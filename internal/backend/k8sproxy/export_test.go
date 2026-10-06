// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy

import "net/http/httputil"

// ClusterContextKey is exposed for testing.
type ClusterContextKey = clusterContextKey

// Claims is exposed for testing.
type Claims = claims

// ForwardHeaders is exposed for testing.
var ForwardHeaders = forwardHeaders

// Rewrite is exposed for testing.
func Rewrite(req *httputil.ProxyRequest) {
	(&proxyHandler{}).rewrite(req)
}
