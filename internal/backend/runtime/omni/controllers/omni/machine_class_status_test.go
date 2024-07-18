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

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineClassStatusSuite struct {
	OmniSuite
}

func (suite *MachineClassStatusSuite) TestReconcile() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.ctx, time.Second*10)

	suite.startRuntime()

	controller := omnictrl.NewMachineClassStatusController()

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	mc := omni.NewMachineClass(resources.DefaultNamespace, "machine-class-test")

	suite.Require().NoError(suite.state.Create(suite.ctx, mc))

	mcs := omni.NewMachineClassStatus(resources.DefaultNamespace, "machine-class-test")

	assertResource(&suite.OmniSuite, mcs.Metadata(), func(r *omni.MachineClassStatus, assertion *assert.Assertions) {
		assertion.Equal(uint32(0), r.TypedSpec().Value.RequiredAdditionalMachines)
	})

	msrm1 := omni.NewMachineSetRequiredMachines(resources.DefaultNamespace, "msrm-1")
	msrm2 := omni.NewMachineSetRequiredMachines(resources.DefaultNamespace, "msrm-2")
	msrm3 := omni.NewMachineSetRequiredMachines(resources.DefaultNamespace, "msrm-3")

	msrm1.TypedSpec().Value.RequiredAdditionalMachines = 1
	msrm2.TypedSpec().Value.RequiredAdditionalMachines = 2
	msrm3.TypedSpec().Value.RequiredAdditionalMachines = 3

	msrm1.Metadata().Labels().Set(omni.LabelMachineClassName, "machine-class-test")
	msrm2.Metadata().Labels().Set(omni.LabelMachineClassName, "machine-class-test")
	msrm3.Metadata().Labels().Set(omni.LabelMachineClassName, "some-other-mc")

	suite.Require().NoError(suite.state.Create(suite.ctx, msrm1))
	suite.Require().NoError(suite.state.Create(suite.ctx, msrm2))
	suite.Require().NoError(suite.state.Create(suite.ctx, msrm3))

	assertResource(&suite.OmniSuite, mcs.Metadata(), func(r *omni.MachineClassStatus, assertion *assert.Assertions) {
		assertion.Equal(uint32(3), r.TypedSpec().Value.RequiredAdditionalMachines)
	})

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, msrm2.Metadata(), func(r *omni.MachineSetRequiredMachines) error {
		r.TypedSpec().Value.RequiredAdditionalMachines = 0

		return nil
	})
	suite.Require().NoError(err)

	assertResource(&suite.OmniSuite, mcs.Metadata(), func(r *omni.MachineClassStatus, assertion *assert.Assertions) {
		assertion.Equal(uint32(1), r.TypedSpec().Value.RequiredAdditionalMachines)
	})

	rtestutils.Destroy[*omni.MachineSetRequiredMachines](suite.ctx, suite.T(), suite.state, []string{msrm1.Metadata().ID()})

	assertResource(&suite.OmniSuite, mcs.Metadata(), func(r *omni.MachineClassStatus, assertion *assert.Assertions) {
		assertion.Equal(uint32(0), r.TypedSpec().Value.RequiredAdditionalMachines)
	})

	rtestutils.Destroy[*omni.MachineClass](suite.ctx, suite.T(), suite.state, []string{mc.Metadata().ID()})

	assertNoResource[*omni.MachineClassStatus](&suite.OmniSuite, mcs)
}

func TestMachineClassStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineClassStatusSuite))
}
