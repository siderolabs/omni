// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package rbac

import (
	"context"
	"fmt"
	"log"

	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

// BuiltinOwner is the owner string for built-in RBAC resources.
// Using a non-empty owner makes these resources impossible to delete or modify via the user-facing API,
// because COSI enforces that only the owner can modify/destroy a resource.
// No controller uses this owner string, so nothing will ever tear them down.
const BuiltinOwner = "rbac-builtin"

// Built-in role IDs.
const (
	RoleIDNone     = "none"
	RoleIDReader   = "reader"
	RoleIDOperator = "operator"
	RoleIDAdmin    = "admin"
)

// builtinRoles defines the 4 built-in roles that map to the legacy role hierarchy.
var builtinRoles = []struct {
	id    string
	rules []*specs.RoleSpec_Rule
}{
	{
		id:    RoleIDNone,
		rules: nil,
	},
	{
		id: RoleIDReader,
		rules: []*specs.RoleSpec_Rule{
			{
				Resources: []string{FamilyClusters, FamilyMachines},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	},
	{
		id: RoleIDOperator,
		rules: []*specs.RoleSpec_Rule{
			{
				Resources:        []string{FamilyClusters, FamilyMachines},
				Verbs:            []string{"get", "list", "watch", "create", "update", "destroy"},
				KubernetesGroups: []string{"system:masters"},
			},
		},
	},
	{
		id: RoleIDAdmin,
		rules: []*specs.RoleSpec_Rule{
			{
				Resources:        []string{"*"},
				Verbs:            []string{"*"},
				KubernetesGroups: []string{"system:masters"},
			},
		},
	},
}

// EnsureBuiltinRoles creates the built-in Role resources if they don't exist.
// They are created with a special owner so that users cannot delete or modify them.
func EnsureBuiltinRoles(ctx context.Context, st state.State, logger *zap.Logger) error {
	log.Printf("[RBAC] ensuring built-in roles exist...")

	for _, def := range builtinRoles {
		role := authres.NewRole(def.id)
		role.TypedSpec().Value.Rules = def.rules
		role.Metadata().Labels().Set("omni.sidero.dev/built-in", "true")

		existing, err := st.Get(ctx, role.Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to check built-in role %q: %w", def.id, err)
		}

		if existing != nil {
			log.Printf("[RBAC] built-in role %q already exists, skipping", def.id)

			continue
		}

		if err := st.Create(ctx, role, state.WithCreateOwner(BuiltinOwner)); err != nil {
			return fmt.Errorf("failed to create built-in role %q: %w", def.id, err)
		}

		ruleCount := len(def.rules)
		if ruleCount > 0 {
			r := def.rules[0]
			log.Printf("[RBAC] created built-in role %q with %d rule(s), first rule: resources=%v verbs=%v clusters=%v k8sGroups=%v",
				def.id, ruleCount, r.GetResources(), r.GetVerbs(), r.GetClusters(), r.GetKubernetesGroups())
		} else {
			log.Printf("[RBAC] created built-in role %q with 0 rules (no permissions)", def.id)
		}
	}

	log.Printf("[RBAC] built-in roles ensured")

	return nil
}
