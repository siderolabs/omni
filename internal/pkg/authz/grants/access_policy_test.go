// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grants_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access/role"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/authz"
	"github.com/siderolabs/omni/internal/pkg/authz/grants"
)

// TestFromAccessPolicyMatchesCheck checks that the grants made from an access policy give the same access to a cluster
// as the role accesspolicy.Check returns for it.
func TestFromAccessPolicyMatchesCheck(t *testing.T) {
	t.Parallel()

	policy := authres.NewAccessPolicy()
	policy.TypedSpec().Value = &specs.AccessPolicySpec{
		UserGroups: map[string]*specs.AccessPolicyUserGroup{
			"named":    {Users: []*specs.AccessPolicyUserGroup_User{{Name: "named@example.org"}}},
			"matched":  {Users: []*specs.AccessPolicyUserGroup_User{{Match: "dev-*@example.org"}}},
			"labeled":  {Users: []*specs.AccessPolicyUserGroup_User{{LabelSelectors: []string{"team=a", "!contractor"}}}},
			"everyone": {Users: []*specs.AccessPolicyUserGroup_User{{Match: "*"}}},
		},
		ClusterGroups: map[string]*specs.AccessPolicyClusterGroup{
			"named":   {Clusters: []*specs.AccessPolicyClusterGroup_Cluster{{Name: "prod-1"}, {Name: "prod-*"}}},
			"matched": {Clusters: []*specs.AccessPolicyClusterGroup_Cluster{{Match: "staging-?"}}},
			"all":     {Clusters: []*specs.AccessPolicyClusterGroup_Cluster{{Match: "*"}}},
		},
		Rules: []*specs.AccessPolicyRule{
			{Users: []string{"direct@example.org"}, Clusters: []string{"prod-1", "*"}, Role: string(role.Operator)},
			{Users: []string{"group/named"}, Clusters: []string{"group/named"}, Role: string(role.Reader)},
			{Users: []string{"group/matched"}, Clusters: []string{"group/matched"}, Role: string(role.Admin)},
			{Users: []string{"group/labeled"}, Clusters: []string{"group/all"}, Role: string(role.Reader)},
			{Users: []string{"group/labeled"}, Clusters: []string{"prod-2"}, Role: string(role.Operator)},
			{Users: []string{"group/everyone"}, Clusters: []string{"shared"}, Role: string(role.None)},
			{Users: []string{"group/everyone"}, Clusters: []string{"dev"}},
		},
	}

	require.NoError(t, accesspolicy.Validate(policy))

	identities := map[string]map[string]string{
		"direct@example.org":     nil,
		"named@example.org":      nil,
		"dev-1@example.org":      nil,
		"labeled@example.org":    {"team": "a"},
		"contractor@example.org": {"team": "a", "contractor": ""},
		"nobody@example.org":     nil,
	}

	clusters := []string{"prod-1", "prod-2", "prod-*", "*", "staging-1", "staging-10", "shared", "dev", "other"}

	for id, labels := range identities {
		identity := authres.NewIdentity(id)

		for k, v := range labels {
			identity.Metadata().Labels().Set(k, v)
		}

		policyGrants, err := grants.FromAccessPolicy(policy.TypedSpec().Value, identity.Metadata())
		require.NoError(t, err)

		subject := authz.Subject{Authenticated: true, Grants: policyGrants}

		for _, cluster := range clusters {
			result, checkErr := accesspolicy.Check(policy, omni.NewCluster(cluster).Metadata(), identity.Metadata())
			require.NoError(t, checkErr)

			canRead := authz.AllowResource(subject, authz.Target{Type: omni.ClusterType, Verb: state.Get, Cluster: cluster}) == nil
			canWrite := authz.AllowResource(subject, authz.Target{Type: omni.ClusterType, Verb: state.Update, Cluster: cluster}) == nil

			assert.Equal(t, result.Role.Check(role.Reader) == nil, canRead, "%s reads %s, the policy gives %s", id, cluster, result.Role)
			assert.Equal(t, result.Role.Check(role.Operator) == nil, canWrite, "%s changes %s, the policy gives %s", id, cluster, result.Role)
		}

		// only the match of a cluster group with the value "*" stands for all clusters
		result, err := accesspolicy.Check(policy, omni.NewCluster("any").Metadata(), identity.Metadata())
		require.NoError(t, err)

		canListAll := authz.AllowResource(subject, authz.Target{Type: omni.ClusterType, Verb: state.List, AllClusters: true}) == nil

		assert.Equal(t, result.MatchesAllClusters && result.Role.Check(role.Reader) == nil, canListAll, "%s lists all clusters", id)
	}
}
