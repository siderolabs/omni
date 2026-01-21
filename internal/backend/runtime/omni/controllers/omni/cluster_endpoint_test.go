// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterEndpointSuite struct {
	OmniSuite
}

func (suite *ClusterEndpointSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController(suite.kubernetesRuntime)))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterEndpointController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))

	clusterName := "talos-default-44"

	cluster, _ := suite.createCluster(clusterName, 3, 1)

	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{clusterName},
		func(clusterEndpoint *omni.ClusterEndpoint, assert *assert.Assertions) {
			assert.Len(clusterEndpoint.TypedSpec().Value.ManagementAddresses, 3)

			if len(clusterEndpoint.TypedSpec().Value.ManagementAddresses) > 0 {
				assert.Equal(suite.socketConnectionString, clusterEndpoint.TypedSpec().Value.ManagementAddresses[0])
			}
		},
	)

	suite.destroyCluster(cluster)

	rtestutils.AssertNoResource[*omni.ClusterEndpoint](suite.ctx, suite.T(), suite.state, clusterName)
}

func TestClusterEndpointSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterEndpointSuite))
}
