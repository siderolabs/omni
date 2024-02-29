// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package scope_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/auth/scope"
)

func TestScopeCheck(t *testing.T) {
	for _, tt := range []struct { //nolint:govet
		name           string
		actorScopes    []scope.Scope
		requiredScopes []scope.Scope
		expectedError  string
	}{
		{
			name: "empty",
		},
		{
			name: "actor okay",
			actorScopes: []scope.Scope{
				scope.ClusterAny,
				scope.UserAny,
				scope.MachineAny,
			},
			requiredScopes: []scope.Scope{
				scope.New(scope.ObjectCluster, scope.ActionRead, scope.PerspectiveNone),
				scope.New(scope.ObjectUser, scope.ActionDestroy, scope.PerspectiveNone),
			},
		},
		{
			name: "missing scope",
			actorScopes: []scope.Scope{
				scope.UserAny,
				scope.MachineAny,
			},
			requiredScopes: []scope.Scope{
				scope.New(scope.ObjectCluster, scope.ActionRead, scope.PerspectiveNone),
				scope.New(scope.ObjectUser, scope.ActionDestroy, scope.PerspectiveNone),
			},
			expectedError: "missing required scope: cluster:read",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actorScopes := scope.NewScopes(tt.actorScopes...)

			err := actorScopes.Check(tt.requiredScopes...)
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseScopes(t *testing.T) {
	scopes := scope.NewScopes(scope.ClusterAny, scope.MachineAny)

	assert.Equal(t, []string{"cluster:*", "machine:*"}, scopes.Strings())

	parsedScopes, err := scope.ParseScopes(scopes.Strings())
	require.NoError(t, err)
	assert.Equal(t, scopes, parsedScopes)
}

func TestIntersection(t *testing.T) {
	scopeUserRead := scope.New(scope.ObjectUser, scope.ActionRead, scope.PerspectiveNone)
	scopeClusterRead := scope.New(scope.ObjectCluster, scope.ActionRead, scope.PerspectiveNone)
	scopeClusterCreate := scope.New(scope.ObjectCluster, scope.ActionCreate, scope.PerspectiveNone)
	scopeClusterUpgrade := scope.New(scope.ObjectCluster, scope.ActionModify, scope.PerspectiveKubernetesUpgrade)
	scopeMachineDestroy := scope.New(scope.ObjectMachine, scope.ActionDestroy, scope.PerspectiveNone)

	testCases := []struct {
		name     string
		scopes1  scope.Scopes
		scopes2  scope.Scopes
		expected scope.Scopes
	}{
		{
			name:     "with-any",
			scopes1:  scope.NewScopes(scope.ClusterAny, scope.MachineAny),
			scopes2:  scope.NewScopes(scope.ClusterAny, scope.MachineAny, scope.UserAny),
			expected: scope.NewScopes(scope.ClusterAny, scope.MachineAny),
		},
		{
			name:     "with-empty",
			scopes1:  scope.NewScopes(scopeUserRead, scope.MachineAny),
			scopes2:  scope.NewScopes(),
			expected: scope.NewScopes(),
		},
		{
			name:     "mixed-1",
			scopes1:  scope.NewScopes(scopeClusterCreate, scope.MachineAny),
			scopes2:  scope.NewScopes(scope.ClusterAny, scope.MachineAny),
			expected: scope.NewScopes(scopeClusterCreate, scope.MachineAny),
		},
		{
			name:     "mixed-2",
			scopes1:  scope.NewScopes(scope.UserDefaultScopes...),
			scopes2:  scope.NewScopes(scopeClusterRead),
			expected: scope.NewScopes(scopeClusterRead),
		},
		{
			name:     "mixed-inverted",
			scopes2:  scope.NewScopes(scope.ClusterAny, scope.MachineAny),
			scopes1:  scope.NewScopes(scopeClusterCreate, scope.MachineAny),
			expected: scope.NewScopes(scopeClusterCreate, scope.MachineAny),
		},
		{
			name:     "complex",
			scopes1:  scope.NewScopes(scopeUserRead, scopeMachineDestroy, scopeClusterUpgrade),
			scopes2:  scope.NewScopes(scope.ClusterAny, scope.MachineAny),
			expected: scope.NewScopes(scopeMachineDestroy, scopeClusterUpgrade),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			intersection := tc.scopes1.Intersect(tc.scopes2)

			expectedStrs := tc.expected.Strings()
			actualStrs := intersection.Strings()

			slices.Sort(expectedStrs)
			slices.Sort(actualStrs)

			assert.Equal(t, expectedStrs, actualStrs)
		})
	}
}
