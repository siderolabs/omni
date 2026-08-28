// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// The credentials the installation media tests configure on both sides: the ImageFactoryAuth resource Omni reads
// for the fallback, and the factory client Omni requests a download token with.
const (
	testFactoryUsername = "user"
	testFactoryPassword = "hunter2"

	// testFactoryAuthorization is the two of them as the factory sees them on the wire.
	testFactoryAuthorization = "Basic dXNlcjpodW50ZXIy"
)

type imageFactoryMock struct {
	listener   net.Listener
	schematics map[string]schematic.Schematic

	// downloadToken decides what POST /download-token answers, returning a status and either the token
	// or an error body. Nil answers 404, which is what an unregistered route returns and so stands in for
	// every factory that does not issue tokens.
	downloadToken func(ttl string) (int, string)

	eg      errgroup.Group
	address string

	// tokenTTLs records the ttl query parameter of every request, so a test can check what Omni asked for.
	tokenTTLs []string

	// tokenAuth records the Authorization header of every request, so a test can check that Omni
	// authenticates the token request rather than relying on the route being open.
	tokenAuth []string

	schematicMu sync.Mutex
	tokenMu     sync.Mutex
}

// setDownloadToken installs what POST /download-token answers and forgets the requests so far, so each
// case reads only its own.
func (m *imageFactoryMock) setDownloadToken(handler func(ttl string) (int, string)) {
	m.tokenMu.Lock()
	defer m.tokenMu.Unlock()

	m.downloadToken = handler
	m.tokenTTLs = nil
	m.tokenAuth = nil
}

// downloadTokenTTLs returns the lifetimes asked for since the last setDownloadToken, in order.
func (m *imageFactoryMock) downloadTokenTTLs() []string {
	m.tokenMu.Lock()
	defer m.tokenMu.Unlock()

	return slices.Clone(m.tokenTTLs)
}

// downloadTokenAuth returns the Authorization header of every token request since the last
// setDownloadToken, in order.
func (m *imageFactoryMock) downloadTokenAuth() []string {
	m.tokenMu.Lock()
	defer m.tokenMu.Unlock()

	return slices.Clone(m.tokenAuth)
}

func (m *imageFactoryMock) handleDownloadToken(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	m.tokenMu.Lock()
	m.tokenTTLs = append(m.tokenTTLs, r.URL.Query().Get("ttl"))
	m.tokenAuth = append(m.tokenAuth, r.Header.Get("Authorization"))
	handler := m.downloadToken
	m.tokenMu.Unlock()

	if handler == nil {
		rw.WriteHeader(http.StatusNotFound)

		return
	}

	code, body := handler(r.URL.Query().Get("ttl"))

	if code != http.StatusOK {
		rw.WriteHeader(code)
		rw.Write([]byte(body)) //nolint:errcheck

		return
	}

	rw.Header().Add("Content-Type", "application/json")

	// The shape the pinned client decodes. It reads access_token and discards the rest, which is why Omni
	// derives the expiry from the lifetime it sent rather than from expires_in.
	rw.Write(fmt.Appendf(nil, `{"access_token":%q,"token_type":"Bearer","expires_in":300}`, body)) //nolint:errcheck
}

func (m *imageFactoryMock) run(ctx context.Context) error {
	var err error

	m.listener, err = (&net.ListenConfig{}).Listen(ctx, "tcp", ":0")
	if err != nil {
		return err
	}

	m.address = fmt.Sprintf("http://%s", m.listener.Addr().String())

	return nil
}

func (m *imageFactoryMock) serve(ctx context.Context) {
	router := httprouter.New()
	router.POST("/schematics", m.handleSchematics)
	router.GET("/schematics/:id", m.handleSchematicGet)
	router.GET("/versions", m.handleVersions)
	router.POST("/download-token", m.handleDownloadToken)

	server := http.Server{
		Handler: router,
	}

	m.eg.Go(func() error {
		if err := server.Serve(m.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	})

	m.eg.Go(func() error {
		<-ctx.Done()

		innerContext, cancel := context.WithTimeout(ctx, time.Second)

		defer cancel()

		if err := server.Shutdown(innerContext); err != nil && !errors.Is(err, ctx.Err()) {
			return err
		}

		return nil
	})
}

func (m *imageFactoryMock) handleVersions(rw http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	rw.Header().Add("Content-Type", "application/json")

	resp, err := json.Marshal([]string{})
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error())) //nolint:errcheck

		return
	}

	rw.Write(resp) //nolint:errcheck
}

func (m *imageFactoryMock) handleSchematics(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	m.schematicMu.Lock()
	defer m.schematicMu.Unlock()

	id, data, err := m.saveSchematic(r)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error())) //nolint:errcheck

		return
	}

	rw.Header().Add("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)

	resp, err := json.Marshal(struct {
		ID        string `json:"id"`
		Schematic string `json:"schematic"`
	}{
		ID:        id,
		Schematic: string(data),
	})
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error())) //nolint:errcheck

		return
	}

	rw.Write(resp) //nolint:errcheck
}

func (m *imageFactoryMock) handleSchematicGet(rw http.ResponseWriter, _ *http.Request, params httprouter.Params) {
	m.schematicMu.Lock()
	defer m.schematicMu.Unlock()

	id := params.ByName("id")

	cfg, ok := m.schematics[id]
	if !ok {
		rw.WriteHeader(http.StatusNotFound)

		return
	}

	data, err := cfg.Marshal()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error())) //nolint:errcheck

		return
	}

	rw.Header().Add("Content-Type", "application/yaml")
	rw.WriteHeader(http.StatusOK)
	rw.Write(data) //nolint:errcheck
}

func (m *imageFactoryMock) saveSchematic(r *http.Request) (string, []byte, error) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", nil, err
	}

	if err = r.Body.Close(); err != nil {
		return "", nil, err
	}

	cfg, err := schematic.Unmarshal(data)
	if err != nil {
		return "", nil, err
	}

	id, err := cfg.ID()
	if err != nil {
		return "", nil, err
	}

	if m.schematics == nil {
		m.schematics = map[string]schematic.Schematic{}
	}

	m.schematics[id] = *cfg

	return id, data, nil
}

func (suite *GrpcSuite) TestSchematicCreate() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	params := siderolink.NewDefaultJoinToken()
	params.TypedSpec().Value.TokenId = "abcd"

	suite.Require().NoError(suite.state.Create(ctx, params))

	apiConfig := siderolink.NewAPIConfig()
	apiConfig.TypedSpec().Value.EventsPort = 8091
	apiConfig.TypedSpec().Value.LogsPort = 8092
	apiConfig.TypedSpec().Value.MachineApiAdvertisedUrl = "grpc://127.0.0.1:8090"

	suite.Require().NoError(suite.state.Create(ctx, apiConfig))

	client := management.NewManagementServiceClient(suite.conn)

	media := omni.NewInstallationMedia("test")

	suite.Require().NoError(suite.state.Create(ctx, media))

	overlayMedia := omni.NewInstallationMedia("overlay-test")
	overlayMedia.TypedSpec().Value.Overlay = "test-overlay"

	suite.Require().NoError(suite.state.Create(ctx, overlayMedia))

	for _, tt := range []struct {
		request       *management.CreateSchematicRequest
		expectedError func(*testing.T, error)
		name          string
	}{
		{
			name:    "empty",
			request: &management.CreateSchematicRequest{},
		},
		{
			name: "with extensions",
			request: &management.CreateSchematicRequest{
				Extensions: []string{
					"github.com/my/cool-extension",
				},
			},
		},
		{
			name: "with extensions and labels",
			request: &management.CreateSchematicRequest{
				Extensions: []string{
					"github.com/my/another-one",
				},
				MetaValues: map[uint32]string{
					meta.LabelsMeta: `machineLabels:
  something: value`,
				},
			},
		},
		{
			name: "with extensions, labels and extra kernel args",
			request: &management.CreateSchematicRequest{
				Extensions: []string{
					"github.com/my/another-one",
				},
				MetaValues: map[uint32]string{
					meta.LabelsMeta: `machineLabels:
  something: value`,
					meta.MetalNetworkPlatformConfig: "{}",
				},
				ExtraKernelArgs: []string{
					"ip=127.0.0.1",
					"another=value",
				},
			},
		},
		{
			name: "fail to set protected meta key",
			request: &management.CreateSchematicRequest{
				Extensions: []string{
					"github.com/my/another-one",
				},
				MetaValues: map[uint32]string{
					meta.StateEncryptionConfig: "",
				},
			},
			expectedError: func(t *testing.T, err error) {
				require.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
		{
			name: "fail to parse labels",
			request: &management.CreateSchematicRequest{
				Extensions: []string{
					"github.com/my/another-one",
				},
				MetaValues: map[uint32]string{
					meta.LabelsMeta: "this is invalid yaml",
				},
			},
			expectedError: func(t *testing.T, err error) {
				require.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
		{
			name: "empty labels",
			request: &management.CreateSchematicRequest{
				Extensions: []string{
					"github.com/my/another-one",
				},
				MetaValues: map[uint32]string{
					meta.LabelsMeta: "{}",
				},
			},
			expectedError: func(t *testing.T, err error) {
				require.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
		{
			name: "legacy labels",
			request: &management.CreateSchematicRequest{
				Extensions: []string{
					"github.com/my/another-one",
				},
				MetaValues: map[uint32]string{
					meta.LabelsMeta: `{"initialMachineLabels": {"aaa": bbb}}`,
				},
			},
			expectedError: func(t *testing.T, err error) {
				require.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
		{
			name: "invalid Talos version",
			request: &management.CreateSchematicRequest{
				TalosVersion: "../../secret",
				MediaId:      "overlay-test",
			},
			expectedError: func(t *testing.T, err error) {
				require.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
	} {
		req := tt.request
		if req.TalosVersion == "" {
			req.TalosVersion = "v1.6.5"
		}

		if req.MediaId == "" {
			req.MediaId = "test"
		}

		suite.T().Run(tt.name, func(t *testing.T) {
			resp, err := client.CreateSchematic(ctx, req)
			if tt.expectedError != nil {
				tt.expectedError(t, err)

				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, resp.SchematicId)

			suite.imageFactory.schematicMu.Lock()
			defer suite.imageFactory.schematicMu.Unlock()

			config, ok := suite.imageFactory.schematics[resp.SchematicId]
			require.Truef(t, ok, "the schematic id %q doesn't exist in the image factory", resp.SchematicId)

			meta := xslices.ToMap(config.Customization.Meta, func(k schematic.MetaValue) (uint32, string) {
				return uint32(k.Key), k.Value
			})

			args := []string{
				"siderolink.api=grpc://127.0.0.1:8090?jointoken=abcd",
				"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
				"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
			}

			require.EqualValues(t, req.MetaValues, meta)
			require.Equal(t, append(args, req.ExtraKernelArgs...), config.Customization.ExtraKernelArgs)
		})
	}
}

// TestMediaURL pins the server-side installation media build: Omni assembles the image factory filename
// from the media spec, picks the factory serving that Talos version, and places the credentials where
// the caller can use them.
func (suite *GrpcSuite) TestMediaURL() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	features := omni.NewFeaturesConfig(omni.FeaturesConfigID)
	features.TypedSpec().Value.ImageFactoryBaseUrl = "https://factory.example.org"
	features.TypedSpec().Value.ImageFactoryPxeBaseUrl = "https://pxe.factory.example.org"

	suite.Require().NoError(suite.state.Create(ctx, features))

	factoryAuth := omni.NewImageFactoryAuth("https://factory.example.org")
	factoryAuth.TypedSpec().Value.Username = "user"
	factoryAuth.TypedSpec().Value.Password = "hunter2"

	suite.Require().NoError(suite.state.Create(ctx, factoryAuth))

	client := management.NewManagementServiceClient(suite.conn)

	const schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

	for _, tt := range []struct {
		name         string
		request      *management.InstallationMediaURLRequest
		expectedURL  string
		expectHeader bool
	}{
		{
			name: "disk image with credentials in the headers",
			request: &management.InstallationMediaURLRequest{
				InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_DISK,
				Platform:              "nocloud",
				Architecture:          "amd64",
				Format:                "raw.xz",
			},
			expectedURL:  "https://factory.example.org/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
			expectHeader: true,
		},
		{
			name: "standalone disk image carries them in the URL",
			request: &management.InstallationMediaURLRequest{
				InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_DISK,
				Platform:              "nocloud",
				Architecture:          "amd64",
				Format:                "qcow2",
				StandaloneUrl:         true,
			},
			expectedURL: "https://user:hunter2@factory.example.org/image/" + schematicID + "/v1.13.0/nocloud-amd64.qcow2",
		},
		{
			name: "secure boot ISO",
			request: &management.InstallationMediaURLRequest{
				InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_ISO,
				Platform:              "metal",
				Architecture:          "amd64",
				SecureBoot:            true,
			},
			expectedURL:  "https://factory.example.org/image/" + schematicID + "/v1.13.0/metal-amd64-secureboot.iso",
			expectHeader: true,
		},
		{
			// PXE firmware cannot send headers, so the URL carries the credentials even unasked.
			name: "PXE is always standalone",
			request: &management.InstallationMediaURLRequest{
				InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_PXE,
				Platform:              "metal",
				Architecture:          "amd64",
			},
			expectedURL: "https://user:hunter2@pxe.factory.example.org/pxe/" + schematicID + "/v1.13.0/metal-amd64",
		},
	} {
		suite.Run(tt.name, func() {
			tt.request.TalosVersion = "1.13.0"
			tt.request.SchematicId = schematicID

			resp, err := client.GetInstallationMediaURL(ctx, tt.request)
			suite.Require().NoError(err)

			suite.Require().Equal(tt.expectedURL, resp.Url)
			suite.Require().Equal("factory.example.org", strings.TrimPrefix(resp.ImageFactoryHost, "pxe."))

			if tt.expectHeader {
				suite.Require().Equal(map[string]string{"Authorization": testFactoryAuthorization}, resp.Headers)
			} else {
				suite.Require().Empty(resp.Headers)
			}
		})
	}

	for _, tt := range []struct {
		request *management.InstallationMediaURLRequest
		name    string
	}{
		{
			name:    "kind unset",
			request: &management.InstallationMediaURLRequest{Platform: "metal", Architecture: "amd64"},
		},
		{
			// These fields become URL path segments, so traversal has to be refused outright.
			name: "platform traversal",
			request: &management.InstallationMediaURLRequest{
				InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_ISO,
				Platform:              "../../secret",
				Architecture:          "amd64",
			},
		},
		{
			name: "disk without a format",
			request: &management.InstallationMediaURLRequest{
				InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_DISK,
				Platform:              "nocloud",
				Architecture:          "amd64",
			},
		},
		{
			// The schematic ID and the Talos version become path segments too.
			name: "schematic ID traversal",
			request: &management.InstallationMediaURLRequest{
				InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_ISO,
				Platform:              "metal",
				Architecture:          "amd64",
				SchematicId:           "../../..",
			},
		},
		{
			name: "Talos version traversal",
			request: &management.InstallationMediaURLRequest{
				InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_ISO,
				Platform:              "metal",
				Architecture:          "amd64",
				TalosVersion:          "1.13.0/../..",
			},
		},
	} {
		suite.Run(tt.name, func() {
			if tt.request.TalosVersion == "" {
				tt.request.TalosVersion = "1.13.0"
			}

			if tt.request.SchematicId == "" {
				tt.request.SchematicId = schematicID
			}

			_, err := client.GetInstallationMediaURL(ctx, tt.request)
			suite.Require().Equal(codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestMediaToken covers what a caller gets when the factory Omni is configured with issues download
// tokens: a URL that expires and only downloads, instead of the long-lived credential that works on every
// factory route.
//
// TestMediaURL points FeaturesConfig at a factory no client is configured for, so it pins the
// credential fallback. This points it at the mock, so ForURL finds the client and the token is issued.
func (suite *GrpcSuite) TestMediaToken() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*30)
	defer cancel()

	const (
		token       = "eyJhbGciOiJFUzI1NiJ9.fake.token"
		schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"
	)

	const pxeBaseURL = "https://pxe.factory.example.org"

	features := omni.NewFeaturesConfig(omni.FeaturesConfigID)
	features.TypedSpec().Value.ImageFactoryBaseUrl = suite.imageFactory.address
	features.TypedSpec().Value.ImageFactoryPxeBaseUrl = pxeBaseURL

	suite.Require().NoError(suite.state.Create(ctx, features))

	factoryAuth := omni.NewImageFactoryAuth(suite.imageFactory.address)
	factoryAuth.TypedSpec().Value.Username = testFactoryUsername
	factoryAuth.TypedSpec().Value.Password = testFactoryPassword

	suite.Require().NoError(suite.state.Create(ctx, factoryAuth))

	client := management.NewManagementServiceClient(suite.conn)

	newRequest := func() *management.InstallationMediaURLRequest {
		return &management.InstallationMediaURLRequest{
			TalosVersion:          "1.13.0",
			SchematicId:           schematicID,
			InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_DISK,
			Platform:              "nocloud",
			Architecture:          "amd64",
			Format:                "raw.xz",
		}
	}

	assetPath := "/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz"

	issuesToken := func(string) (int, string) { return http.StatusOK, token }

	suite.Run("no requested lifetime takes Omni's default", func() {
		suite.imageFactory.setDownloadToken(issuesToken)

		before := time.Now()

		resp, err := client.GetInstallationMediaURL(ctx, newRequest())
		suite.Require().NoError(err)

		suite.Require().Equal(suite.imageFactory.address+assetPath+"?token="+url.QueryEscape(token), resp.Url)
		suite.Require().Empty(resp.Headers, "a token in the URL leaves nothing for the headers to carry")

		// Sent explicitly rather than left to the factory, since the granted lifetime is the one thing the
		// factory does not report back and the expiry has to come from somewhere.
		suite.Require().Equal([]string{"5m0s"}, suite.imageFactory.downloadTokenTTLs())

		// A token is scoped to the identity that asked for it, so an unauthenticated request would either be
		// refused or scoped to nobody. Omni has to present its factory credentials to get one.
		suite.Require().Equal([]string{testFactoryAuthorization}, suite.imageFactory.downloadTokenAuth())

		suite.Require().NotNil(resp.ExpiresAt)
		suite.Require().WithinRange(resp.ExpiresAt.AsTime(), before.Add(5*time.Minute), time.Now().Add(5*time.Minute))
	})

	suite.Run("a requested lifetime reaches the factory and moves the expiry", func() {
		suite.imageFactory.setDownloadToken(issuesToken)

		request := newRequest()
		request.DownloadTokenTtl = durationpb.New(time.Hour)

		before := time.Now()

		resp, err := client.GetInstallationMediaURL(ctx, request)
		suite.Require().NoError(err)

		suite.Require().Equal([]string{"1h0m0s"}, suite.imageFactory.downloadTokenTTLs())
		suite.Require().NotNil(resp.ExpiresAt)
		suite.Require().WithinRange(resp.ExpiresAt.AsTime(), before.Add(time.Hour), time.Now().Add(time.Hour))
	})

	suite.Run("a standalone URL carries the token too", func() {
		suite.imageFactory.setDownloadToken(issuesToken)

		request := newRequest()
		request.StandaloneUrl = true

		resp, err := client.GetInstallationMediaURL(ctx, request)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.imageFactory.address+assetPath+"?token="+url.QueryEscape(token), resp.Url)
		suite.Require().NotContains(resp.Url, "hunter2", "the credential must not travel alongside the token")
	})

	suite.Run("a PXE script carries the token too", func() {
		// The factory forwards it into the kernel and initramfs URLs of the script it serves, so the token
		// authenticates the whole boot and the credentials never leave Omni.
		suite.imageFactory.setDownloadToken(issuesToken)

		request := newRequest()
		request.InstallationMediaKind = management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_PXE
		request.Platform = "metal"
		request.Format = ""

		resp, err := client.GetInstallationMediaURL(ctx, request)
		suite.Require().NoError(err)

		suite.Require().Equal(pxeBaseURL+"/pxe/"+schematicID+"/v1.13.0/metal-amd64?token="+url.QueryEscape(token), resp.Url)
		suite.Require().Empty(resp.Headers, "PXE firmware cannot send a header")
		suite.Require().NotContains(resp.Url, testFactoryPassword, "the credential must not travel alongside the token")

		// A PXE URL is written into a boot configuration before anything boots from it, so its default
		// lifetime is longer than the one an image download gets.
		suite.Require().Equal([]string{"2h0m0s"}, suite.imageFactory.downloadTokenTTLs())
		suite.Require().NotNil(resp.ExpiresAt)
	})

	suite.Run("a PXE script falls back to credentials when the factory issues no token", func() {
		// An image factory below 1.6.0 rejects a token on /pxe/, and one below 1.5.0 or with its own
		// authentication disabled issues none at all. Both answer 404 here.
		suite.imageFactory.setDownloadToken(nil)

		request := newRequest()
		request.InstallationMediaKind = management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_PXE
		request.Platform = "metal"
		request.Format = ""

		resp, err := client.GetInstallationMediaURL(ctx, request)
		suite.Require().NoError(err)

		suite.Require().Equal("https://"+testFactoryUsername+":"+testFactoryPassword+"@pxe.factory.example.org/pxe/"+
			schematicID+"/v1.13.0/metal-amd64", resp.Url)
		suite.Require().Empty(resp.Headers)
		suite.Require().Nil(resp.ExpiresAt)
	})

	suite.Run("a refused lifetime is InvalidArgument", func() {
		// The factory answers 400 for anything outside its configured bounds. The caller chose the lifetime,
		// so it learns its request was refused rather than quietly receiving a long-lived credential.
		suite.imageFactory.setDownloadToken(func(string) (int, string) {
			return http.StatusBadRequest, "ttl must be between 30s and 8h"
		})

		request := newRequest()
		request.DownloadTokenTtl = durationpb.New(100 * time.Hour)

		_, err := client.GetInstallationMediaURL(ctx, request)
		suite.Require().Equal(codes.InvalidArgument, status.Code(err))

		// The bounds are the whole point of passing the factory's own body on rather than summarizing it:
		// they are what the caller needs to pick a lifetime that works.
		suite.Require().Contains(status.Convert(err).Message(), "between 30s and 8h")
		suite.Require().Contains(status.Convert(err).Message(), "100h0m0s")
	})

	suite.Run("a rejected token request falls back to credentials", func() {
		// The credentials Omni requests the token with come from its startup configuration. If a factory
		// refuses them, the resolve must still answer rather than failing a provision.
		suite.imageFactory.setDownloadToken(func(string) (int, string) {
			return http.StatusUnauthorized, "unauthorized"
		})

		resp, err := client.GetInstallationMediaURL(ctx, newRequest())
		suite.Require().NoError(err)

		suite.Require().Equal(suite.imageFactory.address+assetPath, resp.Url)
		suite.Require().Equal(map[string]string{"Authorization": testFactoryAuthorization}, resp.Headers)
		suite.Require().Nil(resp.ExpiresAt)
	})

	suite.Run("a negative lifetime is InvalidArgument", func() {
		suite.imageFactory.setDownloadToken(issuesToken)

		request := newRequest()
		request.DownloadTokenTtl = durationpb.New(-time.Hour)

		_, err := client.GetInstallationMediaURL(ctx, request)
		suite.Require().Equal(codes.InvalidArgument, status.Code(err))

		// It never reaches the factory, since the client would drop a non-positive lifetime and the factory
		// would grant its own default instead, leaving Omni reporting an expiry in the past.
		suite.Require().Empty(suite.imageFactory.downloadTokenTTLs())
	})

	suite.Run("a factory that does not issue tokens falls back to credentials", func() {
		// Nil answers 404, exactly as an unregistered route does: every build below 1.5.0, every community
		// build, and any factory with its own authentication disabled.
		suite.imageFactory.setDownloadToken(nil)

		resp, err := client.GetInstallationMediaURL(ctx, newRequest())
		suite.Require().NoError(err)

		suite.Require().Equal(suite.imageFactory.address+assetPath, resp.Url)
		suite.Require().Equal(map[string]string{"Authorization": testFactoryAuthorization}, resp.Headers)
		suite.Require().Nil(resp.ExpiresAt, "a credential does not expire, so there is nothing to report")
	})
}
