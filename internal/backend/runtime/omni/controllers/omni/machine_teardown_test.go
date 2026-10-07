// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineTeardownSuite struct {
	OmniSuite
}

func (suite *MachineTeardownSuite) TestSkipDisconnected() {
	var disksCalled atomic.Bool

	suite.machineService.disksHook = func() {
		disksCalled.Store(true)
	}

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineTeardownController()))

	machineStatus := omni.NewMachineStatus("disconnected")
	machineStatus.TypedSpec().Value.Connected = false
	machineStatus.TypedSpec().Value.ManagementAddress = suite.socketConnectionString

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	rtestutils.AssertResource(suite.ctx, suite.T(), suite.state, machineStatus.Metadata().ID(), func(res *omni.MachineStatus, assertion *assert.Assertions) {
		assertion.True(res.Metadata().Finalizers().Has(omnictrl.MachineTeardownControllerName))
	})

	_, err := suite.state.Teardown(suite.ctx, machineStatus.Metadata())
	suite.Require().NoError(err)

	// Stay under the 10s wipe timeout so a dial that hangs fails this assertion.
	ctx, cancel := context.WithTimeout(suite.ctx, 2*time.Second)
	defer cancel()

	rtestutils.AssertResource(ctx, suite.T(), suite.state, machineStatus.Metadata().ID(), func(res *omni.MachineStatus, assertion *assert.Assertions) {
		assertion.False(res.Metadata().Finalizers().Has(omnictrl.MachineTeardownControllerName))
	})

	suite.False(disksCalled.Load())
}

func TestMachineTeardownSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineTeardownSuite))
}
