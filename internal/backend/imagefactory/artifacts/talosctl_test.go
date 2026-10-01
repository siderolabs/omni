// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package artifacts_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/imagefactory/artifacts"
)

func TestTalosctlHandler(t *testing.T) {
	var upstreamRequests atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		upstreamRequests.Add(1)

		require.Equal(t, "/talosctl/1.2.3", req.URL.Path)

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		_, err := rw.Write([]byte(`["download-url"]`))
		require.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)

	imageFactoryClient, err := imagefactory.NewClient(upstream.URL, imagefactory.Auth{})
	require.NoError(t, err)

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	require.NoError(t, st.Create(t.Context(), omni.NewTalosVersion("1.2.3")))

	handler := artifacts.NewTalosctlHandler(st, imagefactory.NewClients(st, imageFactoryClient), zaptest.NewLogger(t))

	get := func(version string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/talosctl/downloads/"+version, nil)
		req.SetPathValue("version", version)

		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)

		return resp
	}

	require.Equal(t, http.StatusOK, get("1.2.3").Code)
	require.Equal(t, int32(1), upstreamRequests.Load())

	// second request for the same version is served from the cache
	require.Equal(t, http.StatusOK, get("1.2.3").Code)
	require.Equal(t, int32(1), upstreamRequests.Load())

	// a well-formed version Omni does not track never reaches the factory
	require.Equal(t, http.StatusNotFound, get("1.2.4").Code)
	require.Equal(t, http.StatusNotFound, get("v1.2.3").Code)
	require.Equal(t, http.StatusNotFound, get("1.2.3+build").Code)
	require.Equal(t, int32(1), upstreamRequests.Load())

	require.Equal(t, http.StatusBadRequest, get("../secret").Code)
	require.Equal(t, int32(1), upstreamRequests.Load())
}
