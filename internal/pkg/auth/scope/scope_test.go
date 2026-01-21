// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package scope_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/auth/scope"
)

func TestParse(t *testing.T) {
	for _, sc := range []scope.Scope{
		scope.ClusterAny,
		scope.MachineAny,
		scope.UserAny,
		scope.New(scope.ObjectUser, scope.ActionCreate, "test-perspective"),
	} {
		s := sc.String()

		parsedScope, err := scope.Parse(s)
		require.NoError(t, err)

		assert.Equal(t, sc, parsedScope)
	}

	for _, sc := range []string{
		"cluster:read",
		"cluster:*",
		"cluster:read.kubeconfig",
	} {
		parsedScope, err := scope.Parse(sc)
		require.NoError(t, err)

		s := parsedScope.String()

		assert.Equal(t, sc, s)
	}
}

func TestMatches(t *testing.T) {
	// same scope matches
	assert.True(t, scope.ClusterAny.Matches(scope.ClusterAny))

	// "any" action matches any action
	assert.True(t, scope.New(scope.ObjectCluster, scope.ActionCreate, scope.PerspectiveNone).Matches(scope.ClusterAny))

	// "any" action matches any action + perspective
	assert.True(t, scope.New(scope.ObjectCluster, scope.ActionCreate, "test-perspective").Matches(scope.ClusterAny))

	// the opposite is not true: specific action does not match "any" action
	assert.False(t, scope.ClusterAny.Matches(scope.New(scope.ObjectCluster, scope.ActionCreate, scope.PerspectiveNone)))

	// different objects don't match
	assert.False(t, scope.ClusterAny.Matches(scope.UserAny))

	// different objects with specific action don't match
	assert.False(t, scope.New(scope.ObjectUser, scope.ActionCreate, scope.PerspectiveNone).
		Matches(scope.New(scope.ObjectCluster, scope.ActionCreate, scope.PerspectiveNone)),
	)

	// different actions don't match
	assert.False(t, scope.New(scope.ObjectUser, scope.ActionCreate, scope.PerspectiveNone).
		Matches(scope.New(scope.ObjectUser, scope.ActionDestroy, scope.PerspectiveNone)),
	)

	// different perspectives don't match
	assert.False(t, scope.New(scope.ObjectUser, scope.ActionCreate, scope.PerspectiveNone).
		Matches(scope.New(scope.ObjectUser, scope.ActionCreate, "test-perspective")),
	)
}
