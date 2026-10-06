// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package configtry_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configtry"
)

const (
	sha      = "aaaa"
	otherSha = "bbbb"
)

var base = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

func inFlight(configSha string, attempts uint32, startedAt time.Time) *specs.ConfigTryStatus {
	return &specs.ConfigTryStatus{
		Sha256:    configSha,
		Attempts:  attempts,
		StartedAt: timestamppb.New(startedAt),
		BootId:    "boot-1",
	}
}

func notInFlight(configSha string, attempts uint32) *specs.ConfigTryStatus {
	return &specs.ConfigTryStatus{Sha256: configSha, Attempts: attempts}
}

func TestDefaultTimingsFit(t *testing.T) {
	t.Parallel()

	timings := configtry.Default

	assert.Less(t, timings.ConfirmAfter, timings.ConfirmDeadline)
	assert.Less(t, timings.ConfirmDeadline, timings.Timeout)
	assert.LessOrEqual(t, 2*timings.ConfirmRetry, timings.ConfirmDeadline-timings.ConfirmAfter, "at least two probes must fit into the confirm window")
}

func TestDecide(t *testing.T) {
	t.Parallel()

	timings := configtry.Default
	rollbackDone := timings.Timeout + timings.RollbackGrace

	for _, tt := range []struct {
		now        time.Time
		status     *specs.ConfigTryStatus
		name       string
		wantAction configtry.Action
		wantDelay  time.Duration
	}{
		{
			name:       "no status",
			status:     nil,
			now:        base,
			wantAction: configtry.Try,
		},
		{
			name:       "status for another config, nothing in flight",
			status:     notInFlight(otherSha, timings.MaxAttempts),
			now:        base,
			wantAction: configtry.Try,
		},
		{
			name:       "attempts left, nothing in flight",
			status:     notInFlight(sha, timings.MaxAttempts-1),
			now:        base,
			wantAction: configtry.Try,
		},
		{
			name:       "attempts spent, nothing in flight",
			status:     notInFlight(sha, timings.MaxAttempts),
			now:        base,
			wantAction: configtry.Stop,
		},
		{
			name:       "just pushed",
			status:     inFlight(sha, 1, base),
			now:        base,
			wantAction: configtry.Wait,
			wantDelay:  timings.ConfirmAfter,
		},
		{
			name:       "confirm window opens",
			status:     inFlight(sha, 1, base),
			now:        base.Add(timings.ConfirmAfter),
			wantAction: configtry.Confirm,
		},
		{
			name:       "just before the confirm deadline",
			status:     inFlight(sha, 1, base),
			now:        base.Add(timings.ConfirmDeadline - time.Nanosecond),
			wantAction: configtry.Confirm,
		},
		{
			name:       "confirm deadline closes the window",
			status:     inFlight(sha, 1, base),
			now:        base.Add(timings.ConfirmDeadline),
			wantAction: configtry.Wait,
			wantDelay:  rollbackDone - timings.ConfirmDeadline,
		},
		{
			name:       "desired config changed before the window",
			status:     inFlight(otherSha, 1, base),
			now:        base,
			wantAction: configtry.Try,
		},
		{
			name:       "desired config changed inside the window",
			status:     inFlight(otherSha, 1, base),
			now:        base.Add(timings.ConfirmAfter),
			wantAction: configtry.Try,
		},
		{
			name:       "desired config changed after the confirm deadline",
			status:     inFlight(otherSha, timings.MaxAttempts, base),
			now:        base.Add(timings.ConfirmDeadline),
			wantAction: configtry.Try,
		},
		{
			name:       "rolled back, attempts left",
			status:     inFlight(sha, timings.MaxAttempts-1, base),
			now:        base.Add(rollbackDone),
			wantAction: configtry.Try,
		},
		{
			name:       "rolled back, attempts spent",
			status:     inFlight(sha, timings.MaxAttempts, base),
			now:        base.Add(rollbackDone),
			wantAction: configtry.Stop,
		},
		{
			name:       "long past the rollback, for a config nobody wants anymore",
			status:     inFlight(otherSha, timings.MaxAttempts, base),
			now:        base.Add(time.Hour),
			wantAction: configtry.Try,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			action, delay := timings.Decide(tt.status, sha, tt.now)

			assert.Equal(t, tt.wantAction, action)
			assert.Equal(t, tt.wantDelay, delay)
		})
	}
}

// TestDecideStopsAfterMaxAttempts walks a config through every attempt, the way the controllers do.
func TestDecideStopsAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	timings := configtry.Default
	now := base

	var status *specs.ConfigTryStatus

	for attempt := uint32(1); attempt <= timings.MaxAttempts; attempt++ {
		action, _ := timings.Decide(status, sha, now)
		require.Equal(t, configtry.Try, action)

		status = configtry.Begin(status, sha, "boot-1", now)
		require.Equal(t, attempt, status.GetAttempts())

		now = now.Add(timings.Timeout + timings.RollbackGrace)
	}

	action, _ := timings.Decide(status, sha, now)
	assert.Equal(t, configtry.Stop, action)

	// a new config gets a fresh budget
	action, _ = timings.Decide(status, otherSha, now)
	assert.Equal(t, configtry.Try, action)
	assert.Equal(t, uint32(1), configtry.Begin(status, otherSha, "boot-1", now).GetAttempts())
}

func TestInFlight(t *testing.T) {
	t.Parallel()

	timings := configtry.Default
	rollbackDone := timings.Timeout + timings.RollbackGrace

	assert.False(t, timings.InFlight(nil, base))
	assert.False(t, timings.InFlight(notInFlight(sha, 1), base))
	assert.True(t, timings.InFlight(inFlight(sha, 1, base), base))
	assert.True(t, timings.InFlight(inFlight(sha, 1, base), base.Add(rollbackDone-time.Nanosecond)))
	assert.False(t, timings.InFlight(inFlight(sha, 1, base), base.Add(rollbackDone)))
}
