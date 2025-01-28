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

	suite.Require().NoError(request.TypedSpec().Value.SetUncompressedData([]byte("machine: {}")))

	status.TypedSpec().Value.Id = "1234"

	suite.Require().NoError(suite.state.Create(ctx, request))
	suite.Require().NoError(suite.state.Create(ctx, status))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{id}, func(configPatch *omni.ConfigPatch, assert *assert.Assertions) {
		assert.Equal(configPatch.TypedSpec().Value.CompressedData, request.TypedSpec().Value.CompressedData) //nolint:staticcheck

		value, ok := configPatch.Metadata().Labels().Get(omni.LabelMachine)
		assert.True(ok)
		assert.Equal(status.TypedSpec().Value.Id, value)

		value, ok = configPatch.Metadata().Labels().Get(omni.LabelMachineRequest)
		assert.True(ok)
		assert.Equal(requestID, value)

		value, ok = configPatch.Metadata().Labels().Get(omni.LabelInfraProviderID)
		assert.True(ok)
		assert.Equal(provider, value)
	})

	rtestutils.DestroyAll[*infra.ConfigPatchRequest](suite.ctx, suite.T(), suite.state)

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, id)
}

func TestInfraProviderConfigPatchControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(InfraProviderConfigPatchControllerSuite))
}
