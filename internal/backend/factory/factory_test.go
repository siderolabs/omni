// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package factory_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/factory"
)

func TestDeprecatedHandler(t *testing.T) {
	for _, tt := range []struct {
		method     string
		expectBody bool
	}{
		{
			method:     http.MethodGet,
			expectBody: true,
		},
		{
			method: http.MethodHead,
		},
		{
			method:     http.MethodPost,
			expectBody: true,
		},
	} {
		t.Run(tt.method, func(t *testing.T) {
			body := strings.NewReader("ignored")
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/image/schematic/v1.11.0/iso-amd64", body)
			resp := httptest.NewRecorder()

			factory.NewHandler().ServeHTTP(resp, req)

			require.Equal(t, http.StatusGone, resp.Code)
			require.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))

			if !tt.expectBody {
				require.Empty(t, resp.Body.String())

				return
			}

			require.Contains(t, resp.Body.String(), "Please upgrade omnictl to a recent version")
		})
	}
}
