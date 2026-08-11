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

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

type imageFactoryMock struct {
	listener   net.Listener
	schematics map[string]schematic.Schematic
	eg         errgroup.Group
	address    string

	schematicMu sync.Mutex
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

// TestBootAssetURL pins the server-side boot asset build: Omni assembles the image factory filename
// from the asset spec, picks the factory serving that Talos version, and places the credentials where
// the caller can use them.
func (suite *GrpcSuite) TestBootAssetURL() {
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
		request      *management.BootAssetURLRequest
		expectedURL  string
		expectHeader bool
	}{
		{
			name: "disk image with credentials in the headers",
			request: &management.BootAssetURLRequest{
				BootAssetKind: management.BootAssetURLRequest_BOOT_ASSET_KIND_DISK,
				Platform:      "nocloud",
				Architecture:  "amd64",
				Format:        "raw.xz",
			},
			expectedURL:  "https://factory.example.org/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
			expectHeader: true,
		},
		{
			name: "standalone disk image carries them in the URL",
			request: &management.BootAssetURLRequest{
				BootAssetKind: management.BootAssetURLRequest_BOOT_ASSET_KIND_DISK,
				Platform:      "nocloud",
				Architecture:  "amd64",
				Format:        "qcow2",
				StandaloneUrl: true,
			},
			expectedURL: "https://user:hunter2@factory.example.org/image/" + schematicID + "/v1.13.0/nocloud-amd64.qcow2",
		},
		{
			name: "secure boot ISO",
			request: &management.BootAssetURLRequest{
				BootAssetKind: management.BootAssetURLRequest_BOOT_ASSET_KIND_ISO,
				Platform:      "metal",
				Architecture:  "amd64",
				SecureBoot:    true,
			},
			expectedURL:  "https://factory.example.org/image/" + schematicID + "/v1.13.0/metal-amd64-secureboot.iso",
			expectHeader: true,
		},
		{
			// PXE firmware cannot send headers, so the URL carries the credentials even unasked.
			name: "PXE is always standalone",
			request: &management.BootAssetURLRequest{
				BootAssetKind: management.BootAssetURLRequest_BOOT_ASSET_KIND_PXE,
				Platform:      "metal",
				Architecture:  "amd64",
			},
			expectedURL: "https://user:hunter2@pxe.factory.example.org/pxe/" + schematicID + "/v1.13.0/metal-amd64",
		},
	} {
		suite.Run(tt.name, func() {
			tt.request.TalosVersion = "1.13.0"
			tt.request.SchematicId = schematicID

			resp, err := client.GetBootAssetURL(ctx, tt.request)
			suite.Require().NoError(err)

			suite.Require().Equal(tt.expectedURL, resp.Url)
			suite.Require().Equal("factory.example.org", strings.TrimPrefix(resp.ImageFactoryHost, "pxe."))

			if tt.expectHeader {
				suite.Require().Equal(map[string]string{"Authorization": "Basic dXNlcjpodW50ZXIy"}, resp.Headers)
			} else {
				suite.Require().Empty(resp.Headers)
			}
		})
	}

	for _, tt := range []struct {
		request *management.BootAssetURLRequest
		name    string
	}{
		{
			name:    "kind unset",
			request: &management.BootAssetURLRequest{Platform: "metal", Architecture: "amd64"},
		},
		{
			// These fields become URL path segments, so traversal has to be refused outright.
			name: "platform traversal",
			request: &management.BootAssetURLRequest{
				BootAssetKind: management.BootAssetURLRequest_BOOT_ASSET_KIND_ISO,
				Platform:      "../../secret",
				Architecture:  "amd64",
			},
		},
		{
			name: "disk without a format",
			request: &management.BootAssetURLRequest{
				BootAssetKind: management.BootAssetURLRequest_BOOT_ASSET_KIND_DISK,
				Platform:      "nocloud",
				Architecture:  "amd64",
			},
		},
		{
			// The schematic ID and the Talos version become path segments too.
			name: "schematic ID traversal",
			request: &management.BootAssetURLRequest{
				BootAssetKind: management.BootAssetURLRequest_BOOT_ASSET_KIND_ISO,
				Platform:      "metal",
				Architecture:  "amd64",
				SchematicId:   "../../..",
			},
		},
		{
			name: "Talos version traversal",
			request: &management.BootAssetURLRequest{
				BootAssetKind: management.BootAssetURLRequest_BOOT_ASSET_KIND_ISO,
				Platform:      "metal",
				Architecture:  "amd64",
				TalosVersion:  "1.13.0/../..",
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

			_, err := client.GetBootAssetURL(ctx, tt.request)
			suite.Require().Equal(codes.InvalidArgument, status.Code(err))
		})
	}
}
