// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/internal/pkg/authz"
)

// TestUserManagedResourceTypes checks that the resource types with all verbs are exactly the user-managed ones.
func TestUserManagedResourceTypes(t *testing.T) {
	t.Parallel()

	for _, typ := range common.UserManagedResourceTypes {
		assert.Equal(t, authz.AllVerbs, authz.LookupResourceType(typ).Verbs, "user-managed type %s", typ)
	}

	for _, res := range registry.Resources {
		typ := res.ResourceDefinition().Type

		if authz.LookupResourceType(typ).Verbs == authz.AllVerbs {
			assert.True(t, slices.Contains(common.UserManagedResourceTypes, typ), "type %s has all verbs, but is not user-managed", typ)
		}
	}
}

func TestLookupResourceType(t *testing.T) {
	t.Parallel()

	assert.Equal(t, authz.ResourceType{Cluster: authz.ClusterByID, Verbs: authz.AllVerbs}, authz.LookupResourceType(omni.ClusterType))
	assert.Equal(t, authz.ResourceType{Cluster: authz.ClusterByLabel, Verbs: authz.Read}, authz.LookupResourceType(omni.ClusterMachineType))
	assert.Equal(t, authz.ResourceType{Cluster: authz.ClusterByID}, authz.LookupResourceType(omni.ClusterSecretsType))
	assert.Equal(t, authz.ResourceType{}, authz.LookupResourceType("Unknown.omni.sidero.dev"))
}

func TestVerbsString(t *testing.T) {
	t.Parallel()

	assert.Empty(t, authz.Verbs(0).String())
	assert.Equal(t, "read", authz.Read.String())
	assert.Equal(t, "create and destroy", (authz.Create | authz.Destroy).String())
	assert.Equal(t, "read, update and destroy", (authz.Read | authz.Update | authz.Destroy).String())
}
