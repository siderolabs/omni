// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// schematicID is a valid schematic ID: the hex-encoded SHA-256 of some schematic.
const schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

// upstreamFactory is a stand-in image factory recording the paths it was asked for. The response is
// served by respond, so a test can make the factory fail.
type upstreamFactory struct {
	respond func(http.ResponseWriter, *http.Request)
	paths   []string
}

func (u *upstreamFactory) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.paths = append(u.paths, r.URL.Path)

	if u.respond != nil {
		u.respond(w, r)

		return
	}

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(`{"artifact": true}`)); err != nil {
		panic(err)
	}
}

// factoryServer builds the service against a stand-in factory, and the context a request to it
// arrives on once the auth interceptors have run.
func factoryServer(t *testing.T, factory *upstreamFactory) (*grpcomni.ImageFactoryServer, context.Context) { //nolint:revive
	t.Helper()

	server := httptest.NewServer(factory)
	t.Cleanup(server.Close)

	client, err := imagefactory.NewClient(server.URL, "", "")
	require.NoError(t, err)

	clients := imagefactory.NewClients(state.WrapCore(namespaced.NewState(inmem.Build)), client)

	ctx := ctxstore.WithValue(t.Context(), auth.EnabledAuthContextKey{Enabled: false})

	return grpcomni.NewImageFactoryServer(clients, zaptest.NewLogger(t)), ctx
}

func TestArtifactsAreFetchedFromTheFactory(t *testing.T) {
	for _, test := range []struct {
		call         func(context.Context, *grpcomni.ImageFactoryServer) ([]byte, error)
		name         string
		expectedPath string
	}{
		{
			name:         "scan report",
			expectedPath: "/scans/" + schematicID + "/v1.13.0/amd64/report.json",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) ([]byte, error) {
				resp, err := s.VulnerabilityReport(ctx, &imagefactorypb.VulnerabilityReportRequest{
					SchematicId:  schematicID,
					TalosVersion: "v1.13.0",
					Arch:         imagefactorypb.Arch_AMD64,
					Format:       imagefactorypb.VulnerabilityReportFormat_JSON,
				})

				return resp.GetData(), err
			},
		},
		{
			name:         "scan report in another format",
			expectedPath: "/scans/" + schematicID + "/v1.13.0/arm64/report.table",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) ([]byte, error) {
				resp, err := s.VulnerabilityReport(ctx, &imagefactorypb.VulnerabilityReportRequest{
					SchematicId:  schematicID,
					TalosVersion: "v1.13.0",
					Arch:         imagefactorypb.Arch_ARM64,
					Format:       imagefactorypb.VulnerabilityReportFormat_TABLE,
				})

				return resp.GetData(), err
			},
		},
		{
			name:         "SPDX bundle",
			expectedPath: "/spdx/" + schematicID + "/v1.13.0/amd64",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) ([]byte, error) {
				resp, err := s.SBOM(ctx, &imagefactorypb.SBOMRequest{
					SchematicId:  schematicID,
					TalosVersion: "v1.13.0",
					Arch:         imagefactorypb.Arch_AMD64,
				})

				return resp.GetData(), err
			},
		},
		{
			name:         "VEX document",
			expectedPath: "/vex/v1.13.0/vex.json",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) ([]byte, error) {
				resp, err := s.VEXDocument(ctx, &imagefactorypb.VEXDocumentRequest{TalosVersion: "v1.13.0"})

				return resp.GetData(), err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &upstreamFactory{}
			server, ctx := factoryServer(t, factory)

			data, err := test.call(ctx, server)

			require.NoError(t, err)
			require.Equal(t, []string{test.expectedPath}, factory.paths)
			require.Equal(t, `{"artifact": true}`, string(data))
		})
	}
}

// TestArtifactsAreCached covers that a repeated request is served from the cache, and that the cache
// key covers everything the factory URL is built from - serving one artifact in place of another
// would be worse than not caching at all.
func TestArtifactsAreCached(t *testing.T) {
	factory := &upstreamFactory{}
	server, ctx := factoryServer(t, factory)

	report := func(arch imagefactorypb.Arch, format imagefactorypb.VulnerabilityReportFormat) {
		t.Helper()

		_, err := server.VulnerabilityReport(ctx, &imagefactorypb.VulnerabilityReportRequest{
			SchematicId:  schematicID,
			TalosVersion: "v1.13.0",
			Arch:         arch,
			Format:       format,
		})
		require.NoError(t, err)
	}

	report(imagefactorypb.Arch_AMD64, imagefactorypb.VulnerabilityReportFormat_JSON)
	report(imagefactorypb.Arch_AMD64, imagefactorypb.VulnerabilityReportFormat_JSON)

	require.Equal(t, []string{"/scans/" + schematicID + "/v1.13.0/amd64/report.json"}, factory.paths)

	// A different arch, format or Talos version is a different artifact, and has to be fetched.
	report(imagefactorypb.Arch_ARM64, imagefactorypb.VulnerabilityReportFormat_JSON)
	report(imagefactorypb.Arch_AMD64, imagefactorypb.VulnerabilityReportFormat_SARIF)

	_, err := server.SBOM(ctx, &imagefactorypb.SBOMRequest{
		SchematicId:  schematicID,
		TalosVersion: "v1.13.0",
		Arch:         imagefactorypb.Arch_AMD64,
	})
	require.NoError(t, err)

	require.Equal(t, []string{
		"/scans/" + schematicID + "/v1.13.0/amd64/report.json",
		"/scans/" + schematicID + "/v1.13.0/arm64/report.json",
		"/scans/" + schematicID + "/v1.13.0/amd64/report.sarif",
		"/spdx/" + schematicID + "/v1.13.0/amd64",
	}, factory.paths)
}

// Every request field ends up in the factory URL, so one that could change the shape of that URL must
// be rejected before it gets there: the factory client cleans the "../" it forms, which would leave
// Omni's authenticated client pointed at some other factory endpoint. The architecture and the report
// format cannot express one at all, being enums; what is left is checked.
func TestInvalidRequestsAreNotForwarded(t *testing.T) {
	for _, test := range []struct {
		call            func(context.Context, *grpcomni.ImageFactoryServer) error
		name            string
		expectedMessage string
	}{
		{
			name:            "traversal in the Talos version",
			expectedMessage: "invalid Talos version: must not contain a path separator",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) error {
				_, err := s.VEXDocument(ctx, &imagefactorypb.VEXDocumentRequest{TalosVersion: "../../versions"})

				return err
			},
		},
		{
			name:            "Talos version that is not a version",
			expectedMessage: "invalid Talos version",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) error {
				_, err := s.VEXDocument(ctx, &imagefactorypb.VEXDocumentRequest{TalosVersion: "not-a-version"})

				return err
			},
		},
		{
			name:            "traversal in the schematic ID",
			expectedMessage: "invalid schematic ID",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) error {
				_, err := s.VulnerabilityReport(ctx, &imagefactorypb.VulnerabilityReportRequest{
					SchematicId:  "../../versions",
					TalosVersion: "v1.13.0",
					Arch:         imagefactorypb.Arch_AMD64,
					Format:       imagefactorypb.VulnerabilityReportFormat_JSON,
				})

				return err
			},
		},
		{
			name:            "schematic ID that is not a digest",
			expectedMessage: "invalid schematic ID",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) error {
				_, err := s.SBOM(ctx, &imagefactorypb.SBOMRequest{
					SchematicId:  "not-a-schematic",
					TalosVersion: "v1.13.0",
					Arch:         imagefactorypb.Arch_AMD64,
				})

				return err
			},
		},
		{
			name:            "unset architecture",
			expectedMessage: "invalid architecture",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) error {
				_, err := s.SBOM(ctx, &imagefactorypb.SBOMRequest{
					SchematicId:  schematicID,
					TalosVersion: "v1.13.0",
					Arch:         imagefactorypb.Arch_UNKNOWN_ARCH,
				})

				return err
			},
		},
		{
			name:            "unset report format",
			expectedMessage: "invalid report format",
			call: func(ctx context.Context, s *grpcomni.ImageFactoryServer) error {
				_, err := s.VulnerabilityReport(ctx, &imagefactorypb.VulnerabilityReportRequest{
					SchematicId:  schematicID,
					TalosVersion: "v1.13.0",
					Arch:         imagefactorypb.Arch_AMD64,
					Format:       imagefactorypb.VulnerabilityReportFormat_UNKNOWN_FORMAT,
				})

				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &upstreamFactory{}
			server, ctx := factoryServer(t, factory)

			err := test.call(ctx, server)

			require.Equal(t, grpccodes.InvalidArgument, grpcstatus.Code(err))
			require.Empty(t, factory.paths, "the factory must not be reached at all")
			require.Contains(t, grpcstatus.Convert(err).Message(), test.expectedMessage)
		})
	}
}

// A factory failure is reported under the code it deserves: what the caller asked for being absent is
// theirs to act on, and must not read as an Omni fault.
func TestFactoryFailuresAreMapped(t *testing.T) {
	for _, test := range []struct {
		name            string
		expectedMessage string
		factoryCode     int
		expectedCode    grpccodes.Code
	}{
		{
			name:            "scan not produced yet",
			factoryCode:     http.StatusNotFound,
			expectedCode:    grpccodes.NotFound,
			expectedMessage: "scan report not found",
		},
		{
			name:            "rejected as invalid",
			factoryCode:     http.StatusBadRequest,
			expectedCode:    grpccodes.InvalidArgument,
			expectedMessage: "invalid",
		},
		{
			// Omni's own credentials were rejected: not something the caller can fix, and it must not
			// reach them as Unauthenticated, which the frontend reads as "your keys are stale".
			name:            "credentials rejected",
			factoryCode:     http.StatusUnauthorized,
			expectedCode:    grpccodes.Unavailable,
			expectedMessage: "image factory rejected Omni's credentials",
		},
		{
			name:            "rate limited",
			factoryCode:     http.StatusTooManyRequests,
			expectedCode:    grpccodes.ResourceExhausted,
			expectedMessage: "failed to get scan report",
		},
		{
			name:            "factory broken",
			factoryCode:     http.StatusInternalServerError,
			expectedCode:    grpccodes.Internal,
			expectedMessage: "failed to get scan report",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &upstreamFactory{respond: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.factoryCode)
			}}

			server, ctx := factoryServer(t, factory)

			_, err := server.VulnerabilityReport(ctx, &imagefactorypb.VulnerabilityReportRequest{
				SchematicId:  schematicID,
				TalosVersion: "v1.13.0",
				Arch:         imagefactorypb.Arch_AMD64,
				Format:       imagefactorypb.VulnerabilityReportFormat_JSON,
			})

			require.Equal(t, test.expectedCode, grpcstatus.Code(err))
			require.Contains(t, strings.ToLower(grpcstatus.Convert(err).Message()), strings.ToLower(test.expectedMessage))
		})
	}
}

// Every method is behind the Reader role, which is what replaced the auth middleware the HTTP routes
// these came from were wrapped in.
func TestReaderRoleIsRequired(t *testing.T) {
	for _, test := range []struct {
		name         string
		ctxRole      role.Role
		hasRole      bool
		expectedCode grpccodes.Code
	}{
		{name: "no signature", expectedCode: grpccodes.Unauthenticated},
		{name: "role none", hasRole: true, ctxRole: role.None, expectedCode: grpccodes.PermissionDenied},
		{name: "role reader", hasRole: true, ctxRole: role.Reader, expectedCode: grpccodes.OK},
		{name: "role admin", hasRole: true, ctxRole: role.Admin, expectedCode: grpccodes.OK},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := &upstreamFactory{}
			server, _ := factoryServer(t, factory)

			ctx := ctxstore.WithValue(t.Context(), auth.EnabledAuthContextKey{Enabled: true})
			if test.hasRole {
				ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: test.ctxRole})
			}

			_, err := server.VEXDocument(ctx, &imagefactorypb.VEXDocumentRequest{TalosVersion: "v1.13.0"})

			require.Equal(t, test.expectedCode, grpcstatus.Code(err))

			if test.expectedCode != grpccodes.OK {
				require.Empty(t, factory.paths, "the factory must not be reached at all")
			}
		})
	}
}
