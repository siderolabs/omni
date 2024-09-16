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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineRequestSetPressureSuite struct {
	OmniSuite
}

func (suite *MachineRequestSetPressureSuite) TestReconcile() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.ctx, time.Second*10)

	suite.startRuntime()

	controller := omnictrl.NewMachineRequestSetPressureController()

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	mrs := omni.NewMachineRequestSet(resources.DefaultNamespace, "machine-request-set")

	owner := omnictrl.NewMachineProvisionController().Name()

	suite.Require().NoError(mrs.Metadata().SetOwner(owner))

	suite.Require().NoError(suite.state.Create(suite.ctx, mrs, state.WithCreateOwner(owner)))

	mrsp := omni.NewMachineRequestSetPressure(resources.DefaultNamespace, "machine-request-set")

	assertNoResource(&suite.OmniSuite, mrsp)

	mc := omni.NewMachineClass(resources.DefaultNamespace, mrs.Metadata().ID())
	mc.TypedSpec().Value.AutoProvision = &specs.MachineClassSpec_Provision{
		ProviderId: "test",
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, mc))

	ms1 := omni.NewMachineSetStatus(resources.DefaultNamespace, "ms-1")
	ms2 := omni.NewMachineSetStatus(resources.DefaultNamespace, "ms-2")
	ms3 := omni.NewMachineSetStatus(resources.DefaultNamespace, "ms-3")

	ms1.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name:         mrs.Metadata().ID(),
		MachineCount: 1,
	}
	ms1.TypedSpec().Value.Machines = &specs.Machines{}
	ms2.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name:         mrs.Metadata().ID(),
		MachineCount: 2,
	}
	ms2.TypedSpec().Value.Machines = &specs.Machines{}
	ms3.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name:         "some-other-mc",
		MachineCount: 3,
	}
	ms3.TypedSpec().Value.Machines = &specs.Machines{}

	suite.Require().NoError(suite.state.Create(suite.ctx, ms1))
	suite.Require().NoError(suite.state.Create(suite.ctx, ms2))
	suite.Require().NoError(suite.state.Create(suite.ctx, ms3))

	assertResource(&suite.OmniSuite, mrsp.Metadata(), func(r *omni.MachineRequestSetPressure, assertion *assert.Assertions) {
		assertion.Equal(uint32(3), r.TypedSpec().Value.RequiredMachines)
	})

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, ms2.Metadata(), func(r *omni.MachineSetStatus) error {
		r.TypedSpec().Value.MachineAllocation.MachineCount = 0

		return nil
	})
	suite.Require().NoError(err)

	assertResource(&suite.OmniSuite, mrsp.Metadata(), func(r *omni.MachineRequestSetPressure, assertion *assert.Assertions) {
		assertion.Equal(uint32(1), r.TypedSpec().Value.RequiredMachines)
	})

	rtestutils.Destroy[*omni.MachineSetStatus](suite.ctx, suite.T(), suite.state, []string{ms1.Metadata().ID()})

	assertResource(&suite.OmniSuite, mrsp.Metadata(), func(r *omni.MachineRequestSetPressure, assertion *assert.Assertions) {
		assertion.Equal(uint32(0), r.TypedSpec().Value.RequiredMachines)
	})

	_, err = suite.state.Teardown(suite.ctx, mrs.Metadata(), state.WithTeardownOwner(owner))
	suite.Require().NoError(err)

	_, err = suite.state.WatchFor(suite.ctx, mrs.Metadata(), state.WithFinalizerEmpty())

	suite.Require().NoError(err)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, mrs.Metadata(), state.WithDestroyOwner(owner)))

	rtestutils.Destroy[*omni.MachineRequestSet](suite.ctx, suite.T(), suite.state, []string{mrs.Metadata().ID()})

	assertNoResource(&suite.OmniSuite, mrsp)
}

func TestMachineRequestSetPressureSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineRequestSetPressureSuite))
}
