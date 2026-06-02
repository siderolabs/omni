// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package backend_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	backend "github.com/siderolabs/omni/internal/backend"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
)

func TestTalosctlHandlerRejectsInvalidVersion(t *testing.T) {
	var upstreamRequests atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		upstreamRequests.Add(1)

		require.Equal(t, "/talosctl/v1.2.3", req.URL.Path)

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		_, err := rw.Write([]byte(`["download-url"]`))
		require.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)

	imageFactoryClient, err := imagefactory.NewClient(upstream.URL, "", "")
	require.NoError(t, err)

	handler, err := backend.MakeTalosctlHandler(imageFactoryClient, zaptest.NewLogger(t))
	require.NoError(t, err)

	validReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/talosctl/downloads/v1.2.3", nil)
	validReq.SetPathValue("version", "v1.2.3")

	validResp := httptest.NewRecorder()
	handler.ServeHTTP(validResp, validReq)

	require.Equal(t, http.StatusOK, validResp.Code)
	require.Equal(t, int32(1), upstreamRequests.Load())

	invalidReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/talosctl/downloads/..%2fsecret", nil)
	invalidReq.SetPathValue("version", "../secret")

	invalidResp := httptest.NewRecorder()
	handler.ServeHTTP(invalidResp, invalidReq)

	require.Equal(t, http.StatusBadRequest, invalidResp.Code)
	require.Equal(t, int32(1), upstreamRequests.Load())
}
