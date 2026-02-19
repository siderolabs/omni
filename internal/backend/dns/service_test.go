// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//nolint:goconst
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
	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.T().Context(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	suite.logger = zaptest.NewLogger(suite.T(), zaptest.WrapOptions(zap.AddCaller()))

	suite.dnsService = dns.NewService(suite.state, suite.logger)

	// create a ClusterMachineIdentity before starting the DNS service to make sure it picks the existing records
	bootstrapIdentity := omni.NewClusterMachineIdentity("test-bootstrap-1")
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
	suite.assertResolve("test-bootstrap-cluster-1", "test-bootstrap-1-node", dns.Info{Cluster: "test-bootstrap-cluster-1", ID: "test-bootstrap-1", Name: "test-bootstrap-1-node", Address: "10.0.0.84"})

	identity := omni.NewClusterMachineIdentity("test-1")
	identity.Metadata().Labels().Set(omni.LabelCluster, "test-cluster-1")

	identity.TypedSpec().Value.Nodename = "test-1-node"
	identity.TypedSpec().Value.NodeIps = []string{"10.0.0.42"}

	// create and assert that it resolves by machine ID, address and node name
	suite.Require().NoError(suite.state.Create(suite.ctx, identity))

	expected := dns.Info{Cluster: cluster, ID: "test-1", Name: "test-1-node", Address: "10.0.0.42"}
	suite.assertResolve(cluster, "test-1", expected)
	suite.assertResolve(cluster, "test-1-node", expected)
	suite.assertResolve(cluster, "10.0.0.42", expected)

	// update address, assert that it resolves with new address and old address doesn't resolve anymore
	identity.TypedSpec().Value.NodeIps = []string{"10.0.0.43"}

	suite.Require().NoError(suite.state.Update(suite.ctx, identity))

	expected = dns.Info{Cluster: cluster, ID: "test-1", Name: "test-1-node", Address: "10.0.0.43"}
	suite.assertResolve(cluster, "test-1", expected)
	suite.assertResolve(cluster, "test-1-node", expected)
	suite.assertResolve(cluster, "10.0.0.43", expected)

	suite.assertNotFound("10.0.0.42")

	// update Talos version, assert that it resolves
	machineStatus := omni.NewMachineStatus("test-1")
	machineStatus.TypedSpec().Value.TalosVersion = "1.4.1"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	expected.TalosVersion = "1.4.1"

	suite.assertResolve(cluster, "test-1", expected)

	// destroy the identity, assert that it doesn't resolve anymore
	suite.Require().NoError(suite.state.Destroy(suite.ctx, identity.Metadata()))

	expected = dns.Info{ID: "test-1"}
	expected.TalosVersion = machineStatus.TypedSpec().Value.TalosVersion

	// still resolves by the node id, but has an empty address
	suite.assertResolve(cluster, "test-1", expected)
	suite.assertNotFound("test-1-node")
	suite.assertNotFound("10.0.0.43")

	// destroy the machine status, assert that it doesn't resolve by the node id anymore
	suite.Require().NoError(suite.state.Destroy(suite.ctx, machineStatus.Metadata()))

	suite.assertNotFound("test-1")
}

func (suite *ServiceSuite) TestResolveAllocateAndDeallocate() {
	expected := dns.Info{ID: "test-1"}

	expected.TalosVersion = "3.2.1"

	// In the maintenance mode, we only have MachineStatus, so we start with that
	// (means cache will be initialized with the data on MachineStatus and nothing else - no ClusterMachineIdentity)
	machineStatus := omni.NewMachineStatus("test-1")

	machineStatus.TypedSpec().Value.TalosVersion = "3.2.1"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	suite.assertResolve(cluster, "test-1", expected)

	// allocate the machine to a cluster by creating a ClusterMachineIdentity

	identity := omni.NewClusterMachineIdentity("test-1")
	identity.Metadata().Labels().Set(omni.LabelCluster, "test-cluster-1")

	identity.TypedSpec().Value.Nodename = "test-1-node"

	suite.Require().NoError(suite.state.Create(suite.ctx, identity))

	// assert that cluster information gets resolved

	expected.Cluster = "test-cluster-1"
	expected.Name = "test-1-node"

	suite.assertResolve(cluster, "test-1", expected)

	// deallocate the machine by destroying the ClusterMachineIdentity

	suite.Require().NoError(suite.state.Destroy(suite.ctx, identity.Metadata()))

	// assert that the machine still resolves but the cluster information is gone

	expected.Cluster = ""
	expected.Name = ""

	suite.assertResolve(cluster, "test-1", expected)
}

func (suite *ServiceSuite) assertResolve(clusterName, node string, expected dns.Info) {
	err := retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(suite.ctx, func(context.Context) error {
		resolved, resolveErr := suite.dnsService.Resolve(clusterName, node)
		if resolveErr != nil {
			return retry.ExpectedErrorf("resolve error: %v", resolveErr)
		}

		if !reflect.DeepEqual(resolved, expected) {
			return retry.ExpectedErrorf("expected %#v, got %#v", expected, resolved)
		}

		return nil
	})
	suite.Assert().NoError(err)
}

func (suite *ServiceSuite) assertNotFound(node string) {
	err := retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(suite.ctx, func(context.Context) error {
		_, resolveErr := suite.dnsService.Resolve(cluster, node)
		if resolveErr == nil {
			return retry.ExpectedErrorf("expected node %q to not be found, but it resolved successfully", node)
		}

		return nil
	})
	suite.Assert().NoError(err)
}

func TestServiceSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ServiceSuite))
}
