// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package factory provides the deprecated image factory proxy endpoint.
package factory

import (
	"io"
	"net/http"
)

const deprecatedDownloadMessage = "Omni no longer proxies Talos installation media downloads. " +
	"Please upgrade omnictl to a recent version; newer omnictl downloads directly from the Talos image factory."

// Handler responds to deprecated image proxy requests.
type Handler struct{}

// NewHandler creates a new deprecated factory proxy handler.
func NewHandler() *Handler {
	return &Handler{}
}

// ServeHTTP handles deprecated image proxy requests.
func (*Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body.Close() //nolint:errcheck

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusGone)

	if r.Method == http.MethodHead {
		return
	}

	io.WriteString(w, deprecatedDownloadMessage+"\n") //nolint:errcheck
}
