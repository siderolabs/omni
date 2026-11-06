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
	"github.com/siderolabs/go-retry/retry"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
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
	rtestutils.DestroyAll[*omni.MachinePendingUpdates](suite.ctx, suite.T(), suite.state)

	suite.OmniSuite.TearDownTest()
}

func (suite *ClusterMachineStatusSuite) TestNoMachineStatusSnapShotClusterStatusZeroValue() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}, true)

	// when
	rtestutils.Destroy[*omni.MachineStatus](suite.ctx, suite.T(), suite.state, []string{testID})

	// then
	suite.assertStage(specs.ClusterMachineStatusSpec_UNKNOWN, false)
}

func (suite *ClusterMachineStatusSuite) TestApplyConfigErrorPropagation() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}, true)

	rmock.Mock[*omni.ClusterMachineConfigStatus](
		suite.ctx, suite.T(), suite.state, options.WithID(testID),
		options.Modify(func(s *omni.ClusterMachineConfigStatus) error {
			s.TypedSpec().Value.LastConfigError = "TestApplyConfigErrorPropagation error"

			return nil
		}),
	)

	clusterMachineStatus := omni.NewClusterMachineStatus(testID)

	err := retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(suite.ctx, func(ctx context.Context) error {
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
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}, true)

	rmock.Mock[*omni.MachinePendingUpdates](
		suite.ctx, suite.T(), suite.state, options.WithID(testID),
		options.Modify(func(res *omni.MachinePendingUpdates) error {
			return res.TypedSpec().Value.SetUncompressedData([]byte("config diff"))
		}),
	)

	clusterMachineStatus := omni.NewClusterMachineStatus(testID)

	err := retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(suite.ctx, func(ctx context.Context) error {
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

func (suite *ClusterMachineStatusSuite) TestPendingUpgradeOnly() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}, true)

	rmock.Mock[*omni.MachinePendingUpdates](
		suite.ctx, suite.T(), suite.state, options.WithID(testID),
		options.Modify(func(res *omni.MachinePendingUpdates) error {
			res.TypedSpec().Value.Upgrade = &specs.MachinePendingUpdatesSpec_Upgrade{
				FromVersion: "1.13.0",
				ToVersion:   "1.14.0",
			}

			return nil
		}),
	)

	rtestutils.AssertResource(suite.ctx, suite.T(), suite.state, testID, func(res *omni.ClusterMachineStatus, assertion *assert.Assertions) {
		assertion.True(res.TypedSpec().Value.ConfigUpToDate)
		assertion.Equal(specs.ConfigApplyStatus_APPLIED, res.TypedSpec().Value.ConfigApplyStatus)
	})
}

func (suite *ClusterMachineStatusSuite) setupStageTest(machineStatusEvent *machineapi.MachineStatusEvent, connected bool) {
	suite.setup()

	testID := "testID"

	cluster := rmock.Mock[*omni.Cluster](suite.ctx, suite.T(), suite.state)
	rmock.Mock[*omni.ClusterSecrets](suite.ctx, suite.T(), suite.state, options.WithID(cluster.Metadata().ID()))

	machine := omni.NewMachine(testID)
	machine.TypedSpec().Value.Connected = connected
	machineStatus := omni.NewMachineStatus(testID)

	rmock.Mock[*omni.MachineSetNode](
		suite.ctx, suite.T(), suite.state,
		options.WithID(testID),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelWorkerRole),
	)

	rmock.Mock[*omni.ClusterMachine](
		suite.ctx, suite.T(), suite.state,
		options.WithID(testID),
	)

	statusSnapshot := omni.NewMachineStatusSnapshot(testID)
	statusSnapshot.TypedSpec().Value.MachineStatus = machineStatusEvent

	rmock.Mock[*omni.MachineConfigGenOptions](
		suite.ctx, suite.T(), suite.state,
		options.WithID(testID),
	)

	rmock.Mock[*omni.ClusterMachineConfig](
		suite.ctx, suite.T(), suite.state,
		options.WithID(testID),
	)

	rmock.Mock[*omni.ClusterMachineConfigStatus](
		suite.ctx, suite.T(), suite.state,
		options.WithID(testID),
	)

	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatus))
	suite.Assert().NoError(suite.state.Create(suite.ctx, statusSnapshot))
}

func (suite *ClusterMachineStatusSuite) assertStage(expectedStage specs.ClusterMachineStatusSpec_Stage, expectedReadiness bool) {
	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterMachineStatus("testID").Metadata(),
		func(status *omni.ClusterMachineStatus, assertions *assert.Assertions) {
			statusVal := status.TypedSpec().Value

			assertions.Equal(expectedReadiness, statusVal.Ready)
			assertions.Equal(expectedStage, statusVal.Stage)
		},
	)
}

func (suite *ClusterMachineStatusSuite) TestDestroying() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RESETTING}, true)

	suite.assertStage(specs.ClusterMachineStatusSpec_DESTROYING, false)
}

func (suite *ClusterMachineStatusSuite) TestInstalling() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_INSTALLING}, true)

	suite.assertStage(specs.ClusterMachineStatusSpec_INSTALLING, false)
}

func (suite *ClusterMachineStatusSuite) TestBoot() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_BOOTING}, true)

	suite.assertStage(specs.ClusterMachineStatusSpec_BOOTING, false)
}

func (suite *ClusterMachineStatusSuite) TestReboot() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_REBOOTING}, true)

	suite.assertStage(specs.ClusterMachineStatusSpec_REBOOTING, false)
}

func (suite *ClusterMachineStatusSuite) TestRunning() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{
		Stage:  machineapi.MachineStatusEvent_RUNNING,
		Status: &machineapi.MachineStatusEvent_MachineStatus{Ready: true},
	}, true)

	suite.assertStage(specs.ClusterMachineStatusSpec_RUNNING, true)
}

func (suite *ClusterMachineStatusSuite) TestRunningNotConnected() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{
		Stage:  machineapi.MachineStatusEvent_RUNNING,
		Status: &machineapi.MachineStatusEvent_MachineStatus{Ready: true},
	}, false)

	suite.assertStage(specs.ClusterMachineStatusSpec_RUNNING, false)
}

func (suite *ClusterMachineStatusSuite) TestRunningNotReady() {
	suite.setupStageTest(&machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}, true)

	suite.assertStage(specs.ClusterMachineStatusSpec_RUNNING, false)
}

func TestClusterMachineStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterMachineStatusSuite))
}

//nolint:maintidx
func TestClusterMachineStatusOperation(t *testing.T) {
	t.Parallel()

	const machineID = "machine-1"

	startedAt := timestamppb.New(time.Unix(1000, 0))

	for _, tc := range []struct {
		operationStatus        *specs.MachineOperationStatusSpec
		configStatus           func(*specs.ClusterMachineConfigStatusSpec)
		resolve                func(*specs.ClusterMachineConfigStatusSpec)
		expected               *specs.ClusterMachineStatusSpec_Operation
		failureShown           *specs.ClusterMachineStatusSpec_Operation
		name                   string
		powerStage             specs.MachineStatusSnapshotSpec_PowerStage
		configOutdated         bool
		unknownBootID          bool
		noClusterMachineConfig bool
	}{
		{
			name: "no operation",
		},
		{
			name: "running operation",
			operationStatus: &specs.MachineOperationStatusSpec{
				Phase:          specs.MachineOperationPhase_DRAINING,
				Message:        "draining node node-1",
				PhaseStartedAt: startedAt,
			},
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-1"
			},
			expected: &specs.ClusterMachineStatusSpec_Operation{
				Phase:          specs.MachineOperationPhase_DRAINING,
				Message:        "draining node node-1",
				PhaseStartedAt: startedAt,
			},
		},
		{
			name: "rebooting",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-1"
			},
			expected: &specs.ClusterMachineStatusSpec_Operation{
				Phase: specs.MachineOperationPhase_REBOOTING,
			},
		},
		{
			name: "rebooting with an unknown boot ID",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-0"
			},
			unknownBootID: true,
			expected: &specs.ClusterMachineStatusSpec_Operation{
				Phase: specs.MachineOperationPhase_REBOOTING,
			},
		},
		{
			name: "finalizing",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-0"
			},
			expected: &specs.ClusterMachineStatusSpec_Operation{
				Phase: specs.MachineOperationPhase_FINALIZING,
			},
		},
		{
			name: "uncordoning",
			operationStatus: &specs.MachineOperationStatusSpec{
				Phase:          specs.MachineOperationPhase_UNCORDONING,
				Message:        "uncordoning node node-1",
				PhaseStartedAt: startedAt,
			},
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-0"
			},
			expected: &specs.ClusterMachineStatusSpec_Operation{
				Phase:          specs.MachineOperationPhase_UNCORDONING,
				Message:        "uncordoning node node-1",
				PhaseStartedAt: startedAt,
			},
		},
		{
			name: "failed uncordoning",
			operationStatus: &specs.MachineOperationStatusSpec{
				LastError:   "failed to uncordon node: node unreachable",
				FailedPhase: specs.MachineOperationPhase_UNCORDONING,
			},
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-0"
			},
			expected: &specs.ClusterMachineStatusSpec_Operation{
				Phase:       specs.MachineOperationPhase_FINALIZING,
				LastError:   "failed to uncordon node: node unreachable",
				FailedPhase: specs.MachineOperationPhase_UNCORDONING,
			},
		},
		{
			name: "resolved failed uncordoning",
			operationStatus: &specs.MachineOperationStatusSpec{
				LastError:   "failed to uncordon node: node unreachable",
				FailedPhase: specs.MachineOperationPhase_UNCORDONING,
			},
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-0"
			},
			failureShown: &specs.ClusterMachineStatusSpec_Operation{
				Phase:       specs.MachineOperationPhase_FINALIZING,
				LastError:   "failed to uncordon node: node unreachable",
				FailedPhase: specs.MachineOperationPhase_UNCORDONING,
			},
			resolve: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = ""
			},
		},
		{
			name: "trying config",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.ConfigTry = &specs.ConfigTryStatus{
					Attempts:  2,
					StartedAt: startedAt,
				}
			},
			expected: &specs.ClusterMachineStatusSpec_Operation{
				Phase:            specs.MachineOperationPhase_TRYING_CONFIG,
				PhaseStartedAt:   startedAt,
				ConfigTryAttempt: 2,
			},
		},
		{
			name: "stopped trying config",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.ConfigTry = &specs.ConfigTryStatus{
					Attempts:  2,
					StartedAt: startedAt,
				}
				spec.LastConfigError = "the config was rolled back 2 times, change it to try again"
			},
			configOutdated: true,
		},
		{
			name: "powered off",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-0"
			},
			powerStage: specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF,
		},
		{
			name: "failed operation",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.LastConfigError = "upgrade failed: failed to pull installer image: timeout"
				spec.LastConfigErrorPhase = specs.MachineOperationPhase_PULLING_IMAGE
			},
			configOutdated: true,
			expected: &specs.ClusterMachineStatusSpec_Operation{
				FailedPhase: specs.MachineOperationPhase_PULLING_IMAGE,
			},
		},
		{
			name: "config error after a failed operation",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.LastConfigError = "upgrade failed: context deadline exceeded"
				spec.LastConfigErrorPhase = specs.MachineOperationPhase_DRAINING
			},
			failureShown: &specs.ClusterMachineStatusSpec_Operation{
				FailedPhase: specs.MachineOperationPhase_DRAINING,
			},
			resolve: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.LastConfigError = "failed to apply config to machine: context deadline exceeded"
				spec.LastConfigErrorPhase = specs.MachineOperationPhase_NONE
			},
			configOutdated: true,
		},
		{
			name: "failed operation without a cluster machine config",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.LastConfigError = "upgrade failed: failed to pull installer image: timeout"
				spec.LastConfigErrorPhase = specs.MachineOperationPhase_PULLING_IMAGE
			},
			noClusterMachineConfig: true,
			configOutdated:         true,
		},
		{
			name: "config error while finalizing",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.PreRebootBootId = "boot-0"
				spec.LastConfigError = "failed to generate config: invalid patch"
			},
			configOutdated: true,
			expected: &specs.ClusterMachineStatusSpec_Operation{
				Phase: specs.MachineOperationPhase_FINALIZING,
			},
		},
		{
			name: "resolved failed operation",
			configStatus: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.LastConfigError = "upgrade failed: failed to pull installer image: timeout"
				spec.LastConfigErrorPhase = specs.MachineOperationPhase_PULLING_IMAGE
			},
			failureShown: &specs.ClusterMachineStatusSpec_Operation{
				FailedPhase: specs.MachineOperationPhase_PULLING_IMAGE,
			},
			resolve: func(spec *specs.ClusterMachineConfigStatusSpec) {
				spec.LastConfigError = ""
				spec.LastConfigErrorPhase = specs.MachineOperationPhase_NONE
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(ctx, t, testutils.TestOptions{},
				func(_ context.Context, testContext testutils.TestContext) {
					require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineStatusController()))
				},
				func(ctx context.Context, testContext testutils.TestContext) {
					st := testContext.State

					cluster := rmock.Mock[*omni.Cluster](ctx, t, st)
					rmock.Mock[*omni.ClusterSecrets](ctx, t, st, options.WithID(cluster.Metadata().ID()))

					rmock.Mock[*omni.MachineSetNode](ctx, t, st, options.WithID(machineID), options.LabelCluster(cluster), options.EmptyLabel(omni.LabelWorkerRole))
					rmock.Mock[*omni.ClusterMachine](ctx, t, st, options.WithID(machineID), options.LabelCluster(cluster))
					rmock.Mock[*omni.MachineStatus](ctx, t, st, options.WithID(machineID))
					rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.WithID(machineID), options.Modify(func(res *omni.MachineStatusSnapshot) error {
						if !tc.unknownBootID {
							res.TypedSpec().Value.BootId = "boot-1"
						}

						res.TypedSpec().Value.MachineStatus = &machineapi.MachineStatusEvent{Stage: machineapi.MachineStatusEvent_RUNNING}
						res.TypedSpec().Value.PowerStage = tc.powerStage

						return nil
					}))
					rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, st, options.WithID(machineID))

					if !tc.noClusterMachineConfig {
						rmock.Mock[*omni.ClusterMachineConfig](ctx, t, st, options.WithID(machineID))
					}

					rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, options.WithID(machineID), options.Modify(func(res *omni.ClusterMachineConfigStatus) error {
						if tc.configStatus != nil {
							tc.configStatus(res.TypedSpec().Value)
						}

						return nil
					}))

					if tc.operationStatus != nil {
						operationStatus := omni.NewMachineOperationStatus(machineID)
						operationStatus.TypedSpec().Value = tc.operationStatus

						require.NoError(t, st.Create(ctx, operationStatus))
					}

					// the failure must be shown first, otherwise a reconcile that has not seen the MachineOperationStatus yet passes the final assertion
					if tc.failureShown != nil {
						rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineStatus, assertion *assert.Assertions) {
							assertion.True(tc.failureShown.EqualVT(res.TypedSpec().Value.Operation), "expected %v, got %v", tc.failureShown, res.TypedSpec().Value.Operation)
						})

						rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, st, options.WithID(machineID), options.Modify(func(res *omni.ClusterMachineConfigStatus) error {
							tc.resolve(res.TypedSpec().Value)

							return nil
						}))
					}

					rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineStatus, assertion *assert.Assertions) {
						expectedStage := specs.ClusterMachineStatusSpec_RUNNING
						if tc.powerStage == specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF {
							expectedStage = specs.ClusterMachineStatusSpec_POWERED_OFF
						}

						assertion.Equal(expectedStage, res.TypedSpec().Value.Stage)
						assertion.Equal(tc.configOutdated, !res.TypedSpec().Value.ConfigUpToDate)
						assertion.True(tc.expected.EqualVT(res.TypedSpec().Value.Operation), "expected %v, got %v", tc.expected, res.TypedSpec().Value.Operation)
					})
				},
			)
		})
	}
}
