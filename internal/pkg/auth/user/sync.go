// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package user

import (
	"context"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

// Syncer syncs a user with the labels from an identity provider login.
type Syncer struct {
	state         state.State
	logger        *zap.Logger
	recoveryAdmin string
	allowCreate   bool
}

// NewSyncer creates a new Syncer.
//
// When allowCreate is false, Sync does nothing for a user who does not exist yet.
func NewSyncer(st state.State, logger *zap.Logger, recoveryAdmin string, allowCreate bool) *Syncer {
	return &Syncer{
		state:         st,
		logger:        logger,
		recoveryAdmin: strings.ToLower(recoveryAdmin),
		allowCreate:   allowCreate,
	}
}

// Sync creates the user if needed, assigns the role from the matching IdentityLabelRules and replaces the identity provider labels on the identity.
func (s *Syncer) Sync(ctx context.Context, email string, idpLabels map[string]string) error {
	email = strings.ToLower(email)

	if !s.allowCreate {
		if _, err := s.state.Get(ctx, auth.NewIdentity(email).Metadata()); err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}
	}

	users, err := s.state.List(ctx, auth.NewUser("").Metadata())
	if err != nil {
		return err
	}

	var updateOnEachLogin bool

	r := role.Admin

	if len(users.Items) > 0 {
		var labelRule *auth.IdentityLabelRule

		labelRule, err = s.getIdentityLabelRule(ctx, idpLabels)
		if err != nil {
			return err
		}

		r = role.None

		if labelRule != nil {
			r = role.Role(labelRule.TypedSpec().Value.AssignRole)
			updateOnEachLogin = labelRule.TypedSpec().Value.UpdateOnEachLogin
		}
	}

	if s.recoveryAdmin != "" && email == s.recoveryAdmin && r != role.Admin {
		s.logger.Warn(
			"skipping role update for the recovery admin",
			zap.String("email", email),
			zap.String("rule_role", string(r)),
		)

		updateOnEachLogin = false
	}

	if err = Ensure(ctx, s.state, email, r, updateOnEachLogin); err != nil {
		return err
	}

	return s.updateIdentityLabels(ctx, email, idpLabels)
}

func (s *Syncer) updateIdentityLabels(ctx context.Context, identity string, idpLabels map[string]string) error {
	identityPtr := auth.NewIdentity(identity).Metadata()

	_, err := safe.StateUpdateWithConflicts[*auth.Identity](ctx, s.state, identityPtr, func(r *auth.Identity) error {
		var toDelete []string

		for _, label := range r.Metadata().Labels().Keys() {
			// labels of the other provider are removed too, as only one provider is active at a time
			if !strings.HasPrefix(label, auth.SAMLLabelPrefix) && !strings.HasPrefix(label, auth.OIDCLabelPrefix) {
				continue
			}

			if _, ok := idpLabels[label]; !ok {
				toDelete = append(toDelete, label)
			}
		}

		r.Metadata().Labels().Do(func(temp kvutils.TempKV) {
			for k, v := range idpLabels {
				temp.Set(k, v)
			}

			for _, k := range toDelete {
				temp.Delete(k)
			}
		})

		return nil
	})

	return err
}

// getIdentityLabelRule returns the rule with the highest role among the IdentityLabelRules matching the labels.
//
// If there is no rule matching the labels, nil is returned.
func (s *Syncer) getIdentityLabelRule(ctx context.Context, idpLabels map[string]string) (*auth.IdentityLabelRule, error) {
	labelRuleList, err := safe.ReaderListAll[*auth.IdentityLabelRule](ctx, s.state)
	if err != nil {
		return nil, err
	}

	labelRules := slices.AppendSeq(make([]*auth.IdentityLabelRule, 0, labelRuleList.Len()), labelRuleList.All())

	return MatchIdentityLabelRule(labelRules, idpLabels, s.logger), nil
}

// MatchIdentityLabelRule returns the IdentityLabelRule with the highest access level that matches the labels.
func MatchIdentityLabelRule(rules []*auth.IdentityLabelRule, idpLabels map[string]string, logger *zap.Logger) *auth.IdentityLabelRule {
	matchedRole := role.None

	resLabels := resource.Labels{}

	for key, value := range idpLabels {
		resLabels.Set(key, value)
	}

	var rule *auth.IdentityLabelRule

	for _, labelRule := range rules {
		selectors, selectorsErr := labels.ParseSelectors(labelRule.TypedSpec().Value.GetMatchLabels())
		if selectorsErr != nil {
			logger.Warn("skip invalid match labels on identity label rule", zap.Error(selectorsErr))

			continue
		}

		if selectors.Matches(resLabels) {
			parsedRole, parseErr := role.Parse(labelRule.TypedSpec().Value.AssignRole)
			if parseErr != nil {
				logger.Warn("skip invalid role on identity label rule", zap.Error(parseErr))

				continue
			}

			maxRole, maxErr := role.Max(parsedRole, matchedRole)
			if maxErr != nil {
				logger.Warn("skip invalid role on identity label rule", zap.Error(maxErr))

				continue
			}

			if rule == nil || matchedRole.Compare(maxRole) == -1 {
				rule = labelRule
			}

			matchedRole = maxRole
		}
	}

	return rule
}
