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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-retry/retry"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterMachineStatusSuite struct {
	OmniSuite
}

func (suite *ClusterMachineStatusSuite) setup() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineStatusController()))
}

func (suite *ClusterMachineStatusSuite) TearDownTest() {
	rtestutils.DestroyAll[*omni.ClusterMachine](suite.ctx, suite.T(), suite.state)
	rtestutils.DestroyAll[*omni.Machine](suite.ctx, suite.T(), suite.state)
	rtestutils.DestroyAll[*omni.MachineStatus](suite.ctx, suite.T(), suite.state)
	rtestutils.DestroyAll[*omni.MachineSetNode](suite.ctx, suite.T(), suite.state)
	rtestutils.DestroyAll[*omni.ClusterMachineConfigStatus](suite.ctx, suite.T(), suite.state)
	rtestutils.DestroyAll[*omni.MachineStatusSnapshot](suite.ctx, suite.T(), suite.state)

	suite.OmniSuite.TearDownTest()
}

func (suite *ClusterMachineStatusSuite) TestNoMachineStatusSnapShotClusterStatusZeroValue() {
	suite.setup()

	// given
	testID := "testID"
	machine := omni.NewMachine(resources.DefaultNamespace, testID)
	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, testID)
	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, testID)

	// when
	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatus))
	suite.Assert().NoError(suite.state.Create(suite.ctx, clusterMachine))

	// then
	suite.assertStage(specs.ClusterMachineStatusSpec_UNKNOWN, false, false)
}

func (suite *ClusterMachineStatusSuite) TestApplyConfigErrorPropagation() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}, true, false)

	md := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, testID).Metadata()
	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, md, func(s *omni.ClusterMachineConfigStatus) error {
		s.TypedSpec().Value.LastConfigError = "TestApplyConfigErrorPropagation error"

		return nil
	})
	suite.Require().NoError(err)

	clusterMachineStatus := omni.NewClusterMachineStatus(resources.DefaultNamespace, testID)

	err = retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(suite.ctx, func(ctx context.Context) error {
		res, resErr := safe.StateGet[*omni.ClusterMachineStatus](ctx, suite.state, clusterMachineStatus.Metadata())
		if resErr != nil {
			return retry.ExpectedError(resErr)
		}

		if res.TypedSpec().Value.LastConfigError != "TestApplyConfigErrorPropagation error" {
			return retry.ExpectedErrorf("error not set")
		}

		if res.TypedSpec().Value.ConfigUpToDate {
			return retry.ExpectedErrorf("config is up to date")
		}

		if res.TypedSpec().Value.ConfigApplyStatus != specs.ConfigApplyStatus_FAILED {
			return retry.ExpectedErrorf("config apply status is not failed")
		}

		return nil
	})
	suite.Assert().NoError(err)
}

func (suite *ClusterMachineStatusSuite) TestOutdatedConfig() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}, true, false)

	md := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, testID).Metadata()
	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, md, func(s *omni.ClusterMachineConfigStatus) error {
		s.TypedSpec().Value.ClusterMachineConfigVersion = "42"

		return nil
	})
	suite.Require().NoError(err)

	clusterMachineStatus := omni.NewClusterMachineStatus(resources.DefaultNamespace, testID)

	err = retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(suite.ctx, func(ctx context.Context) error {
		res, resErr := safe.StateGet[*omni.ClusterMachineStatus](ctx, suite.state, clusterMachineStatus.Metadata())
		if resErr != nil {
			return retry.ExpectedError(resErr)
		}

		if res.TypedSpec().Value.ConfigUpToDate {
			return retry.ExpectedErrorf("config is up to date")
		}

		if res.TypedSpec().Value.ConfigApplyStatus != specs.ConfigApplyStatus_PENDING {
			return retry.ExpectedErrorf("config apply status is not pending")
		}

		return nil
	})
	suite.Assert().NoError(err)
}

func (suite *ClusterMachineStatusSuite) setupStageTest(machineStatusEvent *machineapi.MachineStatusEvent, connected, isControlPlaneNode bool) {
	suite.setup()

	testID := "testID"

	machine := omni.NewMachine(resources.DefaultNamespace, testID)
	machine.TypedSpec().Value.Connected = connected
	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, testID)

	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, testID)

	statusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, testID)
	statusSnapshot.TypedSpec().Value.MachineStatus = machineStatusEvent

	clusterMachineConfigStatus := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, testID)

	if isControlPlaneNode {
		clusterMachine.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
	}

	machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, testID, omni.NewMachineSet(resources.DefaultNamespace, "ms"))

	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatus))
	suite.Assert().NoError(suite.state.Create(suite.ctx, clusterMachine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineSetNode))
	suite.Assert().NoError(suite.state.Create(suite.ctx, statusSnapshot))
	suite.Assert().NoError(suite.state.Create(suite.ctx, clusterMachineConfigStatus))
}

func (suite *ClusterMachineStatusSuite) assertStage(expectedStage specs.ClusterMachineStatusSpec_Stage, expectedReadiness, expectedApidAvailable bool) {
	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineStatus(resources.DefaultNamespace, "testID").Metadata(),
		func(status *omni.ClusterMachineStatus, assertions *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			assertions.Equal(expectedReadiness, statusVal.Ready)
			assertions.Equal(expectedStage, statusVal.Stage)
			assertions.Equal(expectedApidAvailable, statusVal.ApidAvailable)
		})
}

func (suite *ClusterMachineStatusSuite) TestDestroying() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RESETTING}, true, false)

	suite.assertStage(specs.ClusterMachineStatusSpec_DESTROYING, false, false)
}

func (suite *ClusterMachineStatusSuite) TestInstalling() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_INSTALLING}, true, false)

	suite.assertStage(specs.ClusterMachineStatusSpec_INSTALLING, false, false)
}

func (suite *ClusterMachineStatusSuite) TestBoot() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_BOOTING}, true, false)

	suite.assertStage(specs.ClusterMachineStatusSpec_BOOTING, false, false)
}

func (suite *ClusterMachineStatusSuite) TestReboot() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_REBOOTING}, true, false)

	suite.assertStage(specs.ClusterMachineStatusSpec_REBOOTING, false, false)
}

func (suite *ClusterMachineStatusSuite) TestRunning() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{
		Stage:  machineapi.MachineStatusEvent_RUNNING,
		Status: &machineapi.MachineStatusEvent_MachineStatus{Ready: true},
	}, true, false)

	suite.assertStage(specs.ClusterMachineStatusSpec_RUNNING, true, false)
}

func (suite *ClusterMachineStatusSuite) TestRunningNotConnected() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{
		Stage:  machineapi.MachineStatusEvent_RUNNING,
		Status: &machineapi.MachineStatusEvent_MachineStatus{Ready: true},
	}, false, false)

	suite.assertStage(specs.ClusterMachineStatusSpec_RUNNING, false, false)
}

func (suite *ClusterMachineStatusSuite) TestRunningNotReady() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}, true, false)

	suite.assertStage(specs.ClusterMachineStatusSpec_RUNNING, false, false)
}

func (suite *ClusterMachineStatusSuite) TestApidAvailable() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{
		Stage:  machineapi.MachineStatusEvent_RUNNING,
		Status: &machineapi.MachineStatusEvent_MachineStatus{Ready: true},
	}, true, true)

	suite.assertStage(specs.ClusterMachineStatusSpec_RUNNING, true, true)
}

func TestClusterMachineStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterMachineStatusSuite))
}
