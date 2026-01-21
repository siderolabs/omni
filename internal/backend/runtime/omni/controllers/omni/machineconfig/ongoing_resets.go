// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
)

type resetStatus struct {
	resetAttempts            uint
	etcdLeaveAttempts        uint
	maintenanceCheckAttempts uint
}

type ongoingResets struct {
	statuses map[resource.ID]*resetStatus
	mu       sync.Mutex
}

func (r *ongoingResets) getStatus(id resource.ID) (*resetStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rs, ok := r.statuses[id]

	return rs, ok
}

func (r *ongoingResets) isGraceful(id resource.ID) bool {
	rs, ok := r.getStatus(id)
	if !ok {
		return true
	}

	return rs.resetAttempts < gracefulResetAttemptCount
}

func (r *ongoingResets) shouldLeaveEtcd(id string) bool {
	rs, ok := r.getStatus(id)
	if !ok {
		return true
	}

	return rs.etcdLeaveAttempts < etcdLeaveAttemptsLimit
}

func (r *ongoingResets) handleReset(id resource.ID) uint {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[id]; !ok {
		r.statuses[id] = &resetStatus{}
	}

	r.statuses[id].resetAttempts++

	return r.statuses[id].resetAttempts
}

func (r *ongoingResets) handleMaintenanceCheck(id resource.ID) uint {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[id]; !ok {
		r.statuses[id] = &resetStatus{}
	}

	r.statuses[id].maintenanceCheckAttempts++

	return r.statuses[id].maintenanceCheckAttempts
}

func (r *ongoingResets) handleEtcdLeave(id resource.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[id]; !ok {
		r.statuses[id] = &resetStatus{}
	}

	r.statuses[id].etcdLeaveAttempts++
}

func (r *ongoingResets) deleteStatus(id resource.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.statuses, id)
}
