// Copyright (c) 2024 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/client/pkg/omni/resources"
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

	machineClass := omni.NewMachineClass(resources.DefaultNamespace, "test")

	suite.Require().NoError(suite.state.Create(ctx, machineClass))

	rtestutils.AssertNoResource[*omni.MachineRequestSet](ctx, suite.T(), suite.state, "test")

	var err error

	machineClass, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineClass.Metadata(), func(r *omni.MachineClass) error {
		r.TypedSpec().Value.AutoProvision = &specs.MachineClassSpec_Provision{
			ProviderId:   "test",
			TalosVersion: "v1.7.6",
			Extensions: []string{
				"hello-world-extension",
			},
			MetaValues: []*specs.MetaValue{
				{
					Key:   0,
					Value: "hi",
				},
			},
			KernelArgs:       []string{"a=b"},
			IdleMachineCount: 4,
		}

		return nil
	})

	suite.Require().NoError(err)

	autoProvision := machineClass.TypedSpec().Value.AutoProvision

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineClass.Metadata().ID()}, func(
		mrs *omni.MachineRequestSet, assert *assert.Assertions,
	) {
		assert.EqualValues(autoProvision.Extensions, mrs.TypedSpec().Value.Extensions)
		assert.EqualValues(autoProvision.ProviderId, mrs.TypedSpec().Value.ProviderId)
		assert.EqualValues(autoProvision.KernelArgs, mrs.TypedSpec().Value.KernelArgs)
		assert.EqualValues(autoProvision.TalosVersion, mrs.TypedSpec().Value.TalosVersion)
		assert.EqualValues(autoProvision.MetaValues, mrs.TypedSpec().Value.MetaValues)

		assert.EqualValues(4, mrs.TypedSpec().Value.MachineCount)
	})

	pressure := omni.NewMachineRequestSetPressure(resources.DefaultNamespace, machineClass.Metadata().ID())

	pressure.TypedSpec().Value.RequiredMachines = 6

	suite.Require().NoError(suite.state.Create(ctx, pressure))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineClass.Metadata().ID()}, func(
		mrs *omni.MachineRequestSet, assert *assert.Assertions,
	) {
		assert.EqualValues(10, mrs.TypedSpec().Value.MachineCount)
	})

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, pressure.Metadata(), func(r *omni.MachineRequestSetPressure) error {
		r.TypedSpec().Value.RequiredMachines = 2

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineClass.Metadata().ID()}, func(
		mrs *omni.MachineRequestSet, assert *assert.Assertions,
	) {
		assert.EqualValues(6, mrs.TypedSpec().Value.MachineCount)
	})
}

func TestMachineProvisionControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineProvisionControllerSuite))
}
