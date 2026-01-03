// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type NodeUniqueTokenCleanupControllerSuite struct {
	OmniSuite
}

func (suite *MachineStatusSnapshotControllerSuite) TestNodeUniqueTokenCleanup() {
	// Register the controller and start the runtime
	controller := omni.NewNodeUniqueTokenCleanupController(time.Millisecond * 100)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	token := siderolink.NewNodeUniqueToken("1")

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	suite.Require().NoError(suite.state.Create(ctx, token))

	rtestutils.AssertNoResource[*siderolink.NodeUniqueToken](ctx, suite.T(), suite.state, token.Metadata().ID())

	link := siderolink.NewLink(token.Metadata().ID(), &specs.SiderolinkSpec{})

	suite.Require().NoError(suite.state.Create(ctx, token))
	suite.Require().NoError(suite.state.Create(ctx, link))

	time.Sleep(time.Second)

	// resource still exists
	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{token.Metadata().ID()},
		func(res *siderolink.NodeUniqueToken, assert *assert.Assertions) {},
	)

	finalizer := "test-finalizer"

	suite.Require().NoError(suite.state.AddFinalizer(ctx, token.Metadata(), finalizer))

	suite.Require().NoError(suite.state.Destroy(ctx, link.Metadata()))

	rtestutils.AssertResource(ctx, suite.T(), suite.state, token.Metadata().ID(),
		func(res *siderolink.NodeUniqueToken, assert *assert.Assertions) {
			assert.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
		},
	)

	suite.Require().NoError(suite.state.RemoveFinalizer(ctx, token.Metadata(), finalizer))

	rtestutils.AssertNoResource[*siderolink.NodeUniqueToken](ctx, suite.T(), suite.state, token.Metadata().ID())
}

func TestNodeUniqueTokenCleanupControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(NodeUniqueTokenCleanupControllerSuite))
}
