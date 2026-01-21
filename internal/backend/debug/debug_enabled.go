// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build sidero.debug

package debug

import (
	"io"
	"net/http"
)

// ServeHTTP handles debug requests.
func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	io.Copy(io.Discard, r.Body) //nolint:errcheck
	r.Body.Close()              //nolint:errcheck

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		// supported
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	switch r.URL.Path {
	case "/debug/controller-graph":
		handler.handleDependencyGraph(w, r, false)
	case "/debug/controller-resource-graph":
		handler.handleDependencyGraph(w, r, true)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
