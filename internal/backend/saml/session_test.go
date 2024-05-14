// Copyright (c) 2024 Sidero Labs, Inc.
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

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	csaml "github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/saml"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
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
			})
			require.NoError(t, err)

			user, err := saml.LocateUserInfo(assertion)

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	s := state.WrapCore(namespaced.NewState(inmem.Build))

	sp := saml.NewSessionProvider(s, nil, zaptest.NewLogger(t))

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

	operatorRoleToDeveloper := auth.NewSAMLLabelRule(resources.DefaultNamespace, "assign-operator-to-developer")

	operatorRoleToDeveloper.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/developer"}
	operatorRoleToDeveloper.TypedSpec().Value.AssignRoleOnRegistration = string(role.Operator)

	readerRoleToDeveloper := auth.NewSAMLLabelRule(resources.DefaultNamespace, "assign-reader-to-developer")

	readerRoleToDeveloper.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/developer"}
	readerRoleToDeveloper.TypedSpec().Value.AssignRoleOnRegistration = string(role.Reader)

	adminRoleToManager := auth.NewSAMLLabelRule(resources.DefaultNamespace, "assign-admin-to-manager")

	adminRoleToManager.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/manager"}
	adminRoleToManager.TypedSpec().Value.AssignRoleOnRegistration = string(role.Admin)

	invalidRoleToFoobar := auth.NewSAMLLabelRule(resources.DefaultNamespace, "assign-invalid-role-to-foobar")

	invalidRoleToFoobar.TypedSpec().Value.MatchLabels = []string{"saml.omni.sidero.dev/role/foobar"}
	invalidRoleToFoobar.TypedSpec().Value.AssignRoleOnRegistration = "invalid-role"

	// match the role in the rules with the highest access level

	matchedRole := saml.RoleInSAMLLabelRules(
		[]*auth.SAMLLabelRule{operatorRoleToDeveloper, readerRoleToDeveloper, adminRoleToManager, invalidRoleToFoobar},
		map[string]string{
			"saml.omni.sidero.dev/role/developer": "",
		}, logger)

	require.Equal(t, matchedRole, role.Operator)

	matchedRole = saml.RoleInSAMLLabelRules([]*auth.SAMLLabelRule{operatorRoleToDeveloper, invalidRoleToFoobar, adminRoleToManager}, map[string]string{
		"saml.omni.sidero.dev/role/manager": "",
	}, logger)

	require.Equal(t, matchedRole, role.Admin)

	// if the role in the rule is invalid, log it and return None

	matchedRole = saml.RoleInSAMLLabelRules([]*auth.SAMLLabelRule{invalidRoleToFoobar}, map[string]string{
		"saml.omni.sidero.dev/role/foobar": "",
	}, logger)

	require.Equal(t, matchedRole, role.None)
}
