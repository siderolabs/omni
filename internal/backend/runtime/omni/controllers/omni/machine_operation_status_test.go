// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

type fakeMachineOperations struct {
	operations map[string]lifecycle.OperationState
	changes    chan struct{}
	mu         sync.Mutex
}

func (f *fakeMachineOperations) Operations() map[string]lifecycle.OperationState {
	f.mu.Lock()
	defer f.mu.Unlock()

	return maps.Clone(f.operations)
}

func (f *fakeMachineOperations) Changes() <-chan struct{} {
	return f.changes
}

func (f *fakeMachineOperations) set(operations map[string]lifecycle.OperationState) {
	f.mu.Lock()
	f.operations = operations
	f.mu.Unlock()

	f.changes <- struct{}{}
}

func TestMachineOperationStatusController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	operations := &fakeMachineOperations{changes: make(chan struct{}, 1)}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			ctrl := omnictrl.NewMachineOperationStatusController(operations)
			ctrl.SetSyncInterval(10 * time.Millisecond)

			require.NoError(t, testContext.Runtime.RegisterController(ctrl))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			startedAt := time.Unix(1000, 0).UTC()

			operations.set(map[string]lifecycle.OperationState{
				"m1": {Phase: specs.MachineOperationPhase_PULLING_IMAGE, Message: "pulling", PhaseStartedAt: startedAt},
				"m2": {LastError: "failed", FailedPhase: specs.MachineOperationPhase_UNCORDONING},
			})

			rtestutils.AssertResource(ctx, t, st, "m1", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationPhase_PULLING_IMAGE, res.TypedSpec().Value.Phase)
				assertion.Equal("pulling", res.TypedSpec().Value.Message)
				assertion.Equal(startedAt, res.TypedSpec().Value.PhaseStartedAt.AsTime())
			})

			rtestutils.AssertResource(ctx, t, st, "m2", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationPhase_NONE, res.TypedSpec().Value.Phase)
				assertion.Equal("failed", res.TypedSpec().Value.LastError)
				assertion.Equal(specs.MachineOperationPhase_UNCORDONING, res.TypedSpec().Value.FailedPhase)
			})

			operations.set(map[string]lifecycle.OperationState{
				"m1": {Phase: specs.MachineOperationPhase_UPGRADING, PhaseStartedAt: startedAt},
			})

			rtestutils.AssertResource(ctx, t, st, "m1", func(res *omni.MachineOperationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.MachineOperationPhase_UPGRADING, res.TypedSpec().Value.Phase)
				assertion.Empty(res.TypedSpec().Value.Message)
			})

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, "m2")

			operations.set(nil)

			rtestutils.AssertNoResource[*omni.MachineOperationStatus](ctx, t, st, "m1")
		},
	)
}
