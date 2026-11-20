// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machineoperation keeps the progress of the operations Omni runs on the machines, e.g., installs and upgrades.
package machineoperation

import (
	"sync"
	"time"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

// Stage is a step of an operation.
type Stage = specs.MachineOperationStatusSpec_Stage

// State represents the last reported state of an operation.
type State struct {
	StartedAt time.Time
	Message   string
	// BootID is set when the operation waits for the machine to reboot: it is the boot ID from before the reboot.
	BootID string
	Stage  Stage
}

// Reporter keeps the last reported state of the operation running on each machine.
//
// Reporting never blocks and never fails, so it cannot delay or break the operation.
//
// A nil Reporter is valid and ignores everything reported to it.
type Reporter struct {
	operations map[string]*Operation
	changes    chan struct{}
	mu         sync.Mutex
}

// NewReporter creates a Reporter.
func NewReporter() *Reporter {
	return &Reporter{
		operations: map[string]*Operation{},
		changes:    make(chan struct{}, 1),
	}
}

// Operation represents a running operation on a machine. A nil Operation is valid and reports nothing.
type Operation struct {
	reporter  *Reporter
	machineID string
	state     State
}

// Start starts reporting an operation on the machine. It replaces the operation of the same machine, if there is one.
func (r *Reporter) Start(machineID string, stage Stage) *Operation {
	return r.start(machineID, stage, time.Now(), true)
}

// StartAtIfIdle starts reporting an operation which began at startedAt. It returns nil and leaves the machine alone when another operation runs on it.
func (r *Reporter) StartAtIfIdle(machineID string, stage Stage, startedAt time.Time) *Operation {
	return r.start(machineID, stage, startedAt, false)
}

func (r *Reporter) start(machineID string, stage Stage, startedAt time.Time, replace bool) *Operation {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, running := r.operations[machineID]; running && !replace {
		return nil
	}

	operation := &Operation{
		reporter:  r,
		machineID: machineID,
		state:     State{Stage: stage, StartedAt: startedAt},
	}

	r.operations[machineID] = operation
	r.signal()

	return operation
}

// SetStage reports that the operation entered the stage.
func (o *Operation) SetStage(stage Stage) {
	o.update(func() {
		o.state = State{Stage: stage, StartedAt: time.Now()}
	})
}

// SetMessage reports the progress of the current stage.
func (o *Operation) SetMessage(message string) {
	o.update(func() {
		o.state.Message = message
	})
}

// Running reports whether the operation has neither ended nor been replaced.
func (o *Operation) Running() bool {
	if o == nil {
		return false
	}

	r := o.reporter

	r.mu.Lock()
	defer r.mu.Unlock()

	return r.operations[o.machineID] == o
}

// End ends the operation.
//
// If rebootBootID is set, the operation stays in the rebooting stage until Booted is called with another boot ID.
func (o *Operation) End(rebootBootID string) {
	if o == nil {
		return
	}

	r := o.reporter

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.operations[o.machineID] != o || o.state.BootID != "" {
		return
	}

	if rebootBootID == "" {
		delete(r.operations, o.machineID)
	} else {
		o.state = State{
			Stage:     specs.MachineOperationStatusSpec_REBOOTING,
			Message:   o.state.Message,
			StartedAt: o.state.StartedAt,
			BootID:    rebootBootID,
		}
	}

	r.signal()
}

// update changes the state, unless the operation is ended or replaced.
func (o *Operation) update(change func()) {
	if o == nil {
		return
	}

	r := o.reporter

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.operations[o.machineID] != o || o.state.BootID != "" {
		return
	}

	change()

	r.signal()
}

// Booted ends the operation of the machine if it waits for a reboot and bootID is not the one from before the reboot.
func (r *Reporter) Booted(machineID, bootID string) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	operation, ok := r.operations[machineID]
	if !ok || operation.state.BootID == "" || operation.state.BootID == bootID {
		return
	}

	delete(r.operations, machineID)
	r.signal()
}

// Changes returns a channel which is notified when an operation changes.
func (r *Reporter) Changes() <-chan struct{} {
	if r == nil {
		return nil
	}

	return r.changes
}

// Snapshot returns the state of every operation by machine ID.
func (r *Reporter) Snapshot() map[string]State {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	result := make(map[string]State, len(r.operations))

	for machineID, operation := range r.operations {
		result[machineID] = operation.state
	}

	return result
}

func (r *Reporter) signal() {
	select {
	case r.changes <- struct{}{}:
	default:
	}
}
