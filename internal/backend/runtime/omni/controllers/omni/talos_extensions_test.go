// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
)

//nolint:govet
type imageFactoryMock struct {
	listener           net.Listener
	talosVersions      []string
	extensionsVersions map[string][]client.ExtensionInfo
	eg                 errgroup.Group
	address            string
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
	router.GET("/version/:version/extensions/official", m.handleVersions)
	router.GET("/versions", m.handleTalosVersions)

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

func (m *imageFactoryMock) handleTalosVersions(rw http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	rw.WriteHeader(http.StatusOK)

	resp, err := json.Marshal(m.talosVersions)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error())) //nolint:errcheck

		return
	}

	rw.Write(resp) //nolint:errcheck
}

func (m *imageFactoryMock) handleVersions(rw http.ResponseWriter, _ *http.Request, params httprouter.Params) {
	version := params.ByName("version")

	versions, ok := m.extensionsVersions[version]
	if !ok {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("version not supported")) //nolint:errcheck

		return
	}

	rw.Header().Add("Content-Type", "application/json")

	resp, err := json.Marshal(versions)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error())) //nolint:errcheck

		return
	}

	rw.Write(resp) //nolint:errcheck
}

type TalosExtensionsSuite struct {
	OmniSuite
}

func (suite *TalosExtensionsSuite) TestReconcile() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	factory := imageFactoryMock{
		extensionsVersions: map[string][]client.ExtensionInfo{
			"v1.6.0": {
				{
					Name:        "siderolabs/hello-world-service",
					Ref:         "github.com/siderolabs/hello-world-service:v1.6.0",
					Digest:      "aaaa",
					Author:      "Sidero Labs",
					Description: "This system extension provides an example Talos extension service.",
				},
			},
			"v200.0.0": {
				{
					Name:   "siderolabs/hello-future",
					Ref:    "github.com/siderolabs/hello-future:v200.0.0",
					Digest: "aaaa",
				},
			},
		},
	}
	suite.Require().NoError(factory.run())

	factory.serve(ctx)

	defer func() {
		cancel()

		factory.eg.Wait() //nolint:errcheck
	}()

	config.Config.ImageFactoryBaseURL = factory.address

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosExtensionsController()))

	versions := []string{
		"v0.14.0", "v1.6.0", "v200.0.0",
	}

	factory.talosVersions = versions[1:]

	for _, v := range versions {
		version := omni.NewTalosVersion(resources.DefaultNamespace, v)
		version.TypedSpec().Value.Version = v

		suite.Require().NoError(suite.state.Create(ctx, version))
	}

	rtestutils.AssertNoResource[*omni.TalosExtensions](ctx, suite.T(), suite.state, versions[0])

	rtestutils.AssertResources(ctx, suite.T(), suite.state, versions[1:], func(res *omni.TalosExtensions, assert *assert.Assertions) {
		assert.Len(res.TypedSpec().Value.Items, 1, "no extensions for version %s", res.Metadata().ID())
		manifest := res.TypedSpec().Value.Items[0]

		switch res.Metadata().ID() {
		case "v1.6.0":
			assert.EqualValues("Sidero Labs", manifest.Author)
			assert.EqualValues("This system extension provides an example Talos extension service.", manifest.Description)
			assert.EqualValues("aaaa", manifest.Digest)
			assert.EqualValues("v1.6.0", manifest.Version)
			assert.EqualValues("siderolabs/hello-world-service", manifest.Name)
			assert.EqualValues("github.com/siderolabs/hello-world-service:v1.6.0", manifest.Ref)
			// no info in the manifests, should still be in the list but without additional info
		case "v200.0.0":
			assert.EqualValues("v200.0.0", manifest.Version)
			assert.EqualValues("siderolabs/hello-future", manifest.Name)
			assert.EqualValues("aaaa", manifest.Digest)
			assert.EqualValues("", manifest.Description)
		}
	})
}

func TestTalosExtensionsSuite(t *testing.T) {
	suite.Run(t, new(TalosExtensionsSuite))
}
