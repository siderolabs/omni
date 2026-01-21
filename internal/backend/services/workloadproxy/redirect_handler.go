// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/auth"
)

// NewRedirectHandler creates a new HTTP redirect handler.
func NewRedirectHandler(key []byte, logger *zap.Logger) (http.HandlerFunc, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			url string
			err error
		)

		if redirect := r.URL.Query().Get(auth.RedirectQueryParam); redirect != "" {
			url, err = DecodeRedirectURL(redirect, key)
			if err != nil {
				logger.Warn("failed to decode redirect query param", zap.Error(err))

				http.Redirect(w, r, "/badrequest", http.StatusSeeOther)

				return
			}
		}

		http.Redirect(w, r, url, http.StatusSeeOther)
	}), nil
}
