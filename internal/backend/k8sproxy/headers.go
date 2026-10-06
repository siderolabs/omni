// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy

import (
	"net/http"

	"github.com/siderolabs/gen/xslices"
	"k8s.io/client-go/transport"
)

// forwardedHeaders are the request headers passed on to the apiserver, in the canonical form a parsed
// request carries them in. Everything else is dropped, so that the caller decides nothing about who
// the apiserver takes them to be.
//
// It is an allow list and not a block list because the header names an apiserver trusts for
// authentication are configurable per cluster, so there is no fixed set of names to drop instead.
var forwardedHeaders = xslices.ToSet([]string{
	"Accept",
	"Accept-Encoding",
	"Content-Type",
	"If-Modified-Since",
	"If-None-Match",
	"User-Agent",

	// kubectl exec, attach, port-forward and cp upgrade the connection. ReverseProxy sets Connection
	// and Upgrade itself before it calls the rewrite, so what arrives here is its value.
	"Connection",
	"Origin",
	"Sec-Websocket-Key",
	"Sec-Websocket-Protocol",
	"Sec-Websocket-Version",
	"Upgrade",
	"X-Stream-Protocol-Version",
})

// forwardHeaders returns the headers the apiserver is sent, built from the outbound request and the
// impersonation the middleware put on the inbound one.
func forwardHeaders(outbound, inbound http.Header) http.Header {
	forwarded := make(http.Header, len(outbound))

	for name, values := range outbound {
		if _, ok := forwardedHeaders[name]; ok {
			forwarded[name] = values
		}
	}

	for _, header := range []string{transport.ImpersonateUserHeader, transport.ImpersonateGroupHeader} {
		for _, value := range inbound.Values(header) {
			forwarded.Add(header, value)
		}
	}

	return forwarded
}
