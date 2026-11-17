// Copyright (c) 2026 Sidero Labs, Inc.
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

	// a machine which joined manually, one which joined through another infra provider, and one which is bound to another machine request
	manualUUID := "manual"
	otherUUID := "other"
	boundUUID := "bound"

	boundLink := siderolink.NewLink(boundUUID, nil)
	boundLink.Metadata().Labels().Set(omni.LabelInfraProviderID, "provider")
	boundLink.Metadata().Labels().Set(omni.LabelMachineRequest, "another-request")

	suite.Require().NoError(suite.state.Create(ctx, boundLink))

	otherLink := siderolink.NewLink(otherUUID, nil)
	otherLink.Metadata().Labels().Set(omni.LabelInfraProviderID, "other-provider")

	suite.Require().NoError(suite.state.Create(ctx, siderolink.NewLink(manualUUID, nil)))
	suite.Require().NoError(suite.state.Create(ctx, otherLink))

	for id, machineUUID := range map[string]string{"request-manual": manualUUID, "request-other": otherUUID, "request-bound": boundUUID} {
		status := infra.NewMachineRequestStatus(id)
		status.Metadata().Labels().Set(omni.LabelInfraProviderID, "provider")
		status.TypedSpec().Value.Id = machineUUID

		suite.Require().NoError(suite.state.Create(ctx, status))
	}

	// the machine request set is taken from the machine request, and not from the status, which the infra provider writes
	machineRequest := infra.NewMachineRequest(requestID)
	machineRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, "provider")
	machineRequest.Metadata().Labels().Set(omni.LabelMachineRequestSet, "set")

	status := infra.NewMachineRequestStatus(requestID)
	status.Metadata().Labels().Set(omni.LabelInfraProviderID, "provider")
	status.Metadata().Labels().Set(omni.LabelMachineRequestSet, "another-set")
	status.TypedSpec().Value.Id = uuid

	link := siderolink.NewLink(uuid, nil)
	link.Metadata().Labels().Set(omni.LabelInfraProviderID, "provider")

	suite.Require().NoError(suite.state.Create(ctx, machineRequest))
	suite.Require().NoError(suite.state.Create(ctx, status))
	suite.Require().NoError(suite.state.Create(ctx, link))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{uuid}, func(link *siderolink.Link, assert *assert.Assertions) {
		request, ok := link.Metadata().Labels().Get(omni.LabelMachineRequest)

		assert.True(ok)
		assert.Equal(requestID, request)

		machineRequestSet, _ := link.Metadata().Labels().Get(omni.LabelMachineRequestSet)
		assert.Equal("set", machineRequestSet)
	})

	// the machines which did not join through the infra provider of the status are not bound to its machine request
	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{manualUUID, otherUUID}, func(link *siderolink.Link, assert *assert.Assertions) {
		_, ok := link.Metadata().Labels().Get(omni.LabelMachineRequest)

		assert.False(ok)
	})

	// a machine which is bound already keeps its machine request
	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{boundUUID}, func(link *siderolink.Link, assert *assert.Assertions) {
		request, _ := link.Metadata().Labels().Get(omni.LabelMachineRequest)

		assert.Equal("another-request", request)
	})
}

// TestCleanup checks that removing a machine request status removes the machine bound to it, and no other machine.
func (suite *MachineRequestLinkSuite) TestCleanup() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineRequestLinkController(suite.state)))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineRequestStatusCleanupController()))

	createMachine := func(uuid, provider string) {
		link := siderolink.NewLink(uuid, nil)

		if provider != "" {
			link.Metadata().Labels().Set(omni.LabelInfraProviderID, provider)
		}

		machineSetNode := omni.NewMachineSetNode(uuid, omni.NewMachineSet("workers"))

		suite.Require().NoError(suite.state.Create(ctx, link))
		suite.Require().NoError(suite.state.Create(ctx, machineSetNode))
	}

	createStatus := func(id, uuid string) *infra.MachineRequestStatus {
		status := infra.NewMachineRequestStatus(id)
		status.Metadata().Labels().Set(omni.LabelInfraProviderID, "provider")
		status.TypedSpec().Value.Id = uuid

		suite.Require().NoError(suite.state.Create(ctx, status))

		return status
	}

	createMachine("own", "provider")
	createMachine("manual", "")

	own := createStatus("request-own", "own")
	manual := createStatus("request-manual", "manual") // points at a machine which did not join through the provider

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{"own"}, func(link *siderolink.Link, assert *assert.Assertions) {
		_, ok := link.Metadata().Labels().Get(omni.LabelMachineRequest)

		assert.True(ok)
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{own.Metadata().ID(), manual.Metadata().ID()},
		func(status *infra.MachineRequestStatus, assert *assert.Assertions) {
			assert.False(status.Metadata().Finalizers().Empty())
		})

	rtestutils.Destroy[*infra.MachineRequestStatus](ctx, suite.T(), suite.state, []string{manual.Metadata().ID()})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{"manual"}, func(machineSetNode *omni.MachineSetNode, assert *assert.Assertions) {
		assert.Equal(resource.PhaseRunning, machineSetNode.Metadata().Phase())
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{"manual"}, func(link *siderolink.Link, assert *assert.Assertions) {
		assert.Equal(resource.PhaseRunning, link.Metadata().Phase())
	})

	_, err := suite.state.Teardown(ctx, own.Metadata())
	suite.Require().NoError(err)

	rtestutils.AssertNoResource[*omni.MachineSetNode](ctx, suite.T(), suite.state, "own")

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{"own"}, func(link *siderolink.Link, assert *assert.Assertions) {
		assert.Equal(resource.PhaseTearingDown, link.Metadata().Phase())
	})
}

func TestMachineRequestLinkSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineRequestLinkSuite))
}
