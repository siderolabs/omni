// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterLoadBalancerSuite struct {
	OmniSuite
}

func (suite *ClusterLoadBalancerSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterEndpointController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterLoadBalancerController(5000, 6000)))

	siderolinkConfig := siderolink.NewConfig(resources.DefaultNamespace)
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolinkConfig))

	clusterName := "talos-default-3"

	cluster, _ := suite.createCluster(clusterName, 1, 1)

	md := *omni.NewLoadBalancerConfig(resources.DefaultNamespace, cluster.Metadata().ID()).Metadata()

	// remove the LB config created by the mock setup to let the controller handle it
	suite.Require().NoError(suite.state.Destroy(suite.ctx, md))

	assertResource(
		&suite.OmniSuite,
		md,
		func(res *omni.LoadBalancerConfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			assertions.NotEmpty(spec.Endpoints, "no endpoints in the loadbalancer config")
			assertions.NotEmpty(spec.SiderolinkEndpoint, "no siderolink endpoint is set on the load balancer")
			v, err := strconv.ParseInt(spec.BindPort, 10, 64)
			assertions.NoError(err, "failed to parse bind port")
			assertions.EqualValues(5000, v, "the port should be 5000")
		},
	)

	// create another cluster and see if the next loadbalancer port will be used for it

	secondClusterName := "talos-default-4"

	secondCluster, _ := suite.createCluster(secondClusterName, 1, 1)

	secondMD := *omni.NewLoadBalancerConfig(resources.DefaultNamespace, secondCluster.Metadata().ID()).Metadata()

	// remove the LB config created by the mock setup to let the controller handle it
	suite.Require().NoError(suite.state.Destroy(suite.ctx, secondMD))

	assertResource(
		&suite.OmniSuite,
		secondMD,
		func(res *omni.LoadBalancerConfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			v, err := strconv.ParseInt(spec.BindPort, 10, 64)
			assertions.NoError(err)
			assertions.EqualValues(5001, v, "the port should be 5001")
		},
	)

	suite.destroyCluster(cluster)

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertNoResource(md),
	))

	// recreate load balancer and see if it will reuse previously used port 5000
	cluster, machines := suite.createCluster(clusterName, 1, 1)

	ids := xslices.Map(machines, func(r *omni.ClusterMachine) string {
		return r.Metadata().ID()
	})

	// wait for cluster machine statuses to be created
	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, ids, func(*omni.ClusterMachineStatus, *assert.Assertions) {})

	md = *omni.NewLoadBalancerConfig(resources.DefaultNamespace, cluster.Metadata().ID()).Metadata()

	// remove the LB config created by the mock setup to let the controller handle it
	suite.Require().NoError(suite.state.Destroy(suite.ctx, md))

	assertResource(
		&suite.OmniSuite,
		md,
		func(res *omni.LoadBalancerConfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			v, err := strconv.ParseInt(spec.BindPort, 10, 64)
			assertions.NoError(err)
			assertions.EqualValues(5000, v, "the port should be 5000")
		},
	)

	suite.destroyCluster(cluster)
}

func TestClusterLoadbalancerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterLoadBalancerSuite))
}
