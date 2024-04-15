// Copyright (c) 2024 Sidero Labs, Inc.
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

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
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

func (m *imageFactoryMock) run() error {
	var err error

	m.listener, err = net.Listen("tcp", ":0")
	if err != nil {
		return err
	}

	m.address = fmt.Sprintf("http://%s", m.listener.Addr().String())

	return nil
}

func (m *imageFactoryMock) serve(ctx context.Context) {
	router := httprouter.New()
	router.POST("/schematics", m.handleSchematics)

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

		return server.Shutdown(innerContext)
	})
}

func (m *imageFactoryMock) handleSchematics(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	m.schematicMu.Lock()
	defer m.schematicMu.Unlock()

	id, err := m.saveSchematic(r)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error())) //nolint:errcheck

		return
	}

	rw.Header().Add("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)

	resp, err := json.Marshal(struct {
		ID string `json:"id"`
	}{
		ID: id,
	})
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error())) //nolint:errcheck

		return
	}

	rw.Write(resp) //nolint:errcheck
}

func (m *imageFactoryMock) saveSchematic(r *http.Request) (string, error) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	if err = r.Body.Close(); err != nil {
		return "", err
	}

	cfg, err := schematic.Unmarshal(data)
	if err != nil {
		return "", err
	}

	id, err := cfg.ID()
	if err != nil {
		return "", err
	}

	if m.schematics == nil {
		m.schematics = map[string]schematic.Schematic{}
	}

	m.schematics[id] = *cfg

	return id, nil
}

func (suite *GrpcSuite) TestSchematicCreate() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	params := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
	params.TypedSpec().Value.Args = "arg=value"

	suite.Require().NoError(suite.state.Create(ctx, params))

	client := management.NewManagementServiceClient(suite.conn)

	media := omni.NewInstallationMedia(resources.EphemeralNamespace, "test")

	suite.Require().NoError(suite.state.Create(ctx, media))

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
					meta.LabelsMeta: "",
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
					meta.LabelsMeta:                 "",
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
	} {
		req := tt.request
		req.TalosVersion = "v1.6.5"
		req.MediaId = "test"

		suite.T().Run(tt.name, func(t *testing.T) {
			resp, err := client.CreateSchematic(ctx, req)
			if tt.expectedError != nil {
				tt.expectedError(t, err)

				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, resp.SchematicId)

			rtestutils.AssertResource(ctx, t, suite.state, resp.SchematicId, func(*omni.Schematic, *assert.Assertions) {})

			suite.imageFactory.schematicMu.Lock()
			defer suite.imageFactory.schematicMu.Unlock()

			config, ok := suite.imageFactory.schematics[resp.SchematicId]
			require.Truef(t, ok, "the schematic id %q doesn't exist in the image factory", resp.SchematicId)

			meta := xslices.ToMap(config.Customization.Meta, func(k schematic.MetaValue) (uint32, string) {
				return uint32(k.Key), k.Value
			})

			require.EqualValues(t, req.MetaValues, meta)
			require.Equal(t, strings.Join(append([]string{params.TypedSpec().Value.Args}, req.ExtraKernelArgs...), " "), strings.Join(config.Customization.ExtraKernelArgs, " "))
		})
	}
}
