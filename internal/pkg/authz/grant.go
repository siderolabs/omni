// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz

import (
	"path/filepath"
	"slices"
	"strings"
)

// AllClustersPattern is the only cluster pattern which covers the requests not limited to a single cluster.
const AllClustersPattern = "*"

// Scope is where a grant applies: either everywhere, or on some clusters.
//
// Only one of the two is set. When Global is set, Clusters is not looked at.
type Scope struct {
	// Clusters are the patterns of the cluster IDs, in the syntax of filepath.Match.
	Clusters []string
	// Global is set for the grants which apply everywhere: to every cluster, to the machines in no cluster and to the resources of the instance.
	Global bool
}

// ExactCluster returns the pattern which matches only the given cluster ID.
//
// A scope holds patterns, but some sources give a plain cluster ID, e.g., a cluster listed in a rule of an access policy.
// Such an ID is escaped here, so that a character like "*" in it is not treated as a part of a pattern.
func ExactCluster(id string) string {
	return strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`).Replace(id)
}

// covers reports whether the scope includes the target.
//
// A request which is not limited to a single cluster is covered only by the pattern which matches all clusters,
// and a target which belongs to no cluster only by a global scope.
func (s Scope) covers(target Target) bool {
	switch {
	case s.Global:
		return true
	case target.AllClusters:
		return slices.Contains(s.Clusters, AllClustersPattern)
	case target.Cluster == "":
		return false
	}

	return slices.ContainsFunc(s.Clusters, func(pattern string) bool {
		matches, err := filepath.Match(pattern, target.Cluster)

		return err == nil && matches
	})
}

// Grant is a role given to a caller in a scope.
type Grant struct {
	Role  *Role
	Scope Scope
}

// PresetGrants returns the grants a preset stands for: each of its roles, everywhere.
func PresetGrants(preset Preset) []Grant {
	roles := PresetRoles(preset)
	grants := make([]Grant, 0, len(roles))

	for _, r := range roles {
		grants = append(grants, Grant{Role: r, Scope: Scope{Global: true}})
	}

	return grants
}
