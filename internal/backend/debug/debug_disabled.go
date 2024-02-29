// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build !sidero.debug

package debug

import "net/http"

// ServeHTTP handles debug requests.
func (handler *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	// no debug handlers when debug is disabled
	http.Error(w, "Not found", http.StatusNotFound)
}
