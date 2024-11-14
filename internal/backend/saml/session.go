// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package saml contains SAML setup handlers.
package saml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/auth/user"
)

// UserInfo describes user identity and fullname.
type UserInfo struct {
	// Identity ...
	Identity string
	// Fullname ...
	Fullname string
}

// NewSessionProvider creates a new SessionProvider.
func NewSessionProvider(state state.State, tracker samlsp.RequestTracker, logger *zap.Logger) *SessionProvider {
	return &SessionProvider{
		state:   state,
		tracker: tracker,
		logger:  logger,
	}
}

// SessionProvider is an implementation of SessionProvider that stores
// session tokens in the COSI state.
type SessionProvider struct {
	state   state.State
	tracker samlsp.RequestTracker
	logger  *zap.Logger
}

// CreateSession is called when we have received a valid SAML assertion and
// should create a new session and do redirect.
func (sp *SessionProvider) CreateSession(w http.ResponseWriter, r *http.Request, assertion *saml.Assertion) error {
	hashInput := ""

	if assertion.Subject == nil {
		return errors.New("no subject in the assertion")
	}

	for _, subjectConfirmation := range assertion.Subject.SubjectConfirmations {
		hashInput += subjectConfirmation.SubjectConfirmationData.InResponseTo
	}

	var logFields []zap.Field

	if sub := assertion.Subject; sub != nil {
		if nameID := sub.NameID; nameID != nil {
			logFields = append(logFields, zap.String("subject_id", nameID.Value))
		}
	}

	for _, attributeStatement := range assertion.AttributeStatements {
		for _, attr := range attributeStatement.Attributes {
			name := attr.FriendlyName
			if name == "" {
				name = attr.Name
			}

			var value string

			if len(attr.Values) > 0 {
				value = attr.Values[0].Value
			}

			logFields = append(logFields, zap.String(name, value))
		}
	}

	sp.logger.Info("new SAML assertion", logFields...)

	var session string

	if hashInput == "" {
		session = uuid.New().String()
	} else {
		h := sha256.New()

		h.Write([]byte(hashInput))

		session = hex.EncodeToString(h.Sum(nil))
	}

	query := r.URL.Query()

	if trackedRequestIndex := r.Form.Get("RelayState"); trackedRequestIndex != "" {
		trackedRequest, err := sp.tracker.GetTrackedRequest(r, trackedRequestIndex)
		if err != nil {
			return err
		}

		redirectURI, err := url.Parse(trackedRequest.URI)
		if err != nil {
			return err
		}

		query = redirectURI.Query()
	}

	query.Set("session", session)

	user, err := LocateUserInfo(assertion)
	if err != nil {
		return err
	}

	query.Set("identity", user.Identity)
	query.Set("fullname", user.Fullname)

	if user.Identity == "" {
		return errors.New("couldn't find user identity in the SAML assertion")
	}

	samlAssertion := auth.NewSAMLAssertion(resources.DefaultNamespace, session)

	samlAssertion.TypedSpec().Value.Data, err = json.Marshal(assertion)
	if err != nil {
		return err
	}

	samlAssertion.TypedSpec().Value.Email = user.Identity

	ctx := actor.MarkContextAsInternalActor(r.Context())

	if err = sp.state.Create(ctx, samlAssertion); err != nil {
		return err
	}

	samlLabels, err := sp.ReadLabelsFromAssertion(ctx, assertion)
	if err != nil {
		return err
	}

	if err = sp.ensureUser(ctx, user.Identity, samlLabels); err != nil {
		return err
	}

	http.Redirect(w, r, fmt.Sprintf("/omni/authenticate?%s", query.Encode()), http.StatusSeeOther)

	return nil
}

// DeleteSession shouldn't be used by SAML provider configured for Omni.
// Session cleanup is done on the first usage or by a timeout.
func (sp *SessionProvider) DeleteSession(http.ResponseWriter, *http.Request) error {
	return errors.New("not implemented")
}

// GetSession shouldn't be used by SAML provider configured for Omni.
func (sp *SessionProvider) GetSession(*http.Request) (samlsp.Session, error) {
	return nil, errors.New("not implemented")
}

// ReadLabelsFromAssertion extract user labels from the SAML assertion attributes.
func (sp *SessionProvider) ReadLabelsFromAssertion(ctx context.Context, assertion *saml.Assertion) (map[string]string, error) {
	knownFields := map[string]string{
		// Microsoft
		"http://schemas.microsoft.com/ws/2008/06/identity/claims/role": "role",
		// Google
		"Role": "role",
		// Samltest
		"role": "role",
	}

	config, err := safe.ReaderGet[*auth.Config](ctx, sp.state, auth.NewAuthConfig().Metadata())
	if err != nil {
		return nil, err
	}

	customFields := config.TypedSpec().Value.Saml.LabelRules

	get := func(attr saml.Attribute, from map[string]string) (string, []string) {
		dest, ok := from[attr.FriendlyName]
		if !ok {
			dest, ok = from[attr.Name]

			if !ok {
				return "", nil
			}
		}

		return dest, xslices.Map(attr.Values, func(val saml.AttributeValue) string {
			return val.Value
		})
	}

	samlLabels := make(map[string]string)

	for _, attributeStatement := range assertion.AttributeStatements {
		for _, attr := range attributeStatement.Attributes {
			if len(attr.Values) == 0 {
				continue
			}

			key, values := get(attr, customFields)
			if key == "" {
				key, values = get(attr, knownFields)
			}

			if key == "" {
				continue
			}

			for _, value := range values {
				samlLabels[fmt.Sprintf("%s%s/%s", auth.SAMLLabelPrefix, key, value)] = ""
			}
		}
	}

	return samlLabels, nil
}

func (sp *SessionProvider) ensureUser(ctx context.Context, email string, samlLabels map[string]string) error {
	users, err := sp.state.List(ctx, auth.NewUser(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		return err
	}

	r := role.Admin
	if len(users.Items) > 0 {
		r, err = sp.getRoleInSAMLLabelRules(ctx, samlLabels)
		if err != nil {
			return err
		}
	}

	if err = user.Ensure(ctx, sp.state, email, r); err != nil {
		return err
	}

	if err = sp.updateIdentityLabels(ctx, email, samlLabels); err != nil {
		return err
	}

	return err
}

func (sp *SessionProvider) updateIdentityLabels(ctx context.Context, identity string, samlLabels map[string]string) error {
	identityPtr := auth.NewIdentity(resources.DefaultNamespace, identity).Metadata()

	_, err := safe.StateUpdateWithConflicts[*auth.Identity](ctx, sp.state, identityPtr, func(r *auth.Identity) error {
		var toDelete []string

		for _, label := range r.Metadata().Labels().Keys() {
			if !strings.HasPrefix(label, auth.SAMLLabelPrefix) {
				continue
			}

			if _, ok := samlLabels[label]; !ok {
				toDelete = append(toDelete, label)
			}
		}

		r.Metadata().Labels().Do(func(temp kvutils.TempKV) {
			for k, v := range samlLabels {
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

// getRoleInSAMLLabelRules returns the highest role found in the SAML label rules.
//
// If there is no rule matching the labels, role.None is returned.
func (sp *SessionProvider) getRoleInSAMLLabelRules(ctx context.Context, samlLabels map[string]string) (role.Role, error) {
	labelRuleList, err := safe.ReaderListAll[*auth.SAMLLabelRule](ctx, sp.state)
	if err != nil {
		return "", err
	}

	labelRules := slices.AppendSeq(make([]*auth.SAMLLabelRule, 0, labelRuleList.Len()), labelRuleList.All())

	return RoleInSAMLLabelRules(labelRules, samlLabels, sp.logger), nil
}

// RoleInSAMLLabelRules returns the role on the SAMLLabelRules with the highest access level that matches the labels.
func RoleInSAMLLabelRules(samlLabelRules []*auth.SAMLLabelRule, samlLabels map[string]string, logger *zap.Logger) role.Role {
	samlRole := role.None

	resLabels := resource.Labels{}

	for key, value := range samlLabels {
		resLabels.Set(key, value)
	}

	for _, labelRule := range samlLabelRules {
		selectors, selectorsErr := labels.ParseSelectors(labelRule.TypedSpec().Value.GetMatchLabels())
		if selectorsErr != nil {
			logger.Warn("skip invalid match labels on identity label rule", zap.Error(selectorsErr))

			continue
		}

		if selectors.Matches(resLabels) {
			parsedRole, parseErr := role.Parse(labelRule.TypedSpec().Value.GetAssignRoleOnRegistration())
			if parseErr != nil {
				logger.Warn("skip invalid role on identity label rule", zap.Error(parseErr))

				continue
			}

			maxRole, maxErr := role.Max(parsedRole, samlRole)
			if maxErr != nil {
				logger.Warn("skip invalid role on identity label rule", zap.Error(maxErr))

				continue
			}

			samlRole = maxRole
		}
	}

	return samlRole
}

// LocateUserInfo searches for user email and fullname in the ACS response.
func LocateUserInfo(assertion *saml.Assertion) (UserInfo, error) {
	var (
		user      UserInfo
		givenName string
		surname   string
	)

	copyFields := map[string]*string{
		// samltest.sp SAML.
		"urn:oasis:names:tc:SAML:attribute:subject-id": &user.Identity,
		"displayName": &user.Fullname,
		// Microsoft SAML.
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name": &user.Identity,
		"http://schemas.microsoft.com/identity/claims/displayname":   &user.Fullname,
		// keycloak X500 mapping
		"email":     &user.Identity,
		"givenName": &givenName,
		"surname":   &surname,
		// Zitadel SAML
		"UserName": &user.Identity,
		"FullName": &user.Fullname,
	}

	// Google SAML keeps that info in Subject.
	if strings.Contains(assertion.Issuer.Value, "google.com") {
		if sub := assertion.Subject; sub != nil {
			if nameID := sub.NameID; nameID != nil {
				user.Identity = nameID.Value
				user.Fullname = nameID.Value
			}
		}
	}

	for _, attributeStatement := range assertion.AttributeStatements {
		for _, attr := range attributeStatement.Attributes {
			param, ok := copyFields[attr.FriendlyName]
			if !ok {
				param, ok = copyFields[attr.Name]

				if !ok {
					continue
				}
			}

			if len(attr.Values) == 0 {
				continue
			}

			*param = attr.Values[0].Value
		}
	}

	if givenName != "" && surname != "" {
		user.Fullname = givenName + " " + surname
	}

	if user.Identity == "" {
		return UserInfo{}, errors.New("unsupported SAML IDP: cannot find user identity in the assertion")
	}

	user.Identity = strings.ToLower(user.Identity)

	return user, nil
}
