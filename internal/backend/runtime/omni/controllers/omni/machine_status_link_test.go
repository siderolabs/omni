// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	siderolinkmanager "github.com/siderolabs/omni/internal/pkg/siderolink"
)

type MachineStatusLinkSuite struct {
	OmniSuite
	deltaCh chan siderolinkmanager.LinkCounterDeltas
}

const testID2 = "testID2"

func (suite *MachineStatusLinkSuite) SetupTest() {
	suite.OmniSuite.SetupTest()

	suite.startRuntime()

	suite.deltaCh = make(chan siderolinkmanager.LinkCounterDeltas)

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineStatusController(
		&imageFactoryClientMock{},
	)))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineStatusLinkController(suite.deltaCh)))
}

func (suite *MachineStatusLinkSuite) TestBasicMachineOnAndOff() {
	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	machine := omni.NewMachine(resources.DefaultNamespace, testID)
	machine.TypedSpec().Value.Connected = true

	suite.create(machine)

	assertResource(
		&suite.OmniSuite,
		makeMD[*omni.MachineStatusLink](testID),
		func(status *omni.MachineStatusLink, asrt *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			asrt.True(statusVal.GetMessageStatus().GetConnected())
		},
	)

	suite.deltaCh <- siderolinkmanager.LinkCounterDeltas{
		testID: siderolinkmanager.LinkCounterDelta{
			BytesSent:     15,
			BytesReceived: 20,
			LastAlive:     time.Unix(1257894000, 0),
		},
	}

	assertResource(
		&suite.OmniSuite,
		makeMD[*omni.MachineStatusLink](testID),
		func(status *omni.MachineStatusLink, asrt *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			asrt.True(statusVal.GetMessageStatus().GetConnected())
			asrt.EqualValues(15, statusVal.GetSiderolinkCounter().GetBytesSent())
			asrt.EqualValues(20, statusVal.GetSiderolinkCounter().GetBytesReceived())
			asrt.EqualValues(1257894000, statusVal.GetSiderolinkCounter().GetLastAlive().AsTime().Unix())
		},
	)

	rtestutils.Destroy[*omni.Machine](suite.ctx, suite.T(), suite.state, []string{machine.Metadata().ID()})

	msl := omni.NewMachineStatusLink(resources.MetricsNamespace, testID)

	assertNoResource(&suite.OmniSuite, msl)
}

func (suite *MachineStatusLinkSuite) TestTwoMachines() {
	machine1 := omni.NewMachine(resources.DefaultNamespace, testID)
	machine1.TypedSpec().Value.Connected = true

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)))
	suite.Require().NoError(suite.state.Create(suite.ctx, siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)))

	suite.create(machine1)

	assertResource(
		&suite.OmniSuite,
		makeMD[*omni.MachineStatusLink](testID),
		func(status *omni.MachineStatusLink, asrt *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			asrt.True(statusVal.GetMessageStatus().GetConnected())
		},
	)

	suite.deltaCh <- siderolinkmanager.LinkCounterDeltas{
		testID: siderolinkmanager.LinkCounterDelta{
			BytesSent:     15,
			BytesReceived: 20,
			LastAlive:     time.Unix(1257894000, 0),
		},
	}

	assertResource(
		&suite.OmniSuite,
		makeMD[*omni.MachineStatusLink](testID),
		func(status *omni.MachineStatusLink, asrt *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			asrt.True(statusVal.GetMessageStatus().GetConnected())
			asrt.EqualValues(15, statusVal.GetSiderolinkCounter().GetBytesSent())
			asrt.EqualValues(20, statusVal.GetSiderolinkCounter().GetBytesReceived())
			asrt.EqualValues(1257894000, statusVal.GetSiderolinkCounter().GetLastAlive().AsTime().Unix())
		},
	)

	machine2 := omni.NewMachine(resources.DefaultNamespace, testID2)
	machine2.TypedSpec().Value.Connected = true

	suite.create(machine2)

	rtestutils.Destroy[*omni.Machine](suite.ctx, suite.T(), suite.state, []string{machine1.Metadata().ID()})

	assertNoResource(&suite.OmniSuite, omni.NewMachineStatusLink(resources.MetricsNamespace, testID))

	assertResource(
		&suite.OmniSuite,
		makeMD[*omni.MachineStatusLink](testID2),
		func(status *omni.MachineStatusLink, asrt *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			asrt.True(statusVal.GetMessageStatus().GetConnected())
		},
	)

	suite.deltaCh <- siderolinkmanager.LinkCounterDeltas{
		testID2: siderolinkmanager.LinkCounterDelta{
			BytesSent:     16,
			BytesReceived: 21,
			LastAlive:     time.Unix(1257894001, 0),
		},
	}

	assertResource(
		&suite.OmniSuite,
		makeMD[*omni.MachineStatusLink](testID2),
		func(status *omni.MachineStatusLink, asrt *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			asrt.True(statusVal.GetMessageStatus().GetConnected())
			asrt.EqualValues(16, statusVal.GetSiderolinkCounter().GetBytesSent())
			asrt.EqualValues(21, statusVal.GetSiderolinkCounter().GetBytesReceived())
			asrt.EqualValues(1257894001, statusVal.GetSiderolinkCounter().GetLastAlive().AsTime().Unix())
		},
	)

	suite.deltaCh <- siderolinkmanager.LinkCounterDeltas{
		testID2: siderolinkmanager.LinkCounterDelta{
			BytesSent:     1,
			BytesReceived: 1,
			LastAlive:     time.Unix(1257894002, 0),
		},
	}

	assertResource(
		&suite.OmniSuite,
		makeMD[*omni.MachineStatusLink](testID2),
		func(status *omni.MachineStatusLink, asrt *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			asrt.True(statusVal.GetMessageStatus().GetConnected())
			asrt.EqualValues(17, statusVal.GetSiderolinkCounter().GetBytesSent())
			asrt.EqualValues(22, statusVal.GetSiderolinkCounter().GetBytesReceived())
			asrt.EqualValues(1257894002, statusVal.GetSiderolinkCounter().GetLastAlive().AsTime().Unix())
		},
	)
}

func (suite *MachineStatusLinkSuite) create(res resource.Resource) {
	suite.Require().NoError(suite.state.Create(suite.ctx, res))
}

func makeMD[T generic.ResourceWithRD](id resource.ID) resource.Metadata {
	var zero T

	return resource.NewMetadata(
		zero.ResourceDefinition().DefaultNamespace,
		zero.ResourceDefinition().Type,
		id,
		resource.VersionUndefined,
	)
}

func TestMachineStatusLinkSuiteSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineStatusLinkSuite))
}
