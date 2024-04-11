// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"encoding/json"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterIdentitySuite struct {
	OmniSuite
}

func (suite *ClusterIdentitySuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterIdentityController()))

	clusterSecrets := omni.NewClusterSecrets(resources.DefaultNamespace, "test-cluster")

	bundle, err := secrets.NewBundle(secrets.NewClock(), config.TalosVersionCurrent)
	suite.Require().NoError(err)

	data, err := json.Marshal(bundle)
	suite.Require().NoError(err)

	clusterSecrets.TypedSpec().Value.Data = data

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterSecrets))

	assertResource[*omni.ClusterIdentity](&suite.OmniSuite, omni.NewClusterIdentity(resources.DefaultNamespace, "test-cluster").Metadata(),
		func(r *omni.ClusterIdentity, assertion *assert.Assertions) {
			assertion.Equal(bundle.Cluster.ID, r.TypedSpec().Value.ClusterId)
		})

	// add a single node

	clusterMachineIdentity1 := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "test-node1")

	clusterMachineIdentity1.Metadata().Labels().Set(omni.LabelCluster, "test-cluster")

	clusterMachineIdentity1.TypedSpec().Value.NodeIdentity = "bbb"

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachineIdentity1))

	assertResource[*omni.ClusterIdentity](&suite.OmniSuite, omni.NewClusterIdentity(resources.DefaultNamespace, "test-cluster").Metadata(),
		func(r *omni.ClusterIdentity, assertion *assert.Assertions) {
			assertion.Equal(bundle.Cluster.ID, r.TypedSpec().Value.ClusterId)
			assertion.Equal([]string{"bbb"}, r.TypedSpec().Value.NodeIds)
		})

	// add another node - assert that the nodeIds are sorted

	clusterMachineIdentity2 := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "test-node2")

	clusterMachineIdentity2.Metadata().Labels().Set(omni.LabelCluster, "test-cluster")

	clusterMachineIdentity2.TypedSpec().Value.NodeIdentity = "aaa"

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachineIdentity2))

	assertResource[*omni.ClusterIdentity](&suite.OmniSuite, omni.NewClusterIdentity(resources.DefaultNamespace, "test-cluster").Metadata(),
		func(r *omni.ClusterIdentity, assertion *assert.Assertions) {
			assertion.Equal(bundle.Cluster.ID, r.TypedSpec().Value.ClusterId)
			assertion.Equal([]string{"aaa", "bbb"}, r.TypedSpec().Value.NodeIds) // assert that it is sorted
		})
}

func TestClusterIdentitySuite(t *testing.T) {
	suite.Run(t, new(ClusterIdentitySuite))
}
