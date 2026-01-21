// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterWorkloadProxySuite struct {
	OmniSuite
}

func (suite *ClusterWorkloadProxySuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&omnictrl.ClusterWorkloadProxyController{}))

	clusterName := "cluster-workload-proxy-test"

	cluster, _ := suite.createCluster(clusterName, 1, 1)

	cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{EnableWorkloadProxy: true}

	suite.Require().NoError(suite.state.Update(suite.ctx, cluster))

	configPatchID := fmt.Sprintf("950-cluster-%s-workload-proxying", cluster.Metadata().ID())

	configPatch := omni.NewConfigPatch(configPatchID)

	assertResource(&suite.OmniSuite, configPatch.Metadata(), func(res *omni.ConfigPatch, assertion *assert.Assertions) {
		buffer, err := res.TypedSpec().Value.GetUncompressedData()
		assertion.NoError(err)

		defer buffer.Free()

		data := string(buffer.Data())

		assertion.Contains(data, "omni-kube-service-exposer")
	})

	cluster.TypedSpec().Value.GetFeatures().EnableWorkloadProxy = false

	suite.Require().NoError(suite.state.Update(suite.ctx, cluster))

	assertNoResource(&suite.OmniSuite, configPatch)

	suite.destroyCluster(cluster)
}

func TestClusterWorkloadProxySuite(t *testing.T) {
	suite.Run(t, new(ClusterWorkloadProxySuite))
}
