// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/runtime"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
)

func TestGatewayForwardsMetadataHeaders(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	st, err := omniruntime.NewTestState(logger)
	require.NoError(t, err)

	// no runtimes, so a request that carries the runtime header fails with "unknown runtime" instead of "missing runtime metadata"
	resourceServer := grpcomni.NewResourceServer(st.Default(), map[string]runtime.Runtime{}, nil, nil)

	servers := func(yield func(grpcomni.ServiceServer, error) bool) {
		yield(resourceServer, nil)
	}

	mux := http.NewServeMux()

	transport, err := grpcomni.RegisterGateway(t.Context(), servers, mux, logger)
	require.NoError(t, err)

	grpcServer, err := grpcomni.NewServer(servers)
	require.NoError(t, err)

	listener, err := transport.Listener()
	require.NoError(t, err)

	go grpcServer.Serve(listener) //nolint:errcheck

	t.Cleanup(grpcServer.Stop)

	httpServer := httptest.NewServer(mux)
	t.Cleanup(httpServer.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, httpServer.URL+"/api/omni.resources.ResourceService/Get",
		strings.NewReader(`{"namespace":"default","type":"Machines.omni.sidero.dev","id":"test"}`))
	require.NoError(t, err)

	req.Header.Set("Grpc-Metadata-"+message.RuntimeHeaderKey, "Omni")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Contains(t, string(body), `unknown runtime \"Omni\"`)
}
