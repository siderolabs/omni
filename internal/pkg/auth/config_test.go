// Copyright (c) 2025 Sidero Labs, Inc.
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
			initialConfig: config.Auth{
				Auth0: config.Auth0{
					Enabled:  true,
					ClientID: "client-id",
					Domain:   "domain",
				},
			},
			expected: &specs.AuthConfigSpec{
				Auth0: &specs.AuthConfigSpec_Auth0{
					Enabled:  true,
					ClientId: "client-id",
					Domain:   "domain",
				},
				Webauthn: &specs.AuthConfigSpec_Webauthn{},
				Saml:     &specs.AuthConfigSpec_SAML{},
			},
		},
		{
			name: "enable webauthn",
			initialConfig: config.Auth{
				WebAuthn: config.WebAuthn{
					Enabled: true,
				},
			},
			expected: &specs.AuthConfigSpec{
				Webauthn: &specs.AuthConfigSpec_Webauthn{
					Enabled: true,
				},
				Auth0: &specs.AuthConfigSpec_Auth0{},
				Saml:  &specs.AuthConfigSpec_SAML{},
			},
		},
		{
			name: "make webauthn not required",
			initialConfig: config.Auth{
				WebAuthn: config.WebAuthn{
					Enabled:  true,
					Required: true,
				},
			},
			updatedConfig: &config.Auth{
				WebAuthn: config.WebAuthn{
					Enabled:  true,
					Required: false,
				},
			},
			expected: &specs.AuthConfigSpec{
				Webauthn: &specs.AuthConfigSpec_Webauthn{
					Enabled: true,
				},
				Auth0: &specs.AuthConfigSpec_Auth0{},
				Saml:  &specs.AuthConfigSpec_SAML{},
			},
		},
		{
			name: "fail to disable webauthn",
			initialConfig: config.Auth{
				WebAuthn: config.WebAuthn{
					Enabled:  true,
					Required: true,
				},
			},
			updatedConfig: &config.Auth{
				WebAuthn: config.WebAuthn{
					Enabled: false,
				},
			},
			expectUpdateError: true,
		},
		{
			name: "fail to disable auth0",
			initialConfig: config.Auth{
				Auth0: config.Auth0{
					Enabled:  true,
					ClientID: "client-id",
					Domain:   "domain",
				},
			},
			updatedConfig:     &config.Auth{},
			expectUpdateError: true,
		},
		{
			name: "switch from auth0 to SAML",
			initialConfig: config.Auth{
				Auth0: config.Auth0{
					Enabled:  true,
					ClientID: "client-id",
					Domain:   "domain",
				},
			},
			updatedConfig: &config.Auth{
				SAML: config.SAML{
					Enabled:     true,
					MetadataURL: "http://samltest.sp/idp",
				},
			},
			expected: &specs.AuthConfigSpec{
				Auth0:    &specs.AuthConfigSpec_Auth0{},
				Webauthn: &specs.AuthConfigSpec_Webauthn{},
				Saml: &specs.AuthConfigSpec_SAML{
					Enabled: true,
					Url:     "http://samltest.sp/idp",
				},
			},
		},
		{
			name: "fail to disable SAML",
			initialConfig: config.Auth{
				SAML: config.SAML{
					Enabled:     true,
					MetadataURL: "http://samltest.sp/idp",
				},
			},
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
