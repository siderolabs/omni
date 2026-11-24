// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package configtry decides when to push, confirm or give up on a machine config applied in Talos try mode.
//
// Talos keeps a config applied in try mode active without persisting it, and puts the previous config
// back after a timeout unless a regular apply confirms the new one first. A config that costs Omni its
// access to the machine is therefore undone by the machine itself.
//
// A try applied while another one is pending keeps the pending rollback target, so a new config is tried
// without waiting for the pending try. Talos releases without that behavior roll back to the pending
// try config instead, which a reboot undoes.
package configtry

import (
	"fmt"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/machineoperation"
)

// MinTalosVersion is the first Talos version that applies every config change without a reboot.
// Before it, a try apply of a change that needs a reboot is rejected, while a plain apply reboots the machine.
var MinTalosVersion = semver.MustParse("1.14.0")

// Timings holds the durations the try mode decisions are based on.
type Timings struct {
	// Timeout is sent to Talos as the try mode timeout. Talos rolls the config back this long after the try apply.
	Timeout time.Duration

	// ConfirmAfter is how long the config has to stay on the machine before Omni confirms it.
	ConfirmAfter time.Duration

	// ConfirmDeadline is the last point at which Omni confirms. Later than this, the confirming apply might lose the race against the rollback.
	ConfirmDeadline time.Duration

	// RollbackGrace is how much longer than Timeout to wait before the machine is considered to be back on its previous config.
	RollbackGrace time.Duration

	// ConfirmRetry is how long to wait before probing the machine again after a failed probe inside the confirm window.
	ConfirmRetry time.Duration

	// MaxAttempts caps the try applies spent on one config before Omni gives up on it.
	MaxAttempts uint32
}

// Default is the production timing set.
//
// The confirm window (ConfirmAfter to ConfirmDeadline) covers the SideroLink keepalive cycle (25
// seconds): a config that changes the machine's underlay address keeps Omni's packets going to the
// old address until the machine sends something from the new one. The ten seconds between
// ConfirmDeadline and Timeout are the margin for the confirming apply to reach the machine before
// the rollback.
var Default = Timings{
	Timeout:         45 * time.Second,
	ConfirmAfter:    10 * time.Second,
	ConfirmDeadline: 35 * time.Second,
	RollbackGrace:   5 * time.Second,
	ConfirmRetry:    5 * time.Second,
	MaxAttempts:     2,
}

// Action is what the caller should do next about the config it wants on the machine.
type Action int

const (
	// Try means push the config in try mode.
	Try Action = iota

	// Wait means a try of the desired config is in flight. It is either too young to confirm, or past the
	// confirm deadline, in which case the machine has to roll back before the config is tried again.
	Wait

	// Confirm means the try has stayed on the machine long enough: check the machine is still there and confirm.
	Confirm

	// Stop means MaxAttempts tries were spent on this config without confirming it: record the failure
	// and leave the machine alone until the config changes.
	Stop
)

// Decide returns what to do about desiredSha256 given the recorded try state.
//
// The returned delay is only set for Wait.
func (t Timings) Decide(status *specs.ConfigTryStatus, desiredSha256 string, now time.Time) (Action, time.Duration) {
	if startedAt := status.GetStartedAt(); startedAt != nil && status.GetSha256() == desiredSha256 {
		elapsed := now.Sub(startedAt.AsTime())

		switch {
		case elapsed < t.ConfirmAfter:
			return Wait, t.ConfirmAfter - elapsed
		case elapsed < t.ConfirmDeadline:
			return Confirm, 0
		case now.Before(t.RollbackEnd(status)):
			return Wait, t.RollbackEnd(status).Sub(now)
		}
	}

	if status.GetSha256() == desiredSha256 && status.GetAttempts() >= t.MaxAttempts {
		return Stop, 0
	}

	return Try, 0
}

// InFlight reports whether a try is still active on the machine or still rolling back.
func (t Timings) InFlight(status *specs.ConfigTryStatus, now time.Time) bool {
	return status.GetStartedAt() != nil && now.Before(t.RollbackEnd(status))
}

// RollbackEnd returns when the machine is considered to be back on its previous config.
func (t Timings) RollbackEnd(status *specs.ConfigTryStatus) time.Time {
	return status.GetStartedAt().AsTime().Add(t.Timeout + t.RollbackGrace)
}

// Begin records a try apply that has just been pushed.
//
// The attempt count continues when the config is the one the previous attempts were spent on, and restarts for a new config.
func Begin(status *specs.ConfigTryStatus, sha256, bootID string, now time.Time) *specs.ConfigTryStatus {
	var attempts uint32

	if status.GetSha256() == sha256 {
		attempts = status.GetAttempts()
	}

	return &specs.ConfigTryStatus{
		Sha256:    sha256,
		Attempts:  attempts + 1,
		StartedAt: timestamppb.New(now),
		BootId:    bootID,
	}
}

// Reporter reports the config try of each machine as a machine operation.
type Reporter struct {
	operations *machineoperation.Reporter
	tries      map[string]reportedTry
	timings    Timings
	mu         sync.Mutex
}

type reportedTry struct {
	operation *machineoperation.Operation
	expiry    *time.Timer
	bootID    string
	sha256    string
	attempts  uint32
}

// NewReporter creates a Reporter.
func NewReporter(operations *machineoperation.Reporter, timings Timings) *Reporter {
	return &Reporter{
		operations: operations,
		tries:      map[string]reportedTry{},
		timings:    timings,
	}
}

// Report starts, updates or ends the operation of the machine to match the recorded try state.
//
// An operation that replaced the reported one, e.g., an install, is left alone.
func (r *Reporter) Report(machineID string, status *specs.ConfigTryStatus, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	reported, ok := r.tries[machineID]

	// a try left without its sha is not counted, it is kept only to hold back the dry run until the rollback
	if !r.timings.InFlight(status, now) || status.GetSha256() == "" {
		if ok {
			r.end(machineID, reported)
		}

		return
	}

	// a try on a new boot or of a new config is a new try, and a replaced operation is gone for good
	if !ok || reported.bootID != status.GetBootId() || reported.sha256 != status.GetSha256() || !reported.operation.Running() {
		if ok {
			r.end(machineID, reported)
		}

		operation := r.operations.StartAtIfIdle(machineID, specs.MachineOperationStatusSpec_TRYING_CONFIG, status.GetStartedAt().AsTime())
		if operation == nil { // reported on a later call if the try outlives the running operation
			return
		}

		reported = reportedTry{
			operation: operation,
			bootID:    status.GetBootId(),
			sha256:    status.GetSha256(),
		}
	}

	if reported.attempts != status.GetAttempts() {
		reported.operation.SetMessage(fmt.Sprintf("attempt %d of %d", status.GetAttempts(), r.timings.MaxAttempts))
		reported.attempts = status.GetAttempts()
	}

	if reported.expiry != nil {
		reported.expiry.Stop()
	}

	// a skipped reconcile, e.g., of a disconnected machine, would otherwise leave the operation running
	var expiry *time.Timer

	expiry = time.AfterFunc(r.timings.RollbackEnd(status).Sub(now), func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		if current, ok := r.tries[machineID]; ok && current.expiry == expiry {
			r.end(machineID, current)
		}
	})

	reported.expiry = expiry
	r.tries[machineID] = reported
}

func (r *Reporter) end(machineID string, reported reportedTry) {
	reported.expiry.Stop()
	reported.operation.End("")
	delete(r.tries, machineID)
}
