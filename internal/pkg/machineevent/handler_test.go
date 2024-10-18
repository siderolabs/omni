// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineevent_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/siderolink/pkg/events"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/machineevent"
)

func TestSequenceEvent(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)
	handler := machineevent.NewHandler(st, logger, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	testMachine := omni.NewMachine(resources.DefaultNamespace, "test-machine")
	infraMachine := infra.NewMachine("test-machine")

	testMachine.Metadata().Labels().Set(omni.MachineAddressLabel, "127.0.0.1")

	require.NoError(t, st.Create(ctx, testMachine))
	require.NoError(t, st.Create(ctx, infraMachine))

	event := events.Event{
		Node: "127.0.0.1:4242",
		Payload: &machine.SequenceEvent{
			Sequence: "initialize",
		},
	}

	require.NoError(t, handler.HandleEvent(ctx, event))

	assertInfraMachineState(ctx, t, st, false, false)

	// assert installed condition 1

	event.Payload = &machine.SequenceEvent{
		Sequence: "install",
		Action:   machine.SequenceEvent_NOOP,
		Error: &common.Error{
			Code:    common.Code_FATAL,
			Message: "something unix.Reboot something",
		},
	}

	require.NoError(t, handler.HandleEvent(ctx, event))

	assertInfraMachineState(ctx, t, st, true, true)

	require.NoError(t, st.Destroy(ctx, infra.NewMachineState("test-machine").Metadata()))

	// assert installed condition 2

	event.Payload = &machine.SequenceEvent{
		Sequence: "boot",
		Action:   machine.SequenceEvent_START,
	}

	require.NoError(t, handler.HandleEvent(ctx, event))

	assertInfraMachineState(ctx, t, st, true, true)

	// remove the infra machine, assert that state is removed

	require.NoError(t, st.Destroy(ctx, infraMachine.Metadata()))

	event.Payload = &machine.SequenceEvent{
		Sequence: "something",
	}

	require.NoError(t, handler.HandleEvent(ctx, event))

	assertInfraMachineState(ctx, t, st, false, false)
}

func assertInfraMachineState(ctx context.Context, t *testing.T, st state.State, exists, installed bool) {
	machineState, err := safe.StateGetByID[*infra.MachineState](ctx, st, "test-machine")
	if err != nil {
		if !state.IsNotFoundError(err) {
			require.NoError(t, err)
		}

		if exists {
			require.Fail(t, "machine state not found")
		}

		return
	}

	assert.Equal(t, installed, machineState.TypedSpec().Value.Installed)
}
