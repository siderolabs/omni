// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/siderolabs/omni/client/pkg/constants"
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
	Role *Role
	// KubernetesGroups are the Kubernetes groups which the grant gives on the clusters in its scope.
	// They count only when the role allows the kubernetes-access operation.
	KubernetesGroups []string
	Scope            Scope
}

// PresetGrants returns the grants a preset stands for: each of its roles, everywhere.
func PresetGrants(preset Preset) []Grant {
	roles := PresetRoles(preset)
	grants := make([]Grant, 0, len(roles))

	for _, r := range roles {
		grant := Grant{Role: r, Scope: Scope{Global: true}}

		// a preset which allows access to the Kubernetes API gives full access to it
		if r.allowsOperation(KubernetesAccess) {
			grant.KubernetesGroups = []string{constants.DefaultAccessGroup}
		}

		grants = append(grants, grant)
	}

	return grants
}

// KubernetesGroups returns the Kubernetes groups which the grants give on the cluster, sorted and without duplicates.
func KubernetesGroups(grants []Grant, cluster string) []string {
	var groups []string

	for _, grant := range grants {
		if grant.Scope.covers(Target{Cluster: cluster}) && grant.Role.allowsOperation(KubernetesAccess) {
			groups = append(groups, grant.KubernetesGroups...)
		}
	}

	slices.Sort(groups)

	return slices.Compact(groups)
}
