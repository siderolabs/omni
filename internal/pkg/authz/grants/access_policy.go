// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grants

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/internal/pkg/authz"
)

// groupPrefix marks a reference to a user group or a cluster group in a rule of an access policy.
const groupPrefix = "group/"

// policyRoles are the roles which a rule of an access policy can give. They are the roles about clusters:
// a rule gives no access to machines or to the instance.
var policyRoles = []*authz.Role{authz.ClusterViewer, authz.ClusterManager, authz.WorkloadProxyUser, authz.KubernetesUser}

// FromAccessPolicy returns what the access policy grants to the identity.
//
// Every rule which matches the identity becomes grants on the clusters of the rule:
//   - The preset in the rule (Reader, Operator or Admin) gives those of its roles which are in policyRoles.
//   - The Kubernetes groups in the rule become a kubernetes-user grant with these groups.
func FromAccessPolicy(policy *specs.AccessPolicySpec, identity *resource.Metadata) ([]authz.Grant, error) {
	var grants []authz.Grant

	for _, rule := range policy.GetRules() {
		matches, err := ruleMatchesIdentity(policy, rule, identity)
		if err != nil {
			return nil, err
		}

		if !matches {
			continue
		}

		scope := authz.Scope{Clusters: clusterPatterns(policy, rule)}

		if rule.GetRole() != "" {
			preset, err := role.Parse(rule.GetRole())
			if err != nil {
				return nil, err
			}

			for _, grant := range authz.PresetGrants(preset) {
				if slices.Contains(policyRoles, grant.Role) {
					grant.Scope = scope

					grants = append(grants, grant)
				}
			}
		}

		if kubernetesGroups := rule.GetKubernetes().GetImpersonate().GetGroups(); len(kubernetesGroups) > 0 {
			grants = append(grants, authz.Grant{Role: authz.KubernetesUser, Scope: scope, KubernetesGroups: kubernetesGroups})
		}
	}

	return grants, nil
}

// ruleMatchesIdentity reports whether a rule of an access policy is for the identity:
// the identity is listed in the rule, or it is in one of the user groups listed in the rule.
func ruleMatchesIdentity(policy *specs.AccessPolicySpec, rule *specs.AccessPolicyRule, identity *resource.Metadata) (bool, error) {
	for _, user := range rule.GetUsers() {
		if user == identity.ID() {
			return true, nil
		}

		groupName, isGroup := strings.CutPrefix(user, groupPrefix)
		if !isGroup {
			continue
		}

		for _, groupUser := range policy.GetUserGroups()[groupName].GetUsers() {
			matches, err := userMatchesIdentity(groupUser, identity)
			if err != nil {
				return false, err
			}

			if matches {
				return true, nil
			}
		}
	}

	return false, nil
}

// userMatchesIdentity reports whether an entry of a user group is for the identity. An entry is for it
// by its name, by a pattern on its name, or by label selectors on its labels.
func userMatchesIdentity(user *specs.AccessPolicyUserGroup_User, identity *resource.Metadata) (bool, error) {
	if user.GetName() != "" && user.GetName() == identity.ID() {
		return true, nil
	}

	if user.GetMatch() != "" {
		matches, err := filepath.Match(user.GetMatch(), identity.ID())
		if err != nil {
			return false, fmt.Errorf("invalid match pattern %q for %s", user.GetMatch(), identity)
		}

		if matches {
			return true, nil
		}
	}

	if len(user.GetLabelSelectors()) == 0 {
		return false, nil
	}

	query, err := labels.ParseSelectors([]string{strings.Join(user.GetLabelSelectors(), ",")})
	if err != nil {
		return false, err
	}

	return query.Matches(*identity.Labels()), nil
}

// clusterPatterns returns the clusters of the rule as patterns.
//
// A cluster named in the rule itself or by its name in a cluster group is an exact ID, only the match of a cluster group is a pattern.
func clusterPatterns(policy *specs.AccessPolicySpec, rule *specs.AccessPolicyRule) []string {
	var patterns []string

	for _, cluster := range rule.GetClusters() {
		groupName, isGroup := strings.CutPrefix(cluster, groupPrefix)
		if !isGroup {
			patterns = append(patterns, authz.ExactCluster(cluster))

			continue
		}

		for _, groupCluster := range policy.GetClusterGroups()[groupName].GetClusters() {
			if groupCluster.GetName() != "" {
				patterns = append(patterns, authz.ExactCluster(groupCluster.GetName()))
			}

			if groupCluster.GetMatch() != "" {
				patterns = append(patterns, groupCluster.GetMatch())
			}
		}
	}

	return patterns
}
