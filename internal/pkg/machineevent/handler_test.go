// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineevent_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
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
	installEventCh := make(chan resource.ID, 1)
	handler := machineevent.NewHandler(st, logger, nil, installEventCh)

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

	assert.Len(t, installEventCh, 0)

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

	assert.Len(t, installEventCh, 1)

	select {
	case <-ctx.Done():
		require.Fail(t, "timeout")
	case installEvent := <-installEventCh:
		assert.Equal(t, testMachine.Metadata().ID(), installEvent)
	}

	// assert installed condition 2

	event.Payload = &machine.SequenceEvent{
		Sequence: "boot",
		Action:   machine.SequenceEvent_START,
	}

	require.NoError(t, handler.HandleEvent(ctx, event))

	assert.Len(t, installEventCh, 1)

	select {
	case <-ctx.Done():
		require.Fail(t, "timeout")
	case <-installEventCh:
	}
}
