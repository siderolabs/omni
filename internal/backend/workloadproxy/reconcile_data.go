// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"iter"
	"maps"

	"github.com/siderolabs/gen/xiter"
)

// ReconcileData is the data structure used to reconcile the load balancer for a specific cluster.
type ReconcileData struct {
	AliasPort map[string]string
	Hosts     []string
}

// GetHosts returns the hosts for the specific cluster.
func (d *ReconcileData) GetHosts() []string {
	if d == nil {
		return nil
	}

	return d.Hosts
}

// AliasesData returns the aliases the specific cluster.
func (d *ReconcileData) AliasesData() iter.Seq[string] {
	if d == nil {
		return xiter.Empty
	}

	return maps.Keys(d.AliasPort)
}

// PortForAlias returns the port for the given alias.
func (d *ReconcileData) PortForAlias(als string) string {
	if d == nil {
		return ""
	}

	return d.AliasPort[als]
}
