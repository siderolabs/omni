// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

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

func newManagerForTest(t *testing.T) *lifecycle.Manager {
	t.Helper()

	c, err := imagefactory.NewClient("factory.talos.dev", imagefactory.Auth{})
	require.NoError(t, err)

	return lifecycle.NewManager(zapNop(t), imagefactory.NewClients(
		state.WrapCore(inmem.NewState()),
		c,
	), nil, nil, nil)
}

func TestCheckTalosVersion(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

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

	m := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"), "first acquire should succeed")
	require.False(t, m.AcquireForTest("machine-1"), "second acquire should be rejected while held")
	require.True(t, m.AcquireForTest("machine-2"), "a different machine is independent")

	m.ReleaseForTest("machine-1", nil)
	require.True(t, m.AcquireForTest("machine-1"), "acquire should succeed again after release")
}

func TestAcquireClearsFailedOperation(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"))
	m.ReleaseForTest("machine-1", errors.New("failed to pull installer image"))
	require.Equal(t, "failed to pull installer image", m.Operations()["machine-1"].LastError)

	require.True(t, m.AcquireForTest("machine-1"))
	assert.Empty(t, m.Operations()["machine-1"].LastError, "a new operation must not show the failure of the previous one")
}

func TestForget(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"))

	m.Forget("machine-1")
	assert.Contains(t, m.Operations(), "machine-1", "an operation in flight must not be forgotten")

	m.ReleaseForTest("machine-1", errors.New("failed"))
	m.Forget("machine-1")
	assert.NotContains(t, m.Operations(), "machine-1")
}

func TestRunRejectsWhenInFlight(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"))

	// MachineStatus is never touched: Run returns at the in-flight guard before building the image.
	err := m.Run(t.Context(), lifecycle.Operation{MachineID: "machine-1", Kind: lifecycle.KindUpgrade})
	require.ErrorIs(t, err, lifecycle.ErrAlreadyInFlight)
}

func TestRunReleasesSlotOnFailure(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

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

// phaseRecorder records the phases of the operation on a machine, without consecutive duplicates.
type phaseRecorder struct {
	m         *lifecycle.Manager
	machineID string
	phases    []specs.MachineOperationPhase
	mu        sync.Mutex
}

func (r *phaseRecorder) record() {
	r.mu.Lock()
	defer r.mu.Unlock()

	phase := r.m.Operations()[r.machineID].Phase

	if len(r.phases) == 0 || r.phases[len(r.phases)-1] != phase {
		r.phases = append(r.phases, phase)
	}
}

func (r *phaseRecorder) get() []specs.MachineOperationPhase {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.phases)
}

func setupOperation(t *testing.T, machineID string) (*lifecycle.Manager, *testutils.MachineServiceMock, lifecycle.Operation) {
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
	}
}

func TestRunRecordsPhases(t *testing.T) {
	t.Parallel()

	m, machineService, op := setupOperation(t, "machine-1")
	recorder := &phaseRecorder{m: m, machineID: "machine-1"}

	machineService.SetLifecycleUpgradeHandler(func(_ *machine.LifecycleServiceUpgradeRequest, srv grpc.ServerStreamingServer[machine.LifecycleServiceUpgradeResponse]) error {
		return srv.Send(&machine.LifecycleServiceUpgradeResponse{
			Progress: &machine.LifecycleServiceInstallProgress{
				Response: &machine.LifecycleServiceInstallProgress_Message{Message: "writing the image"},
			},
		})
	})

	var messages []string

	err := m.Run(t.Context(), op,
		lifecycle.WithPhase(func(specs.MachineOperationPhase) { recorder.record() }),
		lifecycle.WithCordonDrain("cluster", "node-1", false),
		lifecycle.WithPreRebootHooks(func(context.Context, *talosclient.Client) error {
			recorder.record()

			return nil
		}),
		lifecycle.WithProgress(func(msg string) {
			recorder.record()

			assert.Equal(t, msg, m.Operations()["machine-1"].Message)

			messages = append(messages, msg)
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, []specs.MachineOperationPhase{
		specs.MachineOperationPhase_PREPARING_IMAGE,
		specs.MachineOperationPhase_PULLING_IMAGE,
		specs.MachineOperationPhase_UPGRADING,
		specs.MachineOperationPhase_PREPARING_REBOOT,
		specs.MachineOperationPhase_CORDONING,
		specs.MachineOperationPhase_DRAINING,
		specs.MachineOperationPhase_REBOOTING,
	}, recorder.get())
	assert.Contains(t, messages, "[talos] writing the image")
	assert.Empty(t, m.Operations())
}

func TestRunReportsFailedPhase(t *testing.T) {
	t.Parallel()

	t.Run("image pull", func(t *testing.T) {
		t.Parallel()

		m, machineService, op := setupOperation(t, "machine-1")

		machineService.SetImagePullHandler(func(*machine.ImageServicePullRequest) error {
			return status.Error(codes.PermissionDenied, "unauthorized")
		})

		var lastPhase specs.MachineOperationPhase

		err := m.Run(t.Context(), op, lifecycle.WithPhase(func(phase specs.MachineOperationPhase) { lastPhase = phase }))
		require.ErrorContains(t, err, "unauthorized")

		assert.Equal(t, specs.MachineOperationPhase_PULLING_IMAGE, lastPhase)
		assert.Empty(t, m.Operations())
	})

	t.Run("install image build", func(t *testing.T) {
		t.Parallel()

		m, _, op := setupOperation(t, "machine-1")
		op.InstallImage = nil

		var lastPhase specs.MachineOperationPhase

		err := m.Run(t.Context(), op, lifecycle.WithPhase(func(phase specs.MachineOperationPhase) { lastPhase = phase }))
		require.ErrorContains(t, err, "failed to build install image")

		assert.Equal(t, specs.MachineOperationPhase_PREPARING_IMAGE, lastPhase)
		assert.Empty(t, m.Operations())
	})
}

func zapNop(t *testing.T) *zap.Logger {
	t.Helper()

	return zaptest.NewLogger(t)
}
