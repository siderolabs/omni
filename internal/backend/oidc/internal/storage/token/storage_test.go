// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//nolint:goconst
package token_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage/token"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

func TestValidateJWTProfileScopes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	s := token.NewStorage(st, clock.NewMock())

	scopes, err := s.ValidateJWTProfileScopes(ctx, "", []string{
		oidc.ScopeOpenID,
		external.ScopeClusterPrefix + "talos-default",
		"other-scope",
	})
	require.NoError(t, err)

	assert.Equal(t, []string{
		oidc.ScopeOpenID,
		external.ScopeClusterPrefix + "talos-default",
	}, scopes)
}

func TestGetPrivateClaimsFromScopes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	userIdentity := "test-user"
	userID := "test-user-id"
	clusterID := "talos-default"
	group := "foobar"

	accessPolicy := auth.NewAccessPolicy()

	accessPolicy.TypedSpec().Value.Rules = []*specs.AccessPolicyRule{
		{
			Users:    []string{userIdentity},
			Clusters: []string{clusterID},
			Kubernetes: &specs.AccessPolicyRule_Kubernetes{
				Impersonate: &specs.AccessPolicyRule_Kubernetes_Impersonate{
					Groups: []string{group},
				},
			},
		},
	}

	identity := auth.NewIdentity(resources.DefaultNamespace, userIdentity)

	identity.TypedSpec().Value.UserId = userID

	user := auth.NewUser(resources.DefaultNamespace, userID)
	user.TypedSpec().Value.Role = string(role.None)

	cluster := omni.NewCluster(resources.DefaultNamespace, clusterID)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, st.Create(ctx, accessPolicy))
	require.NoError(t, st.Create(ctx, identity))
	require.NoError(t, st.Create(ctx, user))
	require.NoError(t, st.Create(ctx, cluster))

	s := token.NewStorage(st, clock.NewMock())

	claims, err := s.GetPrivateClaimsFromScopes(ctx, userIdentity, "", []string{
		oidc.ScopeOpenID,
		external.ScopeClusterPrefix + clusterID,
		"other-scope",
	})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"cluster": clusterID,
		"groups":  []string{group},
	}, claims)
}

func TestSetUserinfoFromScopes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	userIdentity := "test@example.com"
	userID := "test-user-id"
	clusterID := "talos-default"

	accessPolicy := auth.NewAccessPolicy()

	accessPolicy.TypedSpec().Value.UserGroups = map[string]*specs.AccessPolicyUserGroup{
		"some-other-user-group": {
			Users: []*specs.AccessPolicyUserGroup_User{
				{Name: "some-other-user"},
			},
		},
		"some-user-group": {
			Users: []*specs.AccessPolicyUserGroup_User{
				{Name: "doesntmatter1"},
				{Name: userIdentity},
				{Name: "doesntmatter2"},
			},
		},
	}

	accessPolicy.TypedSpec().Value.ClusterGroups = map[string]*specs.AccessPolicyClusterGroup{
		"some-other-cluster-group": {
			Clusters: []*specs.AccessPolicyClusterGroup_Cluster{
				{Name: "some-other-cluster-1"},
				{Name: "some-other-cluster-2"},
			},
		},
		"some-cluster-group": {
			Clusters: []*specs.AccessPolicyClusterGroup_Cluster{
				{Name: "doesntmatter1"},
				{Name: clusterID},
				{Name: "doesntmatter2"},
			},
		},
	}

	accessPolicy.TypedSpec().Value.Rules = []*specs.AccessPolicyRule{
		{
			Users:    []string{"some-user", "group/some-user-group", "some-other-user"},
			Clusters: []string{"some-cluster", "group/some-cluster-group", "some-other-cluster"},
			Kubernetes: &specs.AccessPolicyRule_Kubernetes{
				Impersonate: &specs.AccessPolicyRule_Kubernetes_Impersonate{
					Groups: []string{"group-1", "group-2"},
				},
			},
		},
		{
			Users:    []string{userIdentity},
			Clusters: []string{clusterID},
			Kubernetes: &specs.AccessPolicyRule_Kubernetes{
				Impersonate: &specs.AccessPolicyRule_Kubernetes_Impersonate{
					Groups: []string{"group-3"},
				},
			},
		},
		{
			Users:    []string{userIdentity},
			Clusters: []string{"some-other-cluster"},
			Kubernetes: &specs.AccessPolicyRule_Kubernetes{
				Impersonate: &specs.AccessPolicyRule_Kubernetes_Impersonate{
					Groups: []string{"group-4"},
				},
			},
		},
	}

	identity := auth.NewIdentity(resources.DefaultNamespace, userIdentity)

	identity.TypedSpec().Value.UserId = userID

	user := auth.NewUser(resources.DefaultNamespace, userID)

	// will bring constants.DefaultAccessGroup scope
	user.TypedSpec().Value.Role = string(role.Operator)

	cluster := omni.NewCluster(resources.DefaultNamespace, clusterID)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, st.Create(ctx, accessPolicy))
	require.NoError(t, st.Create(ctx, identity))
	require.NoError(t, st.Create(ctx, user))
	require.NoError(t, st.Create(ctx, cluster))

	s := token.NewStorage(st, clock.NewMock())

	ui := &oidc.UserInfo{}

	err := s.SetUserinfoFromScopes(ctx, ui, "test@example.com", "", []string{
		oidc.ScopeOpenID,
		external.ScopeClusterPrefix + "talos-default",
		"other-scope",
	})
	require.NoError(t, err)

	assert.Equal(t, userIdentity, ui.Subject)

	actualCluster, ok := ui.Claims["cluster"]
	require.True(t, ok)
	assert.Equal(t, clusterID, actualCluster)

	actualGroups, ok := ui.Claims["groups"]
	require.True(t, ok)

	actualGroupsSlc, ok := actualGroups.([]string)
	require.True(t, ok)

	assert.ElementsMatch(t, []string{constants.DefaultAccessGroup, "group-1", "group-2", "group-3"}, actualGroupsSlc)
}

func TestTokenIntrospection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	req := mockTokenRequest{}

	userIdentity := req.GetSubject()
	userID := "test-user-id"

	identity := auth.NewIdentity(resources.DefaultNamespace, userIdentity)

	identity.TypedSpec().Value.UserId = userID

	user := auth.NewUser(resources.DefaultNamespace, userID)

	// will bring constants.DefaultAccessGroup scope
	user.TypedSpec().Value.Role = string(role.Operator)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, st.Create(ctx, identity))
	require.NoError(t, st.Create(ctx, user))

	clck := clock.NewMock()
	s := token.NewStorage(st, clck)

	// create token and try fetching information from it
	tokenID, expiration, err := s.CreateAccessToken(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, clck.Now().Add(token.Lifetime), expiration)
	assert.NotEmpty(t, tokenID)

	ui := &oidc.UserInfo{}

	err = s.SetUserinfoFromToken(ctx, ui, tokenID, userIdentity, "")
	require.NoError(t, err)

	assert.Equal(t, "some@example.com", ui.Subject)
	assert.Equal(t, map[string]any{
		"cluster": "cluster1",
		"groups":  []string{constants.DefaultAccessGroup},
	}, ui.Claims)

	// advance time so that token expires
	clck.Add(token.Lifetime + time.Second)

	err = s.SetUserinfoFromToken(ctx, ui, tokenID, "", "")
	assert.Error(t, err)

	// invalid ID
	err = s.SetUserinfoFromToken(ctx, ui, "invalid", "", "")
	assert.Error(t, err)
}

func TestRevokeToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	clock := clock.NewMock()
	s := token.NewStorage(st, clock)

	tokenID, _, err := s.CreateAccessToken(ctx, mockTokenRequest{})
	require.NoError(t, err)

	err = s.RevokeToken(ctx, tokenID, mockTokenRequest{}.GetSubject(), "")
	require.Nil(t, err)

	err = s.SetUserinfoFromToken(ctx, nil, tokenID, "", "")
	assert.Error(t, err)
}

func TestTerminateSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	clock := clock.NewMock()
	s := token.NewStorage(st, clock)

	tokenID, _, err := s.CreateAccessToken(ctx, mockTokenRequest{})
	require.NoError(t, err)

	err = s.TerminateSession(ctx, mockTokenRequest{}.GetSubject(), "")
	require.Nil(t, err)

	err = s.SetUserinfoFromToken(ctx, nil, tokenID, "", "")
	assert.Error(t, err)
}
