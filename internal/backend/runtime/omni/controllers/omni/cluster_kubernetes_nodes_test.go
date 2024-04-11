// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterKubernetesNodesSuite struct {
	OmniSuite
}

func (suite *ClusterKubernetesNodesSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterKubernetesNodesController()))

	clusterUUID := omni.NewClusterUUID("test-cluster")

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterUUID))

	assertResource[*omni.ClusterKubernetesNodes](&suite.OmniSuite, omni.NewClusterKubernetesNodes(resources.DefaultNamespace, "test-cluster").Metadata(),
		func(r *omni.ClusterKubernetesNodes, assertion *assert.Assertions) {
			assertion.Empty(r.TypedSpec().Value.Nodes)
		})

	// add a single node

	identity1 := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "test-node1")

	identity1.Metadata().Labels().Set(omni.LabelCluster, "test-cluster")

	identity1.TypedSpec().Value.Nodename = "bbb"

	suite.Require().NoError(suite.state.Create(suite.ctx, identity1))

	assertResource[*omni.ClusterKubernetesNodes](&suite.OmniSuite, omni.NewClusterKubernetesNodes(resources.DefaultNamespace, "test-cluster").Metadata(),
		func(r *omni.ClusterKubernetesNodes, assertion *assert.Assertions) {
			assertion.Equal([]string{"bbb"}, r.TypedSpec().Value.Nodes)
		})

	// add another node - assert that the nodeIds are sorted

	identity2 := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "test-node2")

	identity2.Metadata().Labels().Set(omni.LabelCluster, "test-cluster")

	identity2.TypedSpec().Value.Nodename = "aaa"

	suite.Require().NoError(suite.state.Create(suite.ctx, identity2))

	assertResource[*omni.ClusterKubernetesNodes](&suite.OmniSuite, omni.NewClusterKubernetesNodes(resources.DefaultNamespace, "test-cluster").Metadata(),
		func(r *omni.ClusterKubernetesNodes, assertion *assert.Assertions) {
			assertion.Equal([]string{"aaa", "bbb"}, r.TypedSpec().Value.Nodes) // assert that it is sorted
		})
}

func TestClusterKubernetesNodesSuite(t *testing.T) {
	suite.Run(t, new(ClusterKubernetesNodesSuite))
}
