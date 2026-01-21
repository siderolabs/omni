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
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineStatusSnapshotControllerSuite struct {
	OmniSuite
}

func (suite *MachineStatusSnapshotControllerSuite) TestReconcile() {
	require := suite.Require()

	suite.startRuntime()

	// wait for the runtime to stop, including all controllers and the tasks they started.
	// this is necessary to prevent a data race on the test logger when a task finishes after the test ends.
	suite.T().Cleanup(suite.wg.Wait)

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	suite.T().Cleanup(cancel)

	siderolinkEventsCh := make(chan *omni.MachineStatusSnapshot)

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineStatusSnapshotController(siderolinkEventsCh, nil)))

	m := omni.NewMachine("1")
	m.TypedSpec().Value.Connected = true
	m.TypedSpec().Value.ManagementAddress = suite.socketConnectionString

	require.NoError(suite.state.Create(suite.ctx, m))

	// make sure that the machine became visible in the cached state
	rtestutils.AssertResource(ctx, suite.T(), suite.cachedState, m.Metadata().ID(), func(r *omni.Machine, assertion *assert.Assertions) {})

	snapshot := omni.NewMachineStatusSnapshot(m.Metadata().ID())

	snapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
		Stage: machine.MachineStatusEvent_BOOTING,
		Status: &machine.MachineStatusEvent_MachineStatus{
			Ready: false,
			UnmetConditions: []*machine.MachineStatusEvent_MachineStatus_UnmetCondition{
				{
					Name:   "name",
					Reason: "nope",
				},
			},
		},
	}

	// handle siderolink
	suite.Require().True(channel.SendWithContext(ctx, siderolinkEventsCh, snapshot))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{m.Metadata().ID()}, func(r *omni.MachineStatusSnapshot, assertion *assert.Assertions) {
		assertion.EqualValues(snapshot.TypedSpec().Value, r.TypedSpec().Value)
	})

	snapshot = omni.NewMachineStatusSnapshot("not exists")

	// ignore events for machines that do not exist
	suite.Require().True(channel.SendWithContext(ctx, siderolinkEventsCh, snapshot))

	rtestutils.AssertNoResource[*omni.MachineStatusSnapshot](ctx, suite.T(), suite.state, snapshot.Metadata().ID())

	ms := runtime.NewMachineStatus()
	ms.TypedSpec().Stage = runtime.MachineStageInstalling
	ms.TypedSpec().Status = runtime.MachineStatusStatus{
		Ready: false,
		UnmetConditions: []runtime.UnmetCondition{
			{
				Name:   "you",
				Reason: "failed",
			},
		},
	}

	suite.Require().NoError(suite.machineService.state.Create(ctx, ms))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{m.Metadata().ID()}, func(r *omni.MachineStatusSnapshot, assertion *assert.Assertions) {
		assertion.EqualValues(machine.MachineStatusEvent_INSTALLING, r.TypedSpec().Value.MachineStatus.Stage)
		assertion.EqualValues(false, r.TypedSpec().Value.MachineStatus.Status.Ready)
		assertion.EqualValues("you", r.TypedSpec().Value.MachineStatus.Status.UnmetConditions[0].Name)
		assertion.EqualValues("failed", r.TypedSpec().Value.MachineStatus.Status.UnmetConditions[0].Reason)
	})

	clusterName := "cluster"

	clusterMachine := omni.NewClusterMachine("2")
	clusterMachine.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	suite.Require().NoError(suite.state.Create(ctx, clusterMachine))

	talosConfig := omni.NewTalosConfig(clusterName)

	suite.Require().NoError(suite.state.Create(ctx, talosConfig))

	m = omni.NewMachine(clusterMachine.Metadata().ID())
	m.TypedSpec().Value.Connected = true
	m.TypedSpec().Value.ManagementAddress = suite.socketConnectionString

	suite.Require().NoError(suite.state.Create(ctx, m))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{m.Metadata().ID()}, func(r *omni.MachineStatusSnapshot, assertion *assert.Assertions) {
		assertion.EqualValues(machine.MachineStatusEvent_INSTALLING, r.TypedSpec().Value.MachineStatus.Stage)
		assertion.EqualValues(false, r.TypedSpec().Value.MachineStatus.Status.Ready)
		assertion.EqualValues("you", r.TypedSpec().Value.MachineStatus.Status.UnmetConditions[0].Name)
		assertion.EqualValues("failed", r.TypedSpec().Value.MachineStatus.Status.UnmetConditions[0].Reason)
	})

	rtestutils.DestroyAll[*omni.Machine](ctx, suite.T(), suite.state)

	rtestutils.AssertNoResource[*omni.MachineStatusSnapshot](ctx, suite.T(), suite.state, m.Metadata().ID())
}

func TestMachineStatusSnapshotControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineStatusSnapshotControllerSuite))
}
