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

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type InfraProviderConfigPatchControllerSuite struct {
	OmniSuite
}

func (suite *InfraProviderConfigPatchControllerSuite) TestReconcile() {
	require := suite.Require()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.startRuntime()

	require.NoError(suite.runtime.RegisterQController(omnictrl.NewInfraProviderConfigPatchController()))

	provider := "provider"
	id := "patch"
	requestID := "request-1"

	request := infra.NewConfigPatchRequest(id)
	request.Metadata().Labels().Set(omni.LabelMachineRequest, requestID)
	request.Metadata().Labels().Set(omni.LabelInfraProviderID, provider)

	status := infra.NewMachineRequestStatus(requestID)
	status.Metadata().Labels().Set(omni.LabelInfraProviderID, provider)

	suite.Require().NoError(request.TypedSpec().Value.SetUncompressedData([]byte("machine: {}")))

	// the UUID in the status is reported by the infra provider, no patch is created for a machine which is not bound to the machine request
	status.TypedSpec().Value.Id = "reported"

	// a patch request of another infra provider which points at the same machine request
	otherRequest := infra.NewConfigPatchRequest("other-patch")
	otherRequest.Metadata().Labels().Set(omni.LabelMachineRequest, requestID)
	otherRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, "other-provider")

	suite.Require().NoError(otherRequest.TypedSpec().Value.SetUncompressedData([]byte("machine: {}")))

	suite.Require().NoError(suite.state.Create(ctx, siderolink.NewLink("reported", nil)))
	suite.Require().NoError(suite.state.Create(ctx, otherRequest))
	suite.Require().NoError(suite.state.Create(ctx, request))
	suite.Require().NoError(suite.state.Create(ctx, status))

	// the machine which is bound to the machine request
	link := siderolink.NewLink("1234", nil)
	link.Metadata().Labels().Set(omni.LabelMachineRequest, requestID)

	suite.Require().NoError(suite.state.Create(ctx, link))

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, id)

	// the status reports the bound machine
	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, status.Metadata(), func(res *infra.MachineRequestStatus) error {
		res.TypedSpec().Value.Id = link.Metadata().ID()

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{id}, func(configPatch *omni.ConfigPatch, assert *assert.Assertions) {
		assert.Equal(configPatch.TypedSpec().Value.GetCompressedData(), request.TypedSpec().Value.GetCompressedData())

		value, ok := configPatch.Metadata().Labels().Get(omni.LabelMachine)
		assert.True(ok)
		assert.Equal(link.Metadata().ID(), value)

		value, ok = configPatch.Metadata().Labels().Get(omni.LabelMachineRequest)
		assert.True(ok)
		assert.Equal(requestID, value)

		value, ok = configPatch.Metadata().Labels().Get(omni.LabelInfraProviderID)
		assert.True(ok)
		assert.Equal(provider, value)
	})

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, otherRequest.Metadata().ID())

	rtestutils.DestroyAll[*infra.ConfigPatchRequest](suite.ctx, suite.T(), suite.state)

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, id)
}

func TestInfraProviderConfigPatchControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(InfraProviderConfigPatchControllerSuite))
}
