// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package artifacts_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/internal/backend/imagefactory/artifacts"
)

// schematicID is a valid schematic ID: the hex-encoded SHA-256 of some schematic.
const schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

// upstream is a stand-in image factory recording the paths it was asked for. The response is served
// by respond, so a test can make the factory fail.
type upstream struct {
	respond func(http.ResponseWriter, *http.Request)
	paths   []string
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.paths = append(u.paths, r.URL.Path)

	if u.respond != nil {
		u.respond(w, r)

		return
	}

	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte(`{"artifact": true}`))
	if err != nil {
		panic(err)
	}
}

// serve routes a request through the artifact routes as they are registered in the real mux, so that
// the path values the handlers read are the ones Go's router produces for the request as written -
// escaping and all.
func serve(t *testing.T, factory *upstream, target string) *httptest.ResponseRecorder {
	t.Helper()

	return serveWithLogger(t, factory, target, zaptest.NewLogger(t))
}

func serveWithLogger(t *testing.T, factory *upstream, target string, logger *zap.Logger) *httptest.ResponseRecorder {
	t.Helper()

	server := httptest.NewServer(factory)
	t.Cleanup(server.Close)

	client, err := imagefactory.NewClient(server.URL, "", "")
	require.NoError(t, err)

	clients := imagefactory.NewClients(state.WrapCore(namespaced.NewState(inmem.Build)), client)

	mux := http.NewServeMux()
	mux.Handle("GET /api/vulns/{schematicID}/{talosVersion}/{arch}/{filename}", artifacts.NewScanHandler(clients, logger))
	mux.Handle("GET /api/sbom/{schematicID}/{talosVersion}/{arch}", artifacts.NewSbomHandler(clients, logger))
	mux.Handle("GET /api/vex/{talosVersion}", artifacts.NewVEXHandler(clients, logger))

	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))

	return resp
}

// errorStatus is the status message from an artifact error body.
func errorStatus(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()

	var body struct {
		Status string `json:"status"`
	}

	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))

	return body.Status
}

func TestArtifactsAreFetchedFromTheFactory(t *testing.T) {
	for _, test := range []struct {
		name         string
		target       string
		expectedPath string
		contentType  string
	}{
		{
			name:         "scan report",
			target:       "/api/vulns/" + schematicID + "/v1.13.0/amd64/report.json",
			expectedPath: "/scans/" + schematicID + "/v1.13.0/amd64/report.json",
			contentType:  "application/json",
		},
		{
			name:         "scan report in another format",
			target:       "/api/vulns/" + schematicID + "/v1.13.0/arm64/report.table",
			expectedPath: "/scans/" + schematicID + "/v1.13.0/arm64/report.table",
			contentType:  "text/plain; charset=utf-8",
		},
		{
			name:         "SPDX bundle",
			target:       "/api/sbom/" + schematicID + "/v1.13.0/amd64",
			expectedPath: "/spdx/" + schematicID + "/v1.13.0/amd64",
			contentType:  "application/spdx+json",
		},
		{
			name:         "VEX document",
			target:       "/api/vex/v1.13.0",
			expectedPath: "/vex/v1.13.0/vex.json",
			contentType:  "application/json",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &upstream{}

			resp := serve(t, factory, test.target)

			require.Equal(t, http.StatusOK, resp.Code, errorStatus(t, resp))
			require.Equal(t, []string{test.expectedPath}, factory.paths)
			require.Equal(t, test.contentType, resp.Header().Get("Content-Type"))
			require.Equal(t, `{"artifact": true}`, resp.Body.String())
		})
	}
}

// Every path value a route declares ends up in the factory URL, so one that could change the shape of
// that URL must be rejected before it gets there: an encoded separator survives routing, and the
// factory client cleans the "../" it forms, which would leave Omni's authenticated client pointed at
// some other factory endpoint. A traversal attempt is caught by the separator guard every path value
// goes through, ahead of whatever shape that particular value must have.
func TestInvalidPathValuesAreNotForwarded(t *testing.T) {
	for _, test := range []struct {
		name           string
		target         string
		expectedStatus string
	}{
		{
			name:           "traversal in the report filename",
			target:         "/api/vulns/" + schematicID + "/v1.13.0/amd64/..%2F..%2F..%2Fversions",
			expectedStatus: "invalid report filename: must not contain a path separator",
		},
		{
			name:           "unknown report filename",
			target:         "/api/vulns/" + schematicID + "/v1.13.0/amd64/report.yaml",
			expectedStatus: "invalid report filename",
		},
		{
			name:           "traversal in the architecture",
			target:         "/api/vulns/" + schematicID + "/v1.13.0/..%2F..%2Fversions/report.json",
			expectedStatus: "invalid architecture: must not contain a path separator",
		},
		{
			name:           "unknown architecture",
			target:         "/api/vulns/" + schematicID + "/v1.13.0/riscv64/report.json",
			expectedStatus: "invalid architecture",
		},
		{
			name:           "traversal in the schematic ID",
			target:         "/api/vulns/..%2F..%2Fversions/v1.13.0/amd64/report.json",
			expectedStatus: "invalid schematic ID: must not contain a path separator",
		},
		{
			name:           "schematic ID that is not a digest",
			target:         "/api/vulns/not-a-schematic/v1.13.0/amd64/report.json",
			expectedStatus: "invalid schematic ID",
		},
		{
			name:           "traversal in the Talos version",
			target:         "/api/vex/..%2F..%2Fversions",
			expectedStatus: "invalid Talos version: must not contain a path separator",
		},
		{
			name:           "traversal in the schematic ID of an SPDX request",
			target:         "/api/sbom/..%2F..%2Fversions/v1.13.0/amd64",
			expectedStatus: "invalid schematic ID: must not contain a path separator",
		},
		{
			name:           "traversal in the architecture of an SPDX request",
			target:         "/api/sbom/" + schematicID + "/v1.13.0/..%2F..%2Fversions",
			expectedStatus: "invalid architecture: must not contain a path separator",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &upstream{}

			resp := serve(t, factory, test.target)

			require.Equal(t, http.StatusBadRequest, resp.Code)
			require.Empty(t, factory.paths, "the factory must not be reached at all")
			require.Contains(t, errorStatus(t, resp), test.expectedStatus)
		})
	}
}

// A factory failure is reported under the status it deserves: what the caller asked for being absent
// is theirs to act on, and must not read as an Omni fault.
func TestFactoryFailuresAreMapped(t *testing.T) {
	for _, test := range []struct {
		name           string
		expectedStatus string
		factoryCode    int
		expectedCode   int
	}{
		{
			name:           "scan not produced yet",
			factoryCode:    http.StatusNotFound,
			expectedCode:   http.StatusNotFound,
			expectedStatus: "scan report not found",
		},
		{
			name:           "rejected as invalid",
			factoryCode:    http.StatusBadRequest,
			expectedCode:   http.StatusBadRequest,
			expectedStatus: "invalid",
		},
		{
			// Omni's own credentials were rejected: not something the caller can fix, and it must not
			// reach them as a 401, which the frontend reads as "your keys are stale".
			name:           "credentials rejected",
			factoryCode:    http.StatusUnauthorized,
			expectedCode:   http.StatusBadGateway,
			expectedStatus: "image factory rejected Omni's credentials",
		},
		{
			name:           "rate limited",
			factoryCode:    http.StatusTooManyRequests,
			expectedCode:   http.StatusTooManyRequests,
			expectedStatus: "failed to get scan report",
		},
		{
			name:           "factory broken",
			factoryCode:    http.StatusInternalServerError,
			expectedCode:   http.StatusInternalServerError,
			expectedStatus: "failed to get scan report",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &upstream{respond: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.factoryCode)
			}}

			resp := serve(t, factory, "/api/vulns/"+schematicID+"/v1.13.0/amd64/report.json")

			require.Equal(t, test.expectedCode, resp.Code)
			require.Contains(t, strings.ToLower(errorStatus(t, resp)), strings.ToLower(test.expectedStatus))
		})
	}
}

// Whatever a route logs is tied to the request that produced it: the surrounding request logging
// middleware is not always there, and adjacency is not identity when requests overlap.
func TestFailuresAreLoggedAgainstTheRequestPath(t *testing.T) {
	for _, target := range []string{
		"/api/vulns/" + schematicID + "/v1.13.0/amd64/report.yaml",
		"/api/vex/not-a-version",
		"/api/sbom/" + schematicID + "/v1.13.0/riscv64",
	} {
		t.Run(target, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)

			resp := serveWithLogger(t, &upstream{}, target, zap.New(core))

			require.Equal(t, http.StatusBadRequest, resp.Code)
			require.Equal(t, 1, logs.Len())
			require.Equal(t, target, logs.All()[0].ContextMap()["path"])
		})
	}
}
