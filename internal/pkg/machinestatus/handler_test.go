// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machinestatus_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/rs/xid"
	"github.com/siderolabs/siderolink/pkg/events"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/machinestatus"
)

const (
	machineID     = "machine-status-handler-test"
	machineIP     = "127.0.0.42"
	machinePort   = "1234"
	machineIPPort = machineIP + ":" + machinePort
)

func TestHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(st)

	require.NoError(t, resourceRegistry.Register(ctx, omni.NewMachineStatus(resources.DefaultNamespace, "")))
	require.NoError(t, resourceRegistry.Register(ctx, omni.NewMachine(resources.DefaultNamespace, "")))

	handler := machinestatus.NewHandler(st, zaptest.NewLogger(t))

	// send an event without corresponding machine - assert that it is ignored
	require.ErrorContains(t, handler.HandleEvent(ctx, events.Event{
		Payload: &machineapi.MachineStatusEvent{
			Stage: machineapi.MachineStatusEvent_BOOTING,
		},
		ID:   xid.NewWithTime(time.Now()).String(),
		Node: machineIPPort,
	}), "no machines found for address "+machineIP)

	rtestutils.AssertLength[*omni.MachineStatusSnapshot](ctx, t, st, 0)

	// create a machine

	machine := omni.NewMachine(resources.DefaultNamespace, machineID)

	machine.Metadata().Labels().Set(omni.MachineAddressLabel, machineIP)

	require.NoError(t, st.Create(ctx, machine))

	var eg errgroup.Group

	eg.Go(func() error {
		return handler.Start(ctx)
	})

	// send an event over siderolink & assert that it is stored
	timestamp := time.Now()

	sendEvent(ctx, t, handler, machineapi.MachineStatusEvent_BOOTING, timestamp)
	assertStage(ctx, t, st, machineapi.MachineStatusEvent_BOOTING)

	// send another event over siderolink in the past - it should be stored despite being in the past, as the previous status was also received over siderolink
	sendEvent(ctx, t, handler, machineapi.MachineStatusEvent_INSTALLING, timestamp)
	assertStage(ctx, t, st, machineapi.MachineStatusEvent_INSTALLING)

	// send a machine status in the past - it should be ignored
	sendMachineStatus(ctx, t, st, machineapi.MachineStatusEvent_BOOTING, timestamp.Add(-time.Second))
	time.Sleep(100 * time.Millisecond)
	assertStage(ctx, t, st, machineapi.MachineStatusEvent_INSTALLING)

	// send a machine status in the future - it should be stored
	sendMachineStatus(ctx, t, st, machineapi.MachineStatusEvent_REBOOTING, timestamp.Add(time.Second))
	assertStage(ctx, t, st, machineapi.MachineStatusEvent_REBOOTING)

	// send an event in the past - it should be ignored
	sendEvent(ctx, t, handler, machineapi.MachineStatusEvent_INSTALLING, timestamp.Add(-time.Second))
	assertStage(ctx, t, st, machineapi.MachineStatusEvent_REBOOTING)

	// send an event in the future - it should be stored
	sendEvent(ctx, t, handler, machineapi.MachineStatusEvent_REBOOTING, timestamp.Add(time.Second))
	assertStage(ctx, t, st, machineapi.MachineStatusEvent_REBOOTING)

	// destroy and recreate the machine
	require.NoError(t, st.Destroy(ctx, machine.Metadata()))
	require.NoError(t, st.Create(ctx, machine))

	time.Sleep(100 * time.Millisecond)

	// send a machine status in the past - it should be stored despite being in the past, as the state should be cleared on machine destroy
	sendMachineStatus(ctx, t, st, machineapi.MachineStatusEvent_RESETTING, timestamp.Add(-time.Second))
	assertStage(ctx, t, st, machineapi.MachineStatusEvent_RESETTING)

	cancel()

	require.NoError(t, eg.Wait())
}

func sendMachineStatus(ctx context.Context, t *testing.T, st state.State, stage machineapi.MachineStatusEvent_MachineStage, timestamp time.Time) {
	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, machineID)

	status := &specs.MachineStatusSpec_TalosMachineStatus{
		Status: &machineapi.MachineStatusEvent{
			Stage: stage,
		},
		UpdatedAt: timestamppb.New(timestamp),
	}

	_, err := safe.StateUpdateWithConflicts[*omni.MachineStatus](ctx, st, machineStatus.Metadata(), func(r *omni.MachineStatus) error {
		r.TypedSpec().Value.TalosMachineStatus = status

		return nil
	})
	if err == nil {
		return
	}

	if !state.IsNotFoundError(err) {
		require.NoError(t, err)
	}

	machineStatus.TypedSpec().Value.TalosMachineStatus = status

	require.NoError(t, st.Create(ctx, machineStatus))
}

func sendEvent(ctx context.Context, t *testing.T, handler *machinestatus.Handler, stage machineapi.MachineStatusEvent_MachineStage, timestamp time.Time) {
	err := handler.HandleEvent(ctx, events.Event{
		Payload: &machineapi.MachineStatusEvent{
			Stage: stage,
		},
		ID:   xid.NewWithTime(timestamp).String(),
		Node: machineIPPort,
	})
	require.NoError(t, err)
}

func assertStage(ctx context.Context, t *testing.T, st state.State, stage machineapi.MachineStatusEvent_MachineStage) {
	rtestutils.AssertResource[*omni.MachineStatusSnapshot](ctx, t, st, machineID, func(r *omni.MachineStatusSnapshot, assertion *assert.Assertions) {
		assertion.Equal(stage, r.TypedSpec().Value.GetMachineStatus().GetStage())
	})
}
