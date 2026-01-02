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

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineRequestLinkSuite struct {
	OmniSuite
}

func (suite *MachineRequestLinkSuite) TestReconcile() {
	require := suite.Require()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	suite.startRuntime()

	require.NoError(suite.runtime.RegisterQController(omnictrl.NewMachineRequestLinkController(suite.state)))

	uuid := "aabb"
	requestID := "request-1"

	status := infra.NewMachineRequestStatus(requestID)
	status.TypedSpec().Value.Id = uuid

	suite.Require().NoError(suite.state.Create(ctx, status))
	suite.Require().NoError(suite.state.Create(ctx, siderolink.NewLink(uuid, nil)))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{uuid}, func(link *siderolink.Link, assert *assert.Assertions) {
		request, ok := link.Metadata().Labels().Get(omni.LabelMachineRequest)

		assert.True(ok)
		assert.Equal(requestID, request)
	})
}

func TestMachineRequestLinkSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineRequestLinkSuite))
}
