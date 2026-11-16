// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle_test

import (
	"context"
	"strings"
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
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	k8s "k8s.io/client-go/kubernetes"
	ktesting "k8s.io/client-go/testing"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/machineoperation"
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
	), nil, nil, nil, nil)
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

	m.ReleaseForTest("machine-1")
	require.True(t, m.AcquireForTest("machine-1"), "acquire should succeed again after release")
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

func setupOperation(t *testing.T, machineID string) (*lifecycle.Manager, *machineoperation.Reporter, *testutils.MachineServiceMock, lifecycle.Operation) {
	t.Helper()

	return setupOperationWithClientset(t, machineID, testutils.NewFakeKubernetesClientset("node-1"))
}

func setupOperationWithClientset(
	t *testing.T, machineID string, clientset k8s.Interface,
) (*lifecycle.Manager, *machineoperation.Reporter, *testutils.MachineServiceMock, lifecycle.Operation) {
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
	k8sProvider.SetClientset(clientset)

	operations := machineoperation.NewReporter()

	return testutils.NewReportingLifecycleManager(t, st, k8sProvider, operations), operations, machineService, lifecycle.Operation{
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

// reportedOperation returns what the operation of the machine reported last, nil if the machine has no operation.
func reportedOperation(t *testing.T, operations *machineoperation.Reporter, machineID string) *machineoperation.State {
	t.Helper()

	operation, ok := operations.Snapshot()[machineID]
	if !ok {
		return nil
	}

	return &operation
}

func TestRunReportsStages(t *testing.T) {
	t.Parallel()

	m, operations, machineService, op := setupOperation(t, "machine-1")

	var (
		mu     sync.Mutex
		stages []machineoperation.Stage
	)

	// the reporter keeps only the latest state, so the stage is read inside each step
	recordStage := func() {
		mu.Lock()
		defer mu.Unlock()

		operation := reportedOperation(t, operations, "machine-1")
		if !assert.NotNil(t, operation) {
			return
		}

		if len(stages) == 0 || stages[len(stages)-1] != operation.Stage {
			stages = append(stages, operation.Stage)
		}
	}

	machineService.SetImagePullHandler(func(*machine.ImageServicePullRequest) error {
		recordStage()

		return nil
	})

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
		lifecycle.WithPreRebootHooks(func(context.Context, *talosclient.Client) error {
			recordStage()

			return nil
		}),
		lifecycle.WithProgress(func(msg string) {
			operation := reportedOperation(t, operations, "machine-1")
			if assert.NotNil(t, operation) {
				assert.Equal(t, msg, operation.Message)
			}

			recordStage()

			messages = append(messages, msg)
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, []machineoperation.Stage{
		specs.MachineOperationStatusSpec_PULLING_IMAGE,
		specs.MachineOperationStatusSpec_UPGRADING,
		specs.MachineOperationStatusSpec_PREPARING_REBOOT,
		specs.MachineOperationStatusSpec_CORDONING,
		specs.MachineOperationStatusSpec_DRAINING,
		specs.MachineOperationStatusSpec_REBOOTING,
	}, stages)
	assert.Contains(t, messages, "[talos] writing the image")
	assert.Nil(t, reportedOperation(t, operations, "machine-1"), "the operation must end with the call when the boot ID is not known")
}

func TestRunKeepsPodMessagesOutOfOperation(t *testing.T) {
	t.Parallel()

	clientset := testutils.NewFakeKubernetesClientset("node-1")

	_, err := clientset.CoreV1().Pods("default").Create(t.Context(), &corev1.Pod{
		Name:      "web",
		Namespace: "default",
		Spec:      corev1.PodSpec{NodeName: "node-1"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	clientset.PrependReactor("create", "pods", func(action ktesting.Action) (bool, k8sruntime.Object, error) {
		if action.GetSubresource() != "eviction" {
			return false, nil, nil
		}

		eviction := action.(ktesting.CreateAction).GetObject().(*policyv1.Eviction) //nolint:forcetypeassert,errcheck

		return true, nil, clientset.Tracker().Delete(corev1.SchemeGroupVersion.WithResource("pods"), eviction.Namespace, eviction.Name)
	})

	m, operations, _, op := setupOperationWithClientset(t, "machine-1", clientset)

	var podMessages []string

	err = m.Run(t.Context(), op,
		lifecycle.WithCordonDrain("cluster", "node-1", false),
		lifecycle.WithProgress(func(msg string) {
			if !strings.Contains(msg, "pod default/web") {
				return
			}

			podMessages = append(podMessages, msg)

			if operation := reportedOperation(t, operations, "machine-1"); assert.NotNil(t, operation) {
				assert.Equal(t, "[omni] draining node node-1", operation.Message)
			}
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"[omni] evicting pod default/web", "[omni] evicted pod default/web"}, podMessages)
}

func TestRunReportsRebootUntilMachineIsBack(t *testing.T) {
	t.Parallel()

	m, operations, _, op := setupOperation(t, "machine-1")
	op.BootID = "boot-1"

	require.NoError(t, m.Run(t.Context(), op))

	operation := reportedOperation(t, operations, "machine-1")
	require.NotNil(t, operation)
	assert.Equal(t, specs.MachineOperationStatusSpec_REBOOTING, operation.Stage)
	assert.Equal(t, "boot-1", operation.BootID)

	require.True(t, m.AcquireForTest("machine-1"), "the reported reboot must not hold the in-flight slot")
	m.ReleaseForTest("machine-1")

	operations.Booted("machine-1", "boot-2")

	assert.Nil(t, reportedOperation(t, operations, "machine-1"))
}

func TestRunEndsOperationOnFailure(t *testing.T) {
	t.Parallel()

	m, operations, machineService, op := setupOperation(t, "machine-1")
	op.BootID = "boot-1"

	machineService.SetImagePullHandler(func(*machine.ImageServicePullRequest) error {
		return status.Error(codes.PermissionDenied, "unauthorized")
	})

	require.ErrorContains(t, m.Run(t.Context(), op), "unauthorized")

	assert.Nil(t, reportedOperation(t, operations, "machine-1"))
}

func TestFinalizeRebootReportsUncordon(t *testing.T) {
	t.Parallel()

	m, operations, _, _ := setupOperation(t, "machine-1")

	var stage machineoperation.Stage

	err := m.FinalizeReboot(t.Context(), "machine-1",
		lifecycle.WithUncordon("cluster", "node-1"),
		lifecycle.WithProgress(func(string) {
			if operation := reportedOperation(t, operations, "machine-1"); assert.NotNil(t, operation) {
				stage = operation.Stage
			}
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, specs.MachineOperationStatusSpec_UNCORDONING, stage)
	assert.Nil(t, reportedOperation(t, operations, "machine-1"))
}

func zapNop(t *testing.T) *zap.Logger {
	t.Helper()

	return zaptest.NewLogger(t)
}
