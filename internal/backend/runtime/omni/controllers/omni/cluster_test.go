// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterSuite struct {
	OmniSuite
}

func (suite *ClusterSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterController()))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterConfigVersionController()))

	clusterName := "talos-default-1"

	cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)

	machines := make([]*omni.ClusterMachine, 10)

	for i := range machines {
		machine := omni.NewClusterMachine(
			resources.DefaultNamespace,
			fmt.Sprintf("node-%d", i),
		)

		machine.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
		machine.Metadata().Labels().Set(omni.LabelCluster, clusterName)

		machines[i] = machine

		suite.Require().NoError(suite.state.Create(suite.ctx, machine))
	}

	suite.Assert().NoError(suite.state.Create(suite.ctx, cluster))

	assertResource(
		&suite.OmniSuite,
		*omni.NewCluster(resources.DefaultNamespace, cluster.Metadata().ID()).Metadata(),
		func(res *omni.Cluster, assertions *assert.Assertions) {
			assertions.False(res.Metadata().Finalizers().Empty())
		},
	)

	suite.Require().Error(suite.state.Destroy(suite.ctx, cluster.Metadata()))

	for _, machine := range machines {
		suite.Require().NoError(suite.state.Destroy(suite.ctx, machine.Metadata()))
	}

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			canDestroy, err := suite.state.Teardown(suite.ctx, cluster.Metadata())
			if err != nil {
				return err
			}

			if !canDestroy {
				return retry.ExpectedErrorf("the resource is not ready to be destroyed")
			}

			return retry.ExpectedError(suite.state.Destroy(suite.ctx, cluster.Metadata()))
		}))
}

func TestClusterSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterSuite))
}
