// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineoperation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/machineoperation"
)

const (
	pulling   = specs.MachineOperationStatusSpec_PULLING_IMAGE
	draining  = specs.MachineOperationStatusSpec_DRAINING
	rebooting = specs.MachineOperationStatusSpec_REBOOTING
)

// changed asserts that a change was signaled, and returns the state of every operation.
func changed(t *testing.T, reporter *machineoperation.Reporter) map[string]machineoperation.State {
	t.Helper()

	select {
	case <-reporter.Changes():
	default:
		require.Fail(t, "no change signaled")
	}

	return reporter.Snapshot()
}

// unchanged asserts that no change was signaled.
func unchanged(t *testing.T, reporter *machineoperation.Reporter) {
	t.Helper()

	select {
	case <-reporter.Changes():
		require.Fail(t, "a change was signaled")
	default:
	}
}

func TestOnlyLatestStateIsKept(t *testing.T) {
	t.Parallel()

	reporter := machineoperation.NewReporter()

	operation := reporter.Start("a", pulling)
	operation.SetMessage("pulling")
	operation.SetStage(draining)
	operation.SetMessage("evicting pod 1")
	operation.SetMessage("evicting pod 2")

	states := changed(t, reporter)
	require.Len(t, states, 1)
	assert.Equal(t, draining, states["a"].Stage)
	assert.Equal(t, "evicting pod 2", states["a"].Message)
	assert.False(t, states["a"].StartedAt.IsZero())

	unchanged(t, reporter)
}

func TestEnd(t *testing.T) {
	t.Parallel()

	reporter := machineoperation.NewReporter()

	operation := reporter.Start("a", pulling)
	other := reporter.Start("b", pulling)

	changed(t, reporter)

	operation.End("")

	states := changed(t, reporter)
	assert.NotContains(t, states, "a", "the ended operation is gone")
	assert.Contains(t, states, "b")

	// a late progress message of the ended operation is dropped
	operation.SetMessage("late")
	operation.SetStage(draining)
	operation.End("")

	unchanged(t, reporter)

	other.End("")
}

func TestStartReplacesOperation(t *testing.T) {
	t.Parallel()

	reporter := machineoperation.NewReporter()

	first := reporter.Start("a", pulling)
	second := reporter.Start("a", draining)

	// the replaced operation can neither change nor end the new one
	first.SetMessage("late")
	first.End("")

	states := changed(t, reporter)
	require.Contains(t, states, "a")
	assert.Equal(t, draining, states["a"].Stage)
	assert.Empty(t, states["a"].Message)

	second.End("")
}

func TestRebootKeepsOperationUntilBooted(t *testing.T) {
	t.Parallel()

	reporter := machineoperation.NewReporter()

	operation := reporter.Start("a", rebooting)
	operation.End("boot-1")

	states := changed(t, reporter)
	require.Contains(t, states, "a")
	assert.Equal(t, rebooting, states["a"].Stage)
	assert.Equal(t, "boot-1", states["a"].BootID)

	// nothing more can be reported on it
	operation.SetStage(draining)
	operation.End("")

	// the machine did not reboot yet
	reporter.Booted("a", "boot-1")
	reporter.Booted("b", "boot-9")

	unchanged(t, reporter)

	reporter.Booted("a", "boot-2")

	assert.Empty(t, changed(t, reporter))
}

func TestBootedDoesNotEndRunningOperation(t *testing.T) {
	t.Parallel()

	reporter := machineoperation.NewReporter()

	operation := reporter.Start("a", pulling)

	changed(t, reporter)

	reporter.Booted("a", "boot-2")
	reporter.Booted("a", "")

	unchanged(t, reporter)
	assert.Contains(t, reporter.Snapshot(), "a")

	operation.End("")
}

func TestNilReporter(t *testing.T) {
	t.Parallel()

	var reporter *machineoperation.Reporter

	operation := reporter.Start("a", pulling)
	operation.SetStage(draining)
	operation.SetMessage("message")
	operation.End("boot-1")

	reporter.Booted("a", "boot-2")

	assert.Nil(t, reporter.Changes())
	assert.Empty(t, reporter.Snapshot())
}
