// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineProvisionControllerSuite struct {
	OmniSuite
}

func (suite *MachineProvisionControllerSuite) TestReconcile() {
	require := suite.Require()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.startRuntime()

	require.NoError(suite.runtime.RegisterQController(omnictrl.NewMachineProvisionController()))

	machineClass := omni.NewMachineClass("test")

	machineSet := omni.NewMachineSet("ms1")
	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name:         machineClass.Metadata().ID(),
		MachineCount: 4,
	}

	cluster := omni.NewCluster("cluster")

	cluster.TypedSpec().Value.TalosVersion = "1.7.7"

	machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

	suite.Require().NoError(suite.state.Create(ctx, machineClass))
	suite.Require().NoError(suite.state.Create(ctx, cluster))
	suite.Require().NoError(suite.state.Create(ctx, machineSet))

	rtestutils.AssertNoResource[*omni.MachineRequestSet](ctx, suite.T(), suite.state, "test")

	var err error

	machineClass, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineClass.Metadata(), func(r *omni.MachineClass) error {
		r.TypedSpec().Value.AutoProvision = &specs.MachineClassSpec_Provision{
			ProviderId: "test",
			MetaValues: []*specs.MetaValue{
				{
					Key:   0,
					Value: "hi",
				},
			},
			KernelArgs: []string{"a=b"},
		}

		return nil
	})

	suite.Require().NoError(err)

	autoProvision := machineClass.TypedSpec().Value.AutoProvision

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineSet.Metadata().ID()}, func(
		mrs *omni.MachineRequestSet, assert *assert.Assertions,
	) {
		assert.EqualValues(autoProvision.ProviderId, mrs.TypedSpec().Value.ProviderId)
		assert.EqualValues(autoProvision.KernelArgs, mrs.TypedSpec().Value.KernelArgs)
		assert.EqualValues(cluster.TypedSpec().Value.TalosVersion, mrs.TypedSpec().Value.TalosVersion)
		assert.EqualValues(autoProvision.MetaValues, mrs.TypedSpec().Value.MetaValues)

		assert.EqualValues(4, mrs.TypedSpec().Value.MachineCount)
	})

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(r *omni.MachineSet) error {
		r.TypedSpec().Value.MachineAllocation.MachineCount = 2

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineSet.Metadata().ID()}, func(
		mrs *omni.MachineRequestSet, assert *assert.Assertions,
	) {
		assert.EqualValues(2, mrs.TypedSpec().Value.MachineCount)
	})

	rtestutils.Destroy[*omni.MachineSet](ctx, suite.T(), suite.state, []string{machineSet.Metadata().ID()})

	rtestutils.AssertNoResource[*omni.MachineRequestSet](ctx, suite.T(), suite.state, machineSet.Metadata().ID())
}

func TestMachineProvisionControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineProvisionControllerSuite))
}
