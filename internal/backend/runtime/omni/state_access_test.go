// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
)

var allVerbs = []state.Verb{
	state.Get,
	state.List,
	state.Watch,
	state.Create,
	state.Update,
	state.Destroy,
}

// TestUserManagedResourceTypesAllowAllOperations verifies that every type in
// common.UserManagedResourceTypes is permitted for every CRUD verb by filterAccessByType.
//
// This is the structural invariant of the "user-managed" list: a type belongs in there only if
// users can perform all operations on it via the state API.
func TestUserManagedResourceTypesAllowAllOperations(t *testing.T) {
	t.Parallel()

	for _, rt := range common.UserManagedResourceTypes {
		for _, verb := range allVerbs {
			err := omni.FilterAccessByType(state.Access{ResourceType: rt, Verb: verb})

			assert.NoError(t, err, "user-managed type %q should allow verb %v", rt, verb)
		}
	}
}

// TestFilterAccessByTypeAllRegisteredResources exercises filterAccessByType against every
// resource type registered in registry.Resources with every CRUD verb.
//
// It guards against panics or unexpected error codes and ensures the access filter has a
// deterministic answer (allow or PermissionDenied) for every known resource type.
func TestFilterAccessByTypeAllRegisteredResources(t *testing.T) {
	t.Parallel()

	for _, rd := range registry.Resources {
		rds := rd.ResourceDefinition()

		for _, verb := range allVerbs {
			err := omni.FilterAccessByType(state.Access{
				ResourceNamespace: rds.DefaultNamespace,
				ResourceType:      rds.Type,
				Verb:              verb,
			})
			if err == nil {
				continue
			}

			assert.Equal(t, codes.PermissionDenied, status.Code(err),
				"type %q with verb %v returned unexpected error code: %v", rds.Type, verb, err)
		}
	}
}
