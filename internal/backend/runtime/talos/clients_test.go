// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/logging"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

type ClientsSuite struct { //nolint:govet
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *ClientsSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.T().Context(), 3*time.Minute)
	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.Assert().NoError(err)

	logger := zaptest.NewLogger(suite.T()).With(logging.Component("clients"))

	suite.runtime, err = runtime.NewRuntime(suite.state, logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel)), omniruntime.RuntimeCacheOptions()...)
	suite.Require().NoError(err)

	suite.wg.Go(func() {
		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	})
}

func (suite *ClientsSuite) TestGetClient() {
	clusterName := "omni"

	logger := zaptest.NewLogger(suite.T())

	clientFactory := talos.NewClientFactory(suite.state, logger)

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))

	_, err := clientFactory.Get(suite.ctx, clusterName)
	suite.Require().True(talos.IsClientNotReadyError(err))

	configBundle, err := bundle.NewBundle(bundle.WithInputOptions(
		&bundle.InputOptions{
			ClusterName: clusterName,
			Endpoint:    "https://127.0.0.1:6443",
		}))
	suite.Require().NoError(err)

	talosconfig := omni.NewTalosConfig(clusterName)
	spec := talosconfig.TypedSpec().Value

	context := configBundle.TalosCfg.Contexts[configBundle.TalosCfg.Context]

	spec.Ca = context.CA
	spec.Crt = context.Crt
	spec.Key = context.Key

	clusterStatus := omni.NewClusterStatus(clusterName)
	clusterStatus.TypedSpec().Value.Available = true

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus), state.WithCreateOwner((&omnictrl.ClusterStatusController{}).Name()))

	suite.Require().NoError(suite.state.Create(suite.ctx, talosconfig))

	clusterEndpoint := omni.NewClusterEndpoint(clusterName)
	clusterEndpoint.TypedSpec().Value.ManagementAddresses = []string{"localhost"}
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterEndpoint))

	c1, err := clientFactory.Get(suite.ctx, clusterName)
	suite.Require().NoError(err)

	c2, err := clientFactory.Get(suite.ctx, clusterName)
	suite.Require().NoError(err)

	suite.Assert().Same(c1, c2)
}

func (suite *ClientsSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestClients(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ClientsSuite{})
}
