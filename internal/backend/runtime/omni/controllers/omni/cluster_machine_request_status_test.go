// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterMachineRequestStatusSuite struct {
	OmniSuite
}

func (suite *ClusterMachineRequestStatusSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*30)
	defer cancel()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineRequestStatusController()))

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "test")

	machineSet.Metadata().Labels().Set(omni.LabelCluster, "test")
	machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	infraProviderStatus := infra.NewProviderStatus("talemu")

	infraProviderStatus.TypedSpec().Value.Name = "Talemu"
	infraProviderStatus.TypedSpec().Value.Description = "Fake Talemu"
	infraProviderStatus.TypedSpec().Value.Icon = "<svg/>"

	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name: "talemu-1",
	}

	machineRequest := infra.NewMachineRequest("test")
	machineRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, infraProviderStatus.Metadata().ID())
	machineRequest.Metadata().Labels().Set(omni.LabelMachineRequestSet, machineSet.Metadata().ID())

	suite.Require().NoError(suite.state.Create(ctx, machineSet))
	suite.Require().NoError(suite.state.Create(ctx, machineRequest))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineRequest.Metadata().ID()}, func(res *omni.ClusterMachineRequestStatus, assert *assert.Assertions) {
		assert.Equal(specs.ClusterMachineRequestStatusSpec_PENDING, res.TypedSpec().Value.Stage)

		value, ok := res.Metadata().Labels().Get(omni.LabelMachineSet)
		assert.True(ok)

		assert.Equal(machineSet.Metadata().ID(), value)

		value, ok = res.Metadata().Labels().Get(omni.LabelCluster)
		assert.True(ok)

		assert.Equal("test", value)

		_, ok = res.Metadata().Labels().Get(omni.LabelControlPlaneRole)
		assert.True(ok)
	})

	suite.Require().NoError(suite.state.Create(ctx, infraProviderStatus))

	rtestutils.Destroy[*infra.MachineRequest](ctx, suite.T(), suite.state, []string{machineRequest.Metadata().ID()})

	rtestutils.AssertNoResource[*omni.ClusterMachineRequestStatus](ctx, suite.T(), suite.state, machineRequest.Metadata().ID())
}

func TestClusterMachineRequestStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterMachineRequestStatusSuite))
}
