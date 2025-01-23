// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import "sync"

// ClusterCleanuper is an interface for cleaning up clusters when they are being removed.
type ClusterCleanuper interface {
	ClusterRemove(clusterName string)
}

// cleanuperTracker tracks registered ClusterCleanupers.
type cleanuperTracker struct {
	cleanupers   []ClusterCleanuper
	cleanupersMu sync.Mutex
}

// RegisterCleanuper adds a cleanup handler to the list so that it gets called when cluster is removed.
func (cleanuper *cleanuperTracker) RegisterCleanuper(c ClusterCleanuper) {
	cleanuper.cleanupersMu.Lock()
	defer cleanuper.cleanupersMu.Unlock()

	cleanuper.cleanupers = append(cleanuper.cleanupers, c)
}

func (cleanuper *cleanuperTracker) triggerCleanupers(cluster string) {
	cleanuper.cleanupersMu.Lock()
	defer cleanuper.cleanupersMu.Unlock()

	for _, c := range cleanuper.cleanupers {
		c.ClusterRemove(cluster)
	}
}
