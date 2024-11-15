// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ManagedRequestSetSuite struct {
	OmniSuite
}

func (suite *ManagedRequestSetSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewManagedControlPlaneController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewManagedRequestSetController(omnictrl.ProviderConfig{
		ID:   "kubevirt",
		Data: "{}",
	})))

	cluster := omni.NewCluster(resources.DefaultNamespace, "cluster")
	cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
		UseManagedControlPlanes: true,
	}

	id := omni.ControlPlanesResourceID(cluster.Metadata().ID())

	suite.Require().NoError(suite.state.Create(ctx, cluster))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{id},
		func(machineRequestSet *omni.MachineRequestSet, assert *assert.Assertions) {
			_, ok := machineRequestSet.Metadata().Labels().Get(omni.LabelNoManualAllocation)
			assert.True(ok)

			assert.EqualValues(3, machineRequestSet.TypedSpec().Value.MachineCount)
			assert.Equal(cluster.TypedSpec().Value.TalosVersion, machineRequestSet.TypedSpec().Value.TalosVersion)
			assert.Equal("kubevirt", machineRequestSet.TypedSpec().Value.ProviderId)
			assert.Equal("{}", machineRequestSet.TypedSpec().Value.ProviderData)
		},
	)

	rtestutils.Destroy[*omni.Cluster](ctx, suite.T(), suite.state, []resource.ID{cluster.Metadata().ID()})

	rtestutils.AssertNoResource[*omni.MachineRequestSet](ctx, suite.T(), suite.state, id)
}

func TestManagedRequestSetSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ManagedRequestSetSuite))
}
