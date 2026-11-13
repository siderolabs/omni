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

// TestResourceTypesHaveAGroup checks that a type which can be accessed either needs no grant or belongs to a resource group.
//
// It is a tripwire for new resource types: a type which is added to the table with verbs, but without a group,
// would be accessible to nobody, as no role could grant it. This test fails for it.
func TestResourceTypesHaveAGroup(t *testing.T) {
	t.Parallel()

	for _, res := range registry.Resources {
		typ := res.ResourceDefinition().Type
		resourceType := authz.LookupResourceType(typ)

		if resourceType.Verbs == 0 {
			assert.Empty(t, resourceType.Group, "type %s allows no verbs, but has a resource group", typ)

			continue
		}

		assert.Equal(t, resourceType.Access == authz.RequiresGrant, resourceType.Group != "", "type %s", typ)
	}
}

func TestLookupResourceType(t *testing.T) {
	t.Parallel()

	assert.Equal(t, authz.ResourceType{Group: authz.Clusters, Cluster: authz.ClusterByID, Verbs: authz.AllVerbs}, authz.LookupResourceType(omni.ClusterType))
	assert.Equal(t, authz.ResourceType{Group: authz.Clusters, Cluster: authz.ClusterByLabel, Verbs: authz.Read}, authz.LookupResourceType(omni.ClusterMachineType))
	assert.Equal(t, authz.ResourceType{Cluster: authz.ClusterByID}, authz.LookupResourceType(omni.ClusterSecretsType))
	assert.Equal(t, authz.ResourceType{Group: authz.Machines, Cluster: authz.ClusterByMachine, Verbs: authz.Read}, authz.LookupResourceType(omni.MachineStatusType))
	assert.Equal(t, authz.ResourceType{Access: authz.RequiresAuthentication, Verbs: authz.Read}, authz.LookupResourceType(omni.TalosVersionType))
	assert.Equal(t, authz.ResourceType{}, authz.LookupResourceType("Unknown.omni.sidero.dev"))
}

func TestVerbsString(t *testing.T) {
	t.Parallel()

	assert.Empty(t, authz.Verbs(0).String())
	assert.Equal(t, "read", authz.Read.String())
	assert.Equal(t, "create and destroy", (authz.Create | authz.Destroy).String())
	assert.Equal(t, "read, update and destroy", (authz.Read | authz.Update | authz.Destroy).String())
}
