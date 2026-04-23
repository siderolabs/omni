// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package rbac

import (
	"context"
	"log"
	"path/filepath"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	pkgauth "github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// Check evaluates RBAC for the given resource access.
func Check(ctx context.Context, st state.State, resourceType resource.Type, verb state.Verb, clusterID resource.ID, logger *zap.Logger) error {
	// 1. Internal actor bypass.
	if actor.ContextIsInternalActor(ctx) {
		return nil
	}

	// 2. AuthConfig is public (no auth required).
	if resourceType == authres.AuthConfigType {
		return nil
	}

	// 3. Look up type in family registry.
	config, ok := Registry[resourceType]
	if !ok {
		log.Printf("[RBAC] DENIED type=%s not in registry", resourceType)

		return status.Errorf(codes.PermissionDenied, "no access is permitted on resource type %q", resourceType)
	}

	// 4. System family: only requires authentication.
	if config.Family == FamilySystem {
		err := checkAuthenticated(ctx)
		if err != nil {
			log.Printf("[RBAC] DENIED type=%s family=system reason=unauthenticated", resourceType)
		}

		return err
	}

	// 5. Check if this COSI verb is allowed on this resource type at all (system invariant).
	verbStr := verbToString(verb)

	if _, allowed := config.AllowedVerbs[verb]; !allowed {
		log.Printf("[RBAC] DENIED type=%s family=%s verb=%s reason=verb-not-allowed", resourceType, config.Family, verbStr)

		return status.Errorf(codes.PermissionDenied, "%s access is not permitted on resource type %q", verbStr, resourceType)
	}

	// 6. Get user identity from context.
	identity, ok := getIdentity(ctx)
	if !ok {
		log.Printf("[RBAC] DENIED type=%s family=%s verb=%s reason=no-identity", resourceType, config.Family, verbStr)

		return status.Error(codes.Unauthenticated, "no identity in context")
	}

	// 8. Collect matching RoleBindings.
	internalCtx := actor.MarkContextAsInternalActor(ctx)

	bindings, err := safe.StateListAll[*authres.RoleBinding](internalCtx, st)
	if err != nil {
		log.Printf("[RBAC] ERROR identity=%s listing bindings: %v", identity, err)

		return status.Errorf(codes.Internal, "failed to list role bindings: %v", err)
	}

	bindingCount := 0
	for iter := bindings.Iterator(); iter.Next(); {
		bindingCount++
	}

	log.Printf("[RBAC] CHECK identity=%s type=%s family=%s verb=%s cluster=%s bindings_total=%d",
		identity, resourceType, config.Family, verbStr, clusterID, bindingCount)

	// Get the identity resource for label matching.
	identityRes, identityErr := safe.StateGetByID[*authres.Identity](internalCtx, st, identity)

	var matchedRule *matchResult

	// Re-iterate bindings (iterator was consumed above).
	bindings, _ = safe.StateListAll[*authres.RoleBinding](internalCtx, st)

	for iter := bindings.Iterator(); iter.Next(); {
		binding := iter.Value()

		if !subjectMatches(binding.TypedSpec().Value, identity, identityRes, identityErr) {
			continue
		}

		roleID := binding.TypedSpec().Value.GetRoleRef()
		if roleID == "" {
			log.Printf("[RBAC]   binding=%s matched but has empty roleRef, skipping", binding.Metadata().ID())

			continue
		}

		log.Printf("[RBAC]   binding=%s matched, roleRef=%s", binding.Metadata().ID(), roleID)

		roleRes, roleErr := safe.StateGetByID[*authres.Role](internalCtx, st, roleID)
		if roleErr != nil {
			log.Printf("[RBAC]   role=%s not found: %v", roleID, roleErr)

			continue
		}

		for ruleIdx, rule := range roleRes.TypedSpec().Value.GetRules() {
			matches := ruleMatches(rule, config.Family, verbStr, clusterID, resourceType)

			log.Printf("[RBAC]   role=%s rule[%d] resources=%v verbs=%v clusters=%v -> match=%v",
				roleID, ruleIdx, rule.GetResources(), rule.GetVerbs(), rule.GetClusters(), matches)

			if matches {
				matchedRule = &matchResult{
					roleID:    roleID,
					bindingID: binding.Metadata().ID(),
					rule:      rule,
				}

				break
			}
		}

		if matchedRule != nil {
			break
		}
	}

	// 9. Decision.
	if matchedRule != nil {
		log.Printf("[RBAC] ALLOWED identity=%s type=%s family=%s verb=%s cluster=%s via role=%s binding=%s",
			identity, resourceType, config.Family, verbStr, clusterID, matchedRule.roleID, matchedRule.bindingID)

		return nil
	}

	log.Printf("[RBAC] DENIED identity=%s type=%s family=%s verb=%s cluster=%s reason=no-matching-rule",
		identity, resourceType, config.Family, verbStr, clusterID)

	return status.Errorf(codes.PermissionDenied, "insufficient permissions: %s %s on %s", verbStr, config.Family, resourceType)
}

// IsClusterScoped returns true if the resource type is in clusterIDTypeSet or clusterLabelTypeSet.
var IsClusterScoped func(resource.Type) bool


type matchResult struct {
	roleID    string
	bindingID string
	rule      *specs.RoleSpec_Rule
}

// verbToString maps a COSI verb to its string representation for RBAC rules.
func verbToString(v state.Verb) string {
	switch v {
	case state.Get:
		return "get"
	case state.List:
		return "list"
	case state.Watch:
		return "watch"
	case state.Create:
		return "create"
	case state.Update:
		return "update"
	case state.Destroy:
		return "destroy"
	default:
		return "unknown"
	}
}

// matchesVerb checks if a rule's verb list grants the requested verb.
// Supports individual verbs and the "*" wildcard.
func matchesVerb(ruleVerbs []string, verb string) bool {
	for _, v := range ruleVerbs {
		if v == "*" || v == verb {
			return true
		}
	}

	return false
}

func checkAuthenticated(ctx context.Context) error {
	authVal, ok := ctxstore.Value[pkgauth.EnabledAuthContextKey](ctx)
	if ok && !authVal.Enabled {
		return nil
	}

	if _, ok = getIdentity(ctx); !ok {
		return status.Error(codes.Unauthenticated, "authentication required")
	}

	return nil
}

func getIdentity(ctx context.Context) (string, bool) {
	val, ok := ctxstore.Value[pkgauth.IdentityContextKey](ctx)
	if !ok || val.Identity == "" {
		return "", false
	}

	return val.Identity, true
}

func subjectMatches(spec *specs.RoleBindingSpec, identity string, identityRes *authres.Identity, identityErr error) bool {
	for _, subject := range spec.GetSubjects() {
		if subject.GetName() != "" && subject.GetName() == identity {
			return true
		}

		if pattern := subject.GetMatch(); pattern != "" {
			if matched, _ := filepath.Match(pattern, identity); matched {
				return true
			}
		}

		if selectors := subject.GetLabelSelectors(); len(selectors) > 0 && identityErr == nil && identityRes != nil {
			if labelsMatch(identityRes.Metadata(), selectors) {
				return true
			}
		}
	}

	return false
}

func labelsMatch(md *resource.Metadata, selectors []string) bool {
	for _, sel := range selectors {
		if !md.Labels().Matches(resource.LabelTerm{
			Key: sel,
		}) {
			return false
		}
	}

	return true
}

func ruleMatches(rule *specs.RoleSpec_Rule, family, verb string, clusterID resource.ID, resourceType resource.Type) bool {
	if !sliceContains(rule.GetResources(), family) && !sliceContains(rule.GetResources(), "*") {
		return false
	}

	if !matchesVerb(rule.GetVerbs(), verb) {
		return false
	}

	clusters := rule.GetClusters()
	if len(clusters) > 0 {
		if IsClusterScoped == nil || !IsClusterScoped(resourceType) {
			return false
		}

		if clusterID == "" {
			return false
		}

		matched := false

		for _, pattern := range clusters {
			if pattern == "*" {
				matched = true

				break
			}

			if ok, _ := filepath.Match(pattern, string(clusterID)); ok {
				matched = true

				break
			}
		}

		if !matched {
			return false
		}
	}

	return true
}

func sliceContains(slice []string, value string) bool {
	return slices.Contains(slice, value)
}
