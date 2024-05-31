// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package dns_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/dns"
)

const cluster = "test-cluster-1"

type ServiceSuite struct {
	suite.Suite
	state state.State

	dnsService *dns.Service
	logger     *zap.Logger

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	eg errgroup.Group
}

func (suite *ServiceSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	suite.logger = zaptest.NewLogger(suite.T(), zaptest.WrapOptions(zap.AddCaller()))

	suite.dnsService = dns.NewService(suite.state, suite.logger)

	// create a ClusterMachineIdentity before starting the DNS service to make sure it picks the existing records
	bootstrapIdentity := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "test-bootstrap-1")
	bootstrapIdentity.Metadata().Labels().Set(omni.LabelCluster, "test-bootstrap-cluster-1")

	bootstrapIdentity.TypedSpec().Value.Nodename = "test-bootstrap-1-node"
	bootstrapIdentity.TypedSpec().Value.NodeIps = []string{"10.0.0.84"}

	suite.Require().NoError(suite.state.Create(suite.ctx, bootstrapIdentity))

	suite.eg.Go(func() error {
		return suite.dnsService.Start(suite.ctx)
	})
}

func (suite *ServiceSuite) TearDownTest() {
	suite.logger.Info("tear down")

	suite.ctxCancel()

	suite.Require().NoError(suite.eg.Wait())
}

func (suite *ServiceSuite) TestResolve() {
	suite.assertResolveAddress("test-bootstrap-cluster-1", "test-bootstrap-1-node", "10.0.0.84")

	identity := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "test-1")
	identity.Metadata().Labels().Set(omni.LabelCluster, "test-cluster-1")

	identity.TypedSpec().Value.Nodename = "test-1-node"
	identity.TypedSpec().Value.NodeIps = []string{"10.0.0.42"}

	// create and assert that it resolves by machine ID, address and node name
	suite.Require().NoError(suite.state.Create(suite.ctx, identity))

	expected := dns.Info{
		Cluster: cluster,
		ID:      "test-1",
		Name:    "test-1-node",
		Address: "10.0.0.42",
	}
	suite.assertResolve("test-1", expected)
	suite.assertResolve("test-1-node", expected)
	suite.assertResolve("10.0.0.42", expected)

	// update address, assert that it resolves with new address and old address doesn't resolve anymore
	identity.TypedSpec().Value.NodeIps = []string{"10.0.0.43"}

	suite.Require().NoError(suite.state.Update(suite.ctx, identity))

	expected = dns.Info{
		Cluster: cluster,
		ID:      "test-1",
		Name:    "test-1-node",
		Address: "10.0.0.43",
	}

	suite.assertResolve("test-1", expected)
	suite.assertResolve("test-1-node", expected)
	suite.assertResolve("10.0.0.43", expected)

	var zeroInfo dns.Info

	suite.assertResolve("10.0.0.42", zeroInfo)

	// update Talos version, assert that it resolves
	configStatus := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "test-1")
	configStatus.TypedSpec().Value.TalosVersion = "1.4.1"

	suite.Require().NoError(suite.state.Create(suite.ctx, configStatus))

	expected.TalosVersion = "1.4.1"

	suite.assertResolve("test-1", expected)

	// destroy the identity, assert that it doesn't resolve anymore
	suite.Require().NoError(suite.state.Destroy(suite.ctx, identity.Metadata()))

	suite.assertResolve("test-1", zeroInfo)
	suite.assertResolve("test-1-node", zeroInfo)
	suite.assertResolve("10.0.0.42", zeroInfo)
}

func (suite *ServiceSuite) assertResolveAddress(cluster, node, expected string) {
	err := retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(suite.ctx, func(context.Context) error {
		resolved := suite.dnsService.Resolve(cluster, node)

		if resolved.Address != expected {
			return retry.ExpectedErrorf("expected %s, got %s", expected, resolved)
		}

		return nil
	})
	suite.Assert().NoError(err)
}

func (suite *ServiceSuite) assertResolve(node string, expected dns.Info) {
	err := retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(suite.ctx, func(context.Context) error {
		resolved := suite.dnsService.Resolve(cluster, node)

		if !reflect.DeepEqual(resolved, expected) {
			return retry.ExpectedErrorf("expected %#v, got %#v", expected, resolved)
		}

		return nil
	})
	suite.Assert().NoError(err)
}

func TestServiceSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ServiceSuite))
}
