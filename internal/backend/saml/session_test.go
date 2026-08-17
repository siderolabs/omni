// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package saml_test

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	csaml "github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/saml"
	"github.com/siderolabs/omni/internal/pkg/auth/user"
)

func TestUserInfo(t *testing.T) {
	var fakeTime string

	csaml.TimeNow = func() time.Time {
		now, err := time.Parse(time.RFC3339, fakeTime)
		require.NoError(t, err)

		return now
	}

	// Assertion parse is sensitive to time, signature expects XML to have exactly the same bytes as it was sent by IDP.
	// So we fake time here and add all possible request ids.
	for _, tt := range []struct {
		file       string
		rootURL    string
		time       string
		shouldFail bool
	}{
		{
			file:    "google",
			rootURL: "https://77.108.97.212:8099/",
			time:    "2023-06-01T16:20:13.346Z",
		},
		{
			file:    "microsoft",
			rootURL: "https://localhost:8099/",
			time:    "2023-06-01T16:14:13.346Z",
		},
		{
			file:    "samlsp",
			rootURL: "https://localhost:8099/",
			time:    "2023-06-01T16:14:13.346Z",
		},
	} {
		t.Run(tt.file, func(t *testing.T) {
			fakeTime = tt.time
			csaml.Clock = dsig.NewFakeClockAt(csaml.TimeNow())

			rootURL, err := url.Parse(tt.rootURL)
			require.NoError(t, err)

			d, err := os.ReadFile("testdata/" + tt.file + "_metadata.xml")
			require.NoError(t, err)

			idpMetadata, err := samlsp.ParseMetadata(d)
			require.NoError(t, err)

			opts := samlsp.Options{
				URL:         *rootURL,
				IDPMetadata: idpMetadata,
			}

			sp := samlsp.DefaultServiceProvider(opts)

			d, err = os.ReadFile("testdata/" + tt.file + "_acs.xml")
			require.NoError(t, err)

			assertion, err := sp.ParseXMLResponse(d, []string{
				"id-2837ca5976dd42731472c4d4da0c953603232b9f",
				"id-3809fc8de18772f24b29629342ea4b91d6a5cadc",
				"id-ebe26e0275903436e5a2c334d90f3e953985fd75",
			}, *rootURL)
			require.NoError(t, err)

			user, err := saml.LocateUserInfo(assertion, nil)

			if tt.shouldFail {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, user.Identity)
			require.NotEmpty(t, user.Fullname)
		})
	}
}

func TestReadLabelsFromAssertion(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	s := state.WrapCore(namespaced.NewState(inmem.Build))

	sp := saml.NewSessionProvider(s, nil, zaptest.NewLogger(t), map[string]string{
		"identity": saml.IdentityAttribute,
	}, "")

	authConfig := auth.NewAuthConfig()
	authConfig.TypedSpec().Value.Saml = &specs.AuthConfigSpec_SAML{
		LabelRules: map[string]string{
			"custom": "groups",
		},
	}

	require.NoError(t, s.Create(ctx, authConfig))

	for _, tt := range []struct {
		expectedLabels map[string]string
		assertion      *csaml.Assertion
		name           string
	}{
		{
			name: "simple",
			expectedLabels: map[string]string{
				auth.SAMLLabelPrefix + "role/admin": "",
			},
			assertion: &csaml.Assertion{
				AttributeStatements: []csaml.AttributeStatement{
					{
						Attributes: []csaml.Attribute{
							{
								FriendlyName: "Role",
								Values: []csaml.AttributeValue{
									{
										Value: "admin",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multivalue",
			expectedLabels: map[string]string{
				auth.SAMLLabelPrefix + "role/admin":      "",
				auth.SAMLLabelPrefix + "role/superadmin": "",
			},
			assertion: &csaml.Assertion{
				AttributeStatements: []csaml.AttributeStatement{
					{
						Attributes: []csaml.Attribute{
							{
								FriendlyName: "Role",
								Values: []csaml.AttributeValue{
									{
										Value: "admin",
									},
									{
										Value: "superadmin",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "with custom rules",
			expectedLabels: map[string]string{
				auth.SAMLLabelPrefix + "groups/admins":      "",
				auth.SAMLLabelPrefix + "groups/superadmins": "",
			},
			assertion: &csaml.Assertion{
				AttributeStatements: []csaml.AttributeStatement{
					{
						Attributes: []csaml.Attribute{
							{
								Name: "custom",
								Values: []csaml.AttributeValue{
									{
										Value: "admins",
									},
									{
										Value: "superadmins",
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			labels, err := sp.ReadLabelsFromAssertion(ctx, tt.assertion)

			require.NoError(t, err)
			require.EqualValues(t, tt.expectedLabels, labels)
		})
	}
}

func TestRoleInSAMLLabelRules(t *testing.T) {
	logger := zaptest.NewLogger(t)

	operatorRoleToDeveloper := auth.NewSAMLLabelRule("assign-operator-to-developer")

	operatorRoleToDeveloper.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/developer"}
	operatorRoleToDeveloper.TypedSpec().Value.AssignRole = string(role.Operator)

	readerRoleToDeveloper := auth.NewSAMLLabelRule("assign-reader-to-developer")

	readerRoleToDeveloper.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/developer"}
	readerRoleToDeveloper.TypedSpec().Value.AssignRole = string(role.Reader)

	adminRoleToManager := auth.NewSAMLLabelRule("assign-admin-to-manager")

	adminRoleToManager.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/manager"}
	adminRoleToManager.TypedSpec().Value.AssignRole = string(role.Admin)

	invalidRoleToFoobar := auth.NewSAMLLabelRule("assign-invalid-role-to-foobar")

	invalidRoleToFoobar.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/foobar"}
	invalidRoleToFoobar.TypedSpec().Value.AssignRole = "invalid-role"

	// match the role in the rules with the highest access level

	matchedRole := saml.MatchSAMLLabelRule(
		[]*auth.SAMLLabelRule{operatorRoleToDeveloper, readerRoleToDeveloper, adminRoleToManager, invalidRoleToFoobar},
		map[string]string{
			"saml.omni.sidero.dev/role/developer": "",
		}, logger,
	)

	require.EqualValues(t, matchedRole.TypedSpec().Value.AssignRole, role.Operator)

	matchedRole = saml.MatchSAMLLabelRule([]*auth.SAMLLabelRule{operatorRoleToDeveloper, invalidRoleToFoobar, adminRoleToManager}, map[string]string{
		"saml.omni.sidero.dev/role/manager": "",
	}, logger)

	require.EqualValues(t, matchedRole.TypedSpec().Value.AssignRole, role.Admin)

	// if the role in the rule is invalid, log it and return None

	matchedRole = saml.MatchSAMLLabelRule([]*auth.SAMLLabelRule{invalidRoleToFoobar}, map[string]string{
		"saml.omni.sidero.dev/role/foobar": "",
	}, logger)

	require.Nil(t, matchedRole)
}

func TestRoleCompare(t *testing.T) {
	require.Equal(t, 0, role.Admin.Compare(role.Admin)) //nolint:gocritic

	require.Equal(t, 1, role.Admin.Compare(role.None))

	require.Equal(t, -1, role.Operator.Compare(role.Admin))
}

func TestMatchSAMLLabelRuleNone(t *testing.T) {
	logger := zaptest.NewLogger(t)

	noneRoleToInactive := auth.NewSAMLLabelRule("assign-none-to-inactive")
	noneRoleToInactive.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/omni-none"}
	noneRoleToInactive.TypedSpec().Value.AssignRole = string(role.None)
	noneRoleToInactive.TypedSpec().Value.UpdateOnEachLogin = true

	// None role rule should be returned (not nil)
	matchedRule := saml.MatchSAMLLabelRule(
		[]*auth.SAMLLabelRule{noneRoleToInactive},
		map[string]string{
			"saml.omni.sidero.dev/role/omni-none": "",
		}, logger,
	)

	require.NotNil(t, matchedRule)
	require.EqualValues(t, matchedRule.TypedSpec().Value.AssignRole, role.None)
	require.True(t, matchedRule.TypedSpec().Value.UpdateOnEachLogin)
}

func TestMatchSAMLLabelRuleNoneWithHigherRulePresent(t *testing.T) {
	logger := zaptest.NewLogger(t)

	noneRule := auth.NewSAMLLabelRule("assign-none")
	noneRule.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/omni-none"}
	noneRule.TypedSpec().Value.AssignRole = string(role.None)

	readerRule := auth.NewSAMLLabelRule("assign-reader")
	readerRule.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/omni-reader"}
	readerRule.TypedSpec().Value.AssignRole = string(role.Reader)

	// User with only None label → returns None rule
	matchedRule := saml.MatchSAMLLabelRule(
		[]*auth.SAMLLabelRule{noneRule, readerRule},
		map[string]string{
			"saml.omni.sidero.dev/role/omni-none": "",
		}, logger,
	)

	require.NotNil(t, matchedRule)
	require.EqualValues(t, matchedRule.TypedSpec().Value.AssignRole, role.None)

	// User with both labels → Reader wins (highest role)
	matchedRule = saml.MatchSAMLLabelRule(
		[]*auth.SAMLLabelRule{noneRule, readerRule},
		map[string]string{
			"saml.omni.sidero.dev/role/omni-none":   "",
			"saml.omni.sidero.dev/role/omni-reader": "",
		}, logger,
	)

	require.NotNil(t, matchedRule)
	require.EqualValues(t, matchedRule.TypedSpec().Value.AssignRole, role.Reader)
}

const (
	lockedOutEmail = "locked-out@example.com"
	colleagueEmail = "colleague@example.com"
	developerLabel = "saml.omni.sidero.dev/role/developer"
)

// setupEnsureUser builds a state with two Admin users and one label rule, so ensureUser takes the
// "users already exist" branch and resolves a rule instead of bootstrapping the first admin.
func setupEnsureUser(ctx context.Context, t *testing.T, assignRole role.Role, updateOnEachLogin bool) state.State {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	for _, email := range []string{lockedOutEmail, colleagueEmail} {
		require.NoError(t, user.Ensure(ctx, st, email, role.Admin, false))
	}

	rule := auth.NewSAMLLabelRule("assign-to-developer")
	rule.TypedSpec().Value.MatchLabels = []string{developerLabel}
	rule.TypedSpec().Value.AssignRole = string(assignRole)
	rule.TypedSpec().Value.UpdateOnEachLogin = updateOnEachLogin

	require.NoError(t, st.Create(ctx, rule))

	return st
}

func roleOf(ctx context.Context, t *testing.T, st state.State, email string) string {
	t.Helper()

	identity, err := safe.StateGetByID[*auth.Identity](ctx, st, email)
	require.NoError(t, err)

	usr, err := safe.StateGetByID[*auth.User](ctx, st, identity.TypedSpec().Value.UserId)
	require.NoError(t, err)

	return usr.TypedSpec().Value.Role
}

func TestEnsureUserRecoveryAdmin(t *testing.T) {
	t.Parallel()

	samlLabels := map[string]string{developerLabel: ""}

	for _, tt := range []struct {
		name              string
		recoveryAdmin     string
		assignRole        role.Role
		expectedRole      role.Role
		updateOnEachLogin bool
	}{
		{
			name:              "recovery admin is not demoted",
			recoveryAdmin:     lockedOutEmail,
			assignRole:        role.Reader,
			updateOnEachLogin: true,
			expectedRole:      role.Admin,
		},
		{
			name:              "recovery admin is not stripped of every role",
			recoveryAdmin:     lockedOutEmail,
			assignRole:        role.None,
			updateOnEachLogin: true,
			expectedRole:      role.Admin,
		},
		{
			name:              "recovery admin still matches a rule that assigns Admin",
			recoveryAdmin:     lockedOutEmail,
			assignRole:        role.Admin,
			updateOnEachLogin: true,
			expectedRole:      role.Admin,
		},
		{
			name:              "email is matched case-insensitively",
			recoveryAdmin:     "Locked-Out@Example.com",
			assignRole:        role.Reader,
			updateOnEachLogin: true,
			expectedRole:      role.Admin,
		},
		{
			name:              "everyone else is still demoted",
			assignRole:        role.Reader,
			updateOnEachLogin: true,
			expectedRole:      role.Reader,
		},
		{
			name:          "a rule without updateOnEachLogin changes nobody",
			recoveryAdmin: lockedOutEmail,
			assignRole:    role.Reader,
			expectedRole:  role.Admin,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := setupEnsureUser(ctx, t, tt.assignRole, tt.updateOnEachLogin)

			sp := saml.NewSessionProvider(st, nil, zaptest.NewLogger(t), nil, tt.recoveryAdmin)

			require.NoError(t, sp.EnsureUser(ctx, lockedOutEmail, samlLabels))

			require.EqualValues(t, tt.expectedRole, roleOf(ctx, t, st, lockedOutEmail))

			// the guard only covers the configured email, everyone else keeps what they had
			require.EqualValues(t, role.Admin, roleOf(ctx, t, st, colleagueEmail))
		})
	}
}

// TestEnsureUserRecoveryAdminNotCreated checks that a recovery admin with no identity yet is registered
// with the role its label rule assigns. Elevation happens on the next restart, not here.
func TestEnsureUserRecoveryAdminNotCreated(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := setupEnsureUser(ctx, t, role.Reader, true)

	const newcomer = "newcomer@example.com"

	sp := saml.NewSessionProvider(st, nil, zaptest.NewLogger(t), nil, newcomer)

	require.NoError(t, sp.EnsureUser(ctx, newcomer, map[string]string{developerLabel: ""}))

	require.EqualValues(t, role.Reader, roleOf(ctx, t, st, newcomer))
}
