// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/rbac"
)

// RBACBindingControllerName is the name of the controller.
const RBACBindingControllerName = "RBACBindingController"

// RBACBindingController creates RoleBindings for each Identity based on its User's role field,
// and translates AccessPolicy rules into Roles and RoleBindings.
// Primary input is Identity (ID = email), so binding IDs are human-readable.
type RBACBindingController = qtransform.QController[*authres.Identity, *authres.RoleBinding]

// NewRBACBindingController creates a new RBACBindingController.
func NewRBACBindingController() *RBACBindingController {
	return qtransform.NewQController(
		qtransform.Settings[*authres.Identity, *authres.RoleBinding]{
			Name: RBACBindingControllerName,
			MapMetadataFunc: func(identity *authres.Identity) *authres.RoleBinding {
				return authres.NewRoleBinding("user-role:" + identity.Metadata().ID())
			},
			UnmapMetadataFunc: func(binding *authres.RoleBinding) *authres.Identity {
				return authres.NewIdentity(strings.TrimPrefix(binding.Metadata().ID(), "user-role:"))
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, identity *authres.Identity, binding *authres.RoleBinding) error {
				identityID := identity.Metadata().ID() // email
				userID := identity.TypedSpec().Value.GetUserId()

				if userID == "" {
					log.Printf("[RBAC] controller: identity=%s has no user ID, skipping", identityID)

					return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("identity %q has no user ID", identityID))
				}

				// Read the User to get the legacy role.
				user, err := safe.ReaderGetByID[*authres.User](ctx, r, userID)
				if err != nil {
					if state.IsNotFoundError(err) {
						log.Printf("[RBAC] controller: identity=%s user=%s not found, skipping", identityID, userID)

						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return fmt.Errorf("failed to get user %q: %w", userID, err)
				}

				legacyRole := user.TypedSpec().Value.GetRole()

				log.Printf("[RBAC] controller: reconciling identity=%s user=%s legacyRole=%s", identityID, userID, legacyRole)

				builtinRoleID := mapLegacyRole(legacyRole)
				if builtinRoleID == "" {
					log.Printf("[RBAC] controller: identity=%s has no mappable role (legacy=%s), skipping", identityID, legacyRole)

					return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("user %q has no role", userID))
				}

				binding.TypedSpec().Value.RoleRef = builtinRoleID
				binding.TypedSpec().Value.Subjects = []*specs.RoleBindingSpec_Subject{
					{Name: identityID},
				}

				log.Printf("[RBAC] controller: identity=%s -> RoleBinding id=%s roleRef=%s", identityID, binding.Metadata().ID(), builtinRoleID)

				// Process AccessPolicy.
				if err := reconcileACLBindings(ctx, r, logger, user); err != nil {
					log.Printf("[RBAC] controller: identity=%s ACL reconciliation error: %v", identityID, err)
				}

				return nil
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, identity *authres.Identity) error {
				userID := identity.TypedSpec().Value.GetUserId()

				log.Printf("[RBAC] controller: identity=%s deleted, cleaning up ACL bindings for user=%s", identity.Metadata().ID(), userID)

				return cleanupACLBindings(ctx, r, userID)
			},
		},
		// When User changes (role update), re-reconcile its identities.
		qtransform.WithExtraMappedInput[*authres.User](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, user controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				identities, err := safe.ReaderListAll[*authres.Identity](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(authres.LabelIdentityUserID, user.ID()),
				))
				if err != nil {
					return nil, err
				}

				var pointers []resource.Pointer

				for id := range identities.All() {
					pointers = append(pointers, id.Metadata())
				}

				log.Printf("[RBAC] controller: User %s changed, re-reconciling %d identities", user.ID(), len(pointers))

				return pointers, nil
			},
		),
		// When AccessPolicy changes, re-reconcile all identities.
		qtransform.WithExtraMappedInput[*authres.AccessPolicy](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, ap controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				log.Printf("[RBAC] controller: AccessPolicy changed (id=%s), re-reconciling all identities", ap.ID())

				identities, err := safe.ReaderListAll[*authres.Identity](ctx, r)
				if err != nil {
					return nil, err
				}

				var pointers []resource.Pointer

				for id := range identities.All() {
					pointers = append(pointers, id.Metadata())
				}

				log.Printf("[RBAC] controller: will re-reconcile %d identities due to AccessPolicy change", len(pointers))

				return pointers, nil
			},
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: authres.RoleType,
				Kind: controller.OutputShared,
			},
		),
	)
}

func mapLegacyRole(legacyRole string) string {
	switch strings.ToLower(legacyRole) {
	case "admin":
		return rbac.RoleIDAdmin
	case "operator":
		return rbac.RoleIDOperator
	case "reader":
		return rbac.RoleIDReader
	case "none", "":
		return ""
	default:
		return ""
	}
}

func reconcileACLBindings(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, user *authres.User) error {
	userID := user.Metadata().ID()

	accessPolicy, err := safe.ReaderGetByID[*authres.AccessPolicy](ctx, r, authres.AccessPolicyID)
	if err != nil {
		if state.IsNotFoundError(err) {
			log.Printf("[RBAC] controller: user=%s no AccessPolicy found, cleaning up ACL bindings", userID)

			return cleanupACLBindings(ctx, r, userID)
		}

		return fmt.Errorf("failed to get access policy: %w", err)
	}

	spec := accessPolicy.TypedSpec().Value
	ruleCount := len(spec.GetRules())

	log.Printf("[RBAC] controller: user=%s processing AccessPolicy with %d rules", userID, ruleCount)

	keepBindings := map[string]struct{}{}

	for i, rule := range spec.GetRules() {
		matches := aclRuleMatchesUser(spec, rule, userID)

		log.Printf("[RBAC] controller: user=%s ACL rule[%d] users=%v clusters=%v role=%s -> matches=%v",
			userID, i, rule.GetUsers(), rule.GetClusters(), rule.GetRole(), matches)

		if !matches {
			continue
		}

		roleID := aclRoleID(i, rule)
		bindingID := aclBindingID(userID, roleID)

		keepBindings[bindingID] = struct{}{}

		if err := ensureACLRole(ctx, r, roleID, rule); err != nil {
			return fmt.Errorf("failed to ensure ACL role %q: %w", roleID, err)
		}

		log.Printf("[RBAC] controller: user=%s created/updated ACL role=%s", userID, roleID)

		if err := safe.WriterModify(ctx, r, authres.NewRoleBinding(bindingID), func(rb *authres.RoleBinding) error {
			rb.TypedSpec().Value.RoleRef = roleID
			rb.TypedSpec().Value.Subjects = []*specs.RoleBindingSpec_Subject{
				{Name: userID},
			}

			rb.Metadata().Labels().Set("omni.sidero.dev/source", "access-policy")
			rb.Metadata().Labels().Set("omni.sidero.dev/user", userID)

			return nil
		}); err != nil {
			return fmt.Errorf("failed to ensure ACL binding %q: %w", bindingID, err)
		}

		log.Printf("[RBAC] controller: user=%s created/updated ACL binding=%s -> role=%s", userID, bindingID, roleID)
	}

	return cleanupStaleACLBindings(ctx, r, userID, keepBindings)
}

func ensureACLRole(ctx context.Context, r controller.ReaderWriter, roleID string, rule *specs.AccessPolicyRule) error {
	return safe.WriterModify(ctx, r, authres.NewRole(roleID), func(role *authres.Role) error {
		rbacRules := translateACLRule(rule)
		role.TypedSpec().Value.Rules = rbacRules
		role.Metadata().Labels().Set("omni.sidero.dev/source", "access-policy")

		return nil
	})
}

func translateACLRule(rule *specs.AccessPolicyRule) []*specs.RoleSpec_Rule {
	verbs := []string{"get", "list", "watch"}

	switch strings.ToLower(rule.GetRole()) {
	case "operator":
		verbs = []string{"get", "list", "watch", "create", "update", "destroy"}
	case "admin":
		verbs = []string{"*"}
	}

	clusters := rule.GetClusters()

	rbacRule := &specs.RoleSpec_Rule{
		Resources:        []string{rbac.FamilyClusters},
		Verbs:            verbs,
		Clusters:         clusters,
		KubernetesGroups: rule.GetKubernetes().GetImpersonate().GetGroups(),
	}

	switch strings.ToLower(rule.GetRole()) {
	case "operator", "admin":
		rbacRule.KubernetesGroups = append(rbacRule.KubernetesGroups, "system:masters")
	}

	log.Printf("[RBAC] controller: translated ACL rule -> resources=%v verbs=%v clusters=%v k8sGroups=%v",
		rbacRule.GetResources(), rbacRule.GetVerbs(), rbacRule.GetClusters(), rbacRule.GetKubernetesGroups())

	return []*specs.RoleSpec_Rule{rbacRule}
}

func cleanupACLBindings(ctx context.Context, r controller.ReaderWriter, userID string) error {
	return cleanupStaleACLBindings(ctx, r, userID, nil)
}

func cleanupStaleACLBindings(ctx context.Context, r controller.ReaderWriter, userID string, keep map[string]struct{}) error {
	bindings, err := safe.ReaderListAll[*authres.RoleBinding](ctx, r, state.WithLabelQuery(
		resource.LabelEqual("omni.sidero.dev/source", "access-policy"),
		resource.LabelEqual("omni.sidero.dev/user", userID),
	))
	if err != nil {
		return err
	}

	for binding := range bindings.All() {
		if binding.Metadata().Owner() != RBACBindingControllerName {
			continue
		}

		if _, ok := keep[binding.Metadata().ID()]; ok {
			continue
		}

		log.Printf("[RBAC] controller: user=%s destroying stale ACL binding=%s", userID, binding.Metadata().ID())

		if err := r.Destroy(ctx, binding.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to destroy stale ACL binding %q: %w", binding.Metadata().ID(), err)
		}
	}

	return nil
}

func aclRuleMatchesUser(spec *specs.AccessPolicySpec, rule *specs.AccessPolicyRule, userID string) bool {
	for _, userRef := range rule.GetUsers() {
		if strings.HasPrefix(userRef, "group/") {
			groupName := strings.TrimPrefix(userRef, "group/")

			group, ok := spec.GetUserGroups()[groupName]
			if !ok {
				continue
			}

			for _, u := range group.GetUsers() {
				if u.GetName() != "" && u.GetName() == userID {
					return true
				}

				if pattern := u.GetMatch(); pattern != "" {
					if matched, _ := filepath.Match(pattern, userID); matched {
						return true
					}
				}
			}
		} else if userRef == userID {
			return true
		}
	}

	return false
}

func aclRoleID(index int, rule *specs.AccessPolicyRule) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d:%s:%v", index, rule.GetRole(), rule.GetClusters())

	return fmt.Sprintf("acl-%x", h.Sum(nil)[:8])
}

func aclBindingID(userID, roleID string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%s", userID, roleID)

	return fmt.Sprintf("acl-binding-%x", h.Sum(nil)[:8])
}
