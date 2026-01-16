// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestEnsureAuthConfigResource(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	logger := zaptest.NewLogger(t).With(logging.Component("auth"))

	for _, tt := range []struct { //nolint:govet
		name              string
		initialConfig     config.Auth
		updatedConfig     *config.Auth
		expected          *specs.AuthConfigSpec
		expectInitError   bool
		expectUpdateError bool
	}{
		{
			name:            "empty",
			expectInitError: true,
		},
		{
			name: "enable auth0",
			initialConfig: func() config.Auth {
				var c config.Auth
				c.Auth0.SetEnabled(true)
				c.Auth0.SetClientID("client-id")
				c.Auth0.SetDomain("domain")

				return c
			}(),
			expected: &specs.AuthConfigSpec{
				Auth0: &specs.AuthConfigSpec_Auth0{
					Enabled:  true,
					ClientId: "client-id",
					Domain:   "domain",
				},
				Webauthn: &specs.AuthConfigSpec_Webauthn{},
				Saml:     &specs.AuthConfigSpec_SAML{},
				Oidc:     &specs.AuthConfigSpec_OIDC{},
			},
		},
		{
			name: "enable webauthn",
			initialConfig: func() config.Auth {
				var c config.Auth
				c.Webauthn.SetEnabled(true)

				return c
			}(),
			expected: &specs.AuthConfigSpec{
				Webauthn: &specs.AuthConfigSpec_Webauthn{
					Enabled: true,
				},
				Auth0: &specs.AuthConfigSpec_Auth0{},
				Saml:  &specs.AuthConfigSpec_SAML{},
				Oidc:  &specs.AuthConfigSpec_OIDC{},
			},
		},
		{
			name: "make webauthn not required",
			initialConfig: func() config.Auth {
				var c config.Auth
				c.Webauthn.SetEnabled(true)
				c.Webauthn.SetRequired(true)

				return c
			}(),
			updatedConfig: func() *config.Auth {
				var c config.Auth
				c.Webauthn.SetEnabled(true)
				c.Webauthn.SetRequired(false)

				return &c
			}(),
			expected: &specs.AuthConfigSpec{
				Webauthn: &specs.AuthConfigSpec_Webauthn{
					Enabled: true,
				},
				Auth0: &specs.AuthConfigSpec_Auth0{},
				Saml:  &specs.AuthConfigSpec_SAML{},
				Oidc:  &specs.AuthConfigSpec_OIDC{},
			},
		},
		{
			name: "fail to disable webauthn",
			initialConfig: func() config.Auth {
				var c config.Auth
				c.Webauthn.SetEnabled(true)
				c.Webauthn.SetRequired(true)

				return c
			}(),
			updatedConfig: func() *config.Auth {
				var c config.Auth
				c.Webauthn.SetEnabled(false)

				return &c
			}(),
			expectUpdateError: true,
		},
		{
			name: "fail to disable auth0",
			initialConfig: func() config.Auth {
				var c config.Auth
				c.Auth0.SetEnabled(true)
				c.Auth0.SetClientID("client-id")
				c.Auth0.SetDomain("domain")

				return c
			}(),
			updatedConfig:     &config.Auth{},
			expectUpdateError: true,
		},
		{
			name: "switch from auth0 to SAML",
			initialConfig: func() config.Auth {
				var c config.Auth
				c.Auth0.SetEnabled(true)
				c.Auth0.SetClientID("client-id")
				c.Auth0.SetDomain("domain")

				return c
			}(),
			updatedConfig: func() *config.Auth {
				var c config.Auth
				c.Saml.SetEnabled(true)
				c.Saml.SetUrl("http://samltest.sp/idp")

				return &c
			}(),
			expected: &specs.AuthConfigSpec{
				Auth0:    &specs.AuthConfigSpec_Auth0{},
				Webauthn: &specs.AuthConfigSpec_Webauthn{},
				Saml: &specs.AuthConfigSpec_SAML{
					Enabled: true,
					Url:     "http://samltest.sp/idp",
				},
				Oidc: &specs.AuthConfigSpec_OIDC{},
			},
		},
		{
			name: "fail to disable SAML",
			initialConfig: func() config.Auth {
				var c config.Auth
				c.Saml.SetEnabled(true)
				c.Saml.SetUrl("http://samltest.sp/idp")

				return c
			}(),
			updatedConfig:     &config.Auth{},
			expectUpdateError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			authConfig, err := auth.EnsureAuthConfigResource(ctx, st, logger, tt.initialConfig)
			if tt.expectInitError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			if tt.updatedConfig != nil {
				authConfig, err = auth.EnsureAuthConfigResource(ctx, st, logger, *tt.updatedConfig)

				if tt.expectUpdateError {
					require.Error(t, err)

					return
				}

				require.NoError(t, err)
			}

			assert.True(t, proto.Equal(authConfig.TypedSpec().Value, tt.expected), "expected %v, got %v", tt.expected, authConfig.TypedSpec().Value)
		})
	}
}
