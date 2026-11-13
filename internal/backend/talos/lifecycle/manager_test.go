// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

func newManagerForTest(t *testing.T) (*lifecycle.Manager, state.State) {
	t.Helper()

	c, err := imagefactory.NewClient("factory.talos.dev", imagefactory.Auth{})
	require.NoError(t, err)

	st := state.WrapCore(inmem.NewState())

	return lifecycle.NewManager(zapNop(t), imagefactory.NewClients(st, c), nil, nil, nil, st), st
}

// machineOperation returns the MachineOperation spec of the machine, or nil if it has none.
func machineOperation(t *testing.T, st state.State, machineID string) *specs.MachineOperationSpec {
	t.Helper()

	operation, err := safe.StateGetByID[*omni.MachineOperation](t.Context(), st, machineID)
	if state.IsNotFoundError(err) || !assert.NoError(t, err) {
		return nil
	}

	return operation.TypedSpec().Value
}

func TestCheckTalosVersion(t *testing.T) {
	t.Parallel()

	m, _ := newManagerForTest(t)

	for _, tc := range []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "supported 1.13", version: "1.13.0", wantErr: false},
		{name: "supported with v prefix", version: "v1.13.4", wantErr: false},
		{name: "supported 1.14 alpha", version: "1.14.0-alpha.1", wantErr: false},
		{name: "supported next major", version: "2.0.0", wantErr: false},
		{name: "unsupported 1.12", version: "1.12.5", wantErr: true},
		{name: "unsupported 1.11", version: "1.11.0", wantErr: true},
		{name: "unparseable", version: "garbage", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := m.SupportsLifecycleManagement(tc.version)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "install", lifecycle.KindInstall.String())
	assert.Equal(t, "upgrade", lifecycle.KindUpgrade.String())
}

func TestInFlightGuard(t *testing.T) {
	t.Parallel()

	m, _ := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"), "first acquire should succeed")
	require.False(t, m.AcquireForTest("machine-1"), "second acquire should be rejected while held")
	require.True(t, m.AcquireForTest("machine-2"), "a different machine is independent")

	m.ReleaseForTest("machine-1", nil)
	require.True(t, m.AcquireForTest("machine-1"), "acquire should succeed again after release")
}

func TestAcquireClearsFailedOperation(t *testing.T) {
	t.Parallel()

	m, st := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"))
	m.ReleaseForTest("machine-1", errors.New("failed to pull installer image"))
	require.Equal(t, "failed to pull installer image", machineOperation(t, st, "machine-1").GetLastError())

	require.True(t, m.AcquireForTest("machine-1"))

	operation := machineOperation(t, st, "machine-1")
	require.NotNil(t, operation)
	assert.Empty(t, operation.LastError, "a new operation must not show the failure of the previous one")
}

func TestForget(t *testing.T) {
	t.Parallel()

	m, st := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"))

	m.Forget("machine-1", false)
	assert.NotNil(t, machineOperation(t, st, "machine-1"), "an operation in flight must not be forgotten")

	m.ReleaseForTest("machine-1", errors.New("failed"))
	m.Forget("machine-1", false)
	assert.Nil(t, machineOperation(t, st, "machine-1"))
}

func TestForgetRemovedMachine(t *testing.T) {
	t.Parallel()

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		m, st := newManagerForTest(t)

		require.True(t, m.AcquireForTest("machine-1"))

		m.Forget("machine-1", true)
		assert.NotNil(t, machineOperation(t, st, "machine-1"), "an operation in flight must not be dropped before it finishes")

		m.ReleaseForTest("machine-1", errors.New("failed"))
		assert.Nil(t, machineOperation(t, st, "machine-1"), "the failure of a removed machine must not be kept")
	})

	t.Run("kept rebooting stage", func(t *testing.T) {
		t.Parallel()

		m, _, op, st := setupOperation(t, "machine-1")
		setBootID(t, st, "machine-1", "boot-1")

		require.NoError(t, m.Run(t.Context(), op, lifecycle.WithProgress(func(string) {
			m.Forget("machine-1", true)
		})))
		assert.Nil(t, machineOperation(t, st, "machine-1"), "the kept stage of a removed machine must not be kept")
	})
}

func TestLateProgressIsIgnored(t *testing.T) {
	t.Parallel()

	m, st := newManagerForTest(t)

	progress, started := m.StartForTest("machine-1")
	require.True(t, started)

	progress("pulling")
	m.ReleaseForTest("machine-1", errors.New("failed"))

	progress("late message")
	assert.Equal(t, "pulling", machineOperation(t, st, "machine-1").GetMessage(), "a failed operation must keep its last message")

	_, started = m.StartForTest("machine-1")
	require.True(t, started)

	progress("late message")
	assert.Empty(t, machineOperation(t, st, "machine-1").GetMessage(), "the progress of an operation must not reach the next one")
}

func setBootID(t *testing.T, st state.State, machineID, bootID string) {
	t.Helper()

	snapshot := omni.NewMachineStatusSnapshot(machineID)
	snapshot.TypedSpec().Value.BootId = bootID

	require.NoError(t, st.Create(t.Context(), snapshot))
}

func TestReleaseDestroysMachineOperation(t *testing.T) {
	t.Parallel()

	m, st := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"))
	assert.Equal(t, specs.MachineOperationSpec_PREPARING_IMAGE, machineOperation(t, st, "machine-1").GetStage())

	m.ReleaseForTest("machine-1", nil)
	assert.Nil(t, machineOperation(t, st, "machine-1"))

	m.ReleaseForTest("machine-2", nil)
	m.Forget("machine-2", false)
	assert.Nil(t, machineOperation(t, st, "machine-2"), "a machine without an operation must not get a MachineOperation")
}

func TestRunRejectsWhenInFlight(t *testing.T) {
	t.Parallel()

	m, _ := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"))

	// MachineStatus is never touched: Run returns at the in-flight guard before building the image.
	err := m.Run(t.Context(), lifecycle.Operation{MachineID: "machine-1", Kind: lifecycle.KindUpgrade})
	require.ErrorIs(t, err, lifecycle.ErrAlreadyInFlight)
}

func TestRunReleasesSlotOnFailure(t *testing.T) {
	t.Parallel()

	m, _ := newManagerForTest(t)

	// No platform metadata → the flow fails fast in buildInstallImage.
	ms := omni.NewMachineStatus("machine-1")
	ms.TypedSpec().Value.TalosVersion = "1.13.1"

	err := m.Run(t.Context(), lifecycle.Operation{
		MachineID:     "machine-1",
		MachineStatus: ms,
		Kind:          lifecycle.KindUpgrade,
	})
	require.Error(t, err)
	assert.NotErrorIs(t, err, lifecycle.ErrAlreadyInFlight)

	// The slot was released, so the machine can be acquired again.
	require.True(t, m.AcquireForTest("machine-1"))
}

// watchStages watches the MachineOperation of the machine. The returned function waits until the MachineOperation
// is destroyed and returns the stages it went through, without consecutive duplicates.
func watchStages(t *testing.T, st state.State, machineID string) func() []specs.MachineOperationSpec_Stage {
	t.Helper()

	events := make(chan state.Event)

	require.NoError(t, st.Watch(t.Context(), omni.NewMachineOperation(machineID).Metadata(), events))

	return func() []specs.MachineOperationSpec_Stage {
		var stages []specs.MachineOperationSpec_Stage

		for {
			var event state.Event

			select {
			case <-t.Context().Done():
				require.FailNow(t, "the machine operation was not destroyed")
			case event = <-events:
			}

			switch event.Type { //nolint:exhaustive
			case state.Errored:
				require.NoError(t, event.Error)
			case state.Destroyed:
				if len(stages) > 0 { // the first event is a destroyed one if the resource does not exist yet
					return stages
				}
			case state.Created, state.Updated:
				operation, ok := event.Resource.(*omni.MachineOperation)
				require.True(t, ok)

				if stage := operation.TypedSpec().Value.Stage; len(stages) == 0 || stages[len(stages)-1] != stage {
					stages = append(stages, stage)
				}
			}
		}
	}
}

func setupOperation(t *testing.T, machineID string) (*lifecycle.Manager, *testutils.MachineServiceMock, lifecycle.Operation, state.State) {
	t.Helper()

	st := state.WrapCore(omniruntime.TestStateBuilder())

	machineService := testutils.NewMachineServiceMock(t.Context(), t, machineID, st)

	ms := omni.NewMachineStatus(machineID)
	ms.TypedSpec().Value.TalosVersion = "1.13.1"
	ms.TypedSpec().Value.ManagementAddress = machineService.SocketConnectionString
	ms.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{Platform: "metal"}
	ms.TypedSpec().Value.SecurityState = &specs.SecurityState{}

	require.NoError(t, st.Create(t.Context(), ms))

	k8sProvider := &testutils.FakeKubernetesProvider{}
	k8sProvider.SetClientset(testutils.NewFakeKubernetesClientset("node-1"))

	return testutils.NewLifecycleManager(t, st, k8sProvider), machineService, lifecycle.Operation{
		MachineID:     machineID,
		MachineStatus: ms,
		Kind:          lifecycle.KindUpgrade,
		InstallImage: &specs.MachineConfigGenOptionsSpec_InstallImage{
			TalosVersion:         "1.13.2",
			SchematicId:          "schematic",
			SchematicInitialized: true,
			Platform:             "metal",
			SecurityState:        &specs.SecurityState{},
			ImageFactoryHost:     "factory.talos.dev",
		},
	}, st
}

func TestRunRecordsStages(t *testing.T) {
	t.Parallel()

	m, machineService, op, st := setupOperation(t, "machine-1")
	stages := watchStages(t, st, "machine-1")

	machineService.SetLifecycleUpgradeHandler(func(_ *machine.LifecycleServiceUpgradeRequest, srv grpc.ServerStreamingServer[machine.LifecycleServiceUpgradeResponse]) error {
		return srv.Send(&machine.LifecycleServiceUpgradeResponse{
			Progress: &machine.LifecycleServiceInstallProgress{
				Response: &machine.LifecycleServiceInstallProgress_Message{Message: "writing the image"},
			},
		})
	})

	var messages []string

	err := m.Run(t.Context(), op,
		lifecycle.WithCordonDrain("cluster", "node-1", false),
		lifecycle.WithPreRebootHooks(func(context.Context, *talosclient.Client) error { return nil }),
		lifecycle.WithProgress(func(msg string) {
			assert.Equal(t, msg, machineOperation(t, st, "machine-1").GetMessage())

			messages = append(messages, msg)
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, []specs.MachineOperationSpec_Stage{
		specs.MachineOperationSpec_PREPARING_IMAGE,
		specs.MachineOperationSpec_PULLING_IMAGE,
		specs.MachineOperationSpec_UPGRADING,
		specs.MachineOperationSpec_PREPARING_REBOOT,
		specs.MachineOperationSpec_CORDONING,
		specs.MachineOperationSpec_DRAINING,
		specs.MachineOperationSpec_REBOOTING,
	}, stages())
	assert.Contains(t, messages, "[talos] writing the image")
	assert.Nil(t, machineOperation(t, st, "machine-1"))
}

func TestRunKeepsRebootingStage(t *testing.T) {
	t.Parallel()

	t.Run("known boot ID", func(t *testing.T) {
		t.Parallel()

		m, _, op, st := setupOperation(t, "machine-1")
		setBootID(t, st, "machine-1", "boot-1")

		require.NoError(t, m.Run(t.Context(), op))

		operation := machineOperation(t, st, "machine-1")
		require.NotNil(t, operation)
		assert.Equal(t, specs.MachineOperationSpec_REBOOTING, operation.Stage)
		assert.Equal(t, "boot-1", operation.PreRebootBootId)

		require.True(t, m.AcquireForTest("machine-1"), "the kept stage must not hold the in-flight slot")
		m.ReleaseForTest("machine-1", nil)

		require.NoError(t, m.Run(t.Context(), op))

		m.Forget("machine-1", false)
		assert.Nil(t, machineOperation(t, st, "machine-1"))
	})

	t.Run("unknown boot ID", func(t *testing.T) {
		t.Parallel()

		m, _, op, st := setupOperation(t, "machine-1")

		require.NoError(t, m.Run(t.Context(), op))
		assert.Nil(t, machineOperation(t, st, "machine-1"), "without a boot ID, nothing can tell that the machine is back")
	})
}

func TestRunReportsFailedStage(t *testing.T) {
	t.Parallel()

	t.Run("image pull", func(t *testing.T) {
		t.Parallel()

		m, machineService, op, st := setupOperation(t, "machine-1")

		machineService.SetImagePullHandler(func(*machine.ImageServicePullRequest) error {
			return status.Error(codes.PermissionDenied, "unauthorized")
		})

		err := m.Run(t.Context(), op)
		require.ErrorContains(t, err, "unauthorized")

		operation := machineOperation(t, st, "machine-1")
		require.NotNil(t, operation)

		assert.Equal(t, specs.MachineOperationSpec_PULLING_IMAGE, operation.FailedStage)
		assert.Equal(t, specs.MachineOperationSpec_UNKNOWN, operation.Stage)
		assert.NotEmpty(t, operation.LastError)
	})

	t.Run("upgrade", func(t *testing.T) {
		t.Parallel()

		m, machineService, op, st := setupOperation(t, "machine-1")

		machineService.SetLifecycleUpgradeHandler(func(_ *machine.LifecycleServiceUpgradeRequest, srv grpc.ServerStreamingServer[machine.LifecycleServiceUpgradeResponse]) error {
			if err := srv.Send(&machine.LifecycleServiceUpgradeResponse{
				Progress: &machine.LifecycleServiceInstallProgress{
					Response: &machine.LifecycleServiceInstallProgress_Message{Message: "writing the image"},
				},
			}); err != nil {
				return err
			}

			return status.Error(codes.Internal, "disk full")
		})

		err := m.Run(t.Context(), op)
		require.ErrorContains(t, err, "disk full")

		operation := machineOperation(t, st, "machine-1")
		require.NotNil(t, operation)

		assert.Equal(t, specs.MachineOperationSpec_UPGRADING, operation.FailedStage)
		assert.Equal(t, "[talos] writing the image", operation.Message, "a failed operation must keep its last message")
	})

	t.Run("install image build", func(t *testing.T) {
		t.Parallel()

		m, _, op, st := setupOperation(t, "machine-1")
		op.InstallImage = nil

		err := m.Run(t.Context(), op)
		require.ErrorContains(t, err, "failed to build install image")

		operation := machineOperation(t, st, "machine-1")
		require.NotNil(t, operation)

		assert.Equal(t, specs.MachineOperationSpec_PREPARING_IMAGE, operation.FailedStage)
		assert.Equal(t, specs.MachineOperationSpec_UNKNOWN, operation.Stage)
		assert.NotEmpty(t, operation.LastError)
	})
}

func zapNop(t *testing.T) *zap.Logger {
	t.Helper()

	return zaptest.NewLogger(t)
}
