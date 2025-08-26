// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// EnsureAuthConfigResource creates/configures the auth config resource.
func EnsureAuthConfigResource(ctx context.Context, st state.State, logger *zap.Logger, authParams config.Auth) (*auth.Config, error) {
	err := validateParams(authParams)
	if err != nil {
		return nil, err
	}

	confPtr := auth.NewAuthConfig().Metadata()

	setConfig := func(res *auth.Config) {
		if res.TypedSpec().Value.Auth0 == nil {
			res.TypedSpec().Value.Auth0 = &specs.AuthConfigSpec_Auth0{}
		}

		if res.TypedSpec().Value.Saml == nil {
			res.TypedSpec().Value.Saml = &specs.AuthConfigSpec_SAML{}
		}

		if res.TypedSpec().Value.Webauthn == nil {
			res.TypedSpec().Value.Webauthn = &specs.AuthConfigSpec_Webauthn{}
		}

		if res.TypedSpec().Value.Oidc == nil {
			res.TypedSpec().Value.Oidc = &specs.AuthConfigSpec_OIDC{}
		}

		res.TypedSpec().Value.Auth0.Enabled = authParams.Auth0.Enabled
		res.TypedSpec().Value.Auth0.Domain = authParams.Auth0.Domain
		res.TypedSpec().Value.Auth0.ClientId = authParams.Auth0.ClientID
		res.TypedSpec().Value.Auth0.UseFormData = authParams.Auth0.UseFormData
		res.TypedSpec().Value.Saml.Enabled = authParams.SAML.Enabled
		res.TypedSpec().Value.Saml.Url = authParams.SAML.MetadataURL
		res.TypedSpec().Value.Saml.Metadata = authParams.SAML.Metadata
		res.TypedSpec().Value.Saml.LabelRules = authParams.SAML.LabelRules
		res.TypedSpec().Value.Saml.AttributeRules = authParams.SAML.AttributeRules
		res.TypedSpec().Value.Saml.NameIdFormat = authParams.SAML.NameIDFormat

		res.TypedSpec().Value.Oidc.Enabled = authParams.OIDC.Enabled
		res.TypedSpec().Value.Oidc.ClientId = authParams.OIDC.ClientID
		res.TypedSpec().Value.Oidc.ClientSecret = authParams.OIDC.ClientSecret
		res.TypedSpec().Value.Oidc.ProviderUrl = authParams.OIDC.ProviderURL
		res.TypedSpec().Value.Oidc.Scopes = authParams.OIDC.Scopes

		if res.TypedSpec().Value.Webauthn.Enabled && !authParams.WebAuthn.Enabled {
			logger.Warn("webauthn is disabled in Config, but enabled in the cluster, refusing to disable it",
				zap.String("resource", confPtr.ID()),
			)
		} else {
			res.TypedSpec().Value.Webauthn.Enabled = authParams.WebAuthn.Enabled
			res.TypedSpec().Value.Webauthn.Required = authParams.WebAuthn.Required
		}

		res.TypedSpec().Value.Suspended = authParams.Suspended
	}

	_, err = safe.StateGet[*auth.Config](ctx, st, confPtr)
	if state.IsNotFoundError(err) {
		authConfig := auth.NewAuthConfig()
		setConfig(authConfig)

		err = st.Create(ctx, authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Config resource: %w", err)
		}

		logger.Info("created Config resource",
			zap.String("resource", confPtr.ID()),
			zap.Any("auth0", authParams.Auth0),
			zap.Any("webauthn", authParams.WebAuthn),
			zap.Any("saml", authParams.SAML),
			zap.Any("oidc", authParams.OIDC),
		)

		return authConfig, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get Config resource: %w", err)
	}

	authConfig, err := safe.StateUpdateWithConflicts(ctx, st, confPtr, func(res *auth.Config) error {
		setConfig(res)

		logger.Info("updated Config resource",
			zap.String("resource", confPtr.ID()),
			zap.Any("auth0", authParams.Auth0),
			zap.Any("webauthn", authParams.WebAuthn),
			zap.Any("saml", authParams.SAML),
			zap.Any("oidc", authParams.OIDC),
		)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update Config resource: %w", err)
	}

	return authConfig, nil
}

func validateParams(authParams config.Auth) error {
	if !authParams.SAML.Enabled && !authParams.Auth0.Enabled && !authParams.WebAuthn.Enabled && !authParams.OIDC.Enabled {
		return errors.New("no authentication is enabled")
	}

	enabledAuths := make([]string, 0, 3)

	for auth, enabled := range map[string]bool{
		"auth0": authParams.Auth0.Enabled,
		"SAML":  authParams.SAML.Enabled,
		"OIDC":  authParams.OIDC.Enabled,
	} {
		if !enabled {
			continue
		}

		enabledAuths = append(enabledAuths, auth)
	}

	if len(enabledAuths) > 1 {
		return fmt.Errorf("several auth methods are enabled: %s, only one can be enabled at the same time", strings.Join(enabledAuths, ", "))
	}

	if authParams.SAML.Enabled && authParams.SAML.MetadataURL == "" && authParams.SAML.Metadata == "" {
		return errors.New("SAML is enabled but neither URL nor metadata is set")
	}

	if authParams.Auth0.Enabled {
		if authParams.Auth0.Domain == "" {
			return errors.New("auth0 is enabled but its domain is not set")
		}

		if authParams.Auth0.ClientID == "" {
			return errors.New("auth0 is enabled but its client id is not set")
		}
	}

	if authParams.OIDC.Enabled {
		if authParams.OIDC.ClientID == "" {
			return errors.New("OIDC is enabled by it's client id is not set")
		}

		if authParams.OIDC.ClientSecret == "" {
			return errors.New("OIDC is enabled by it's client secret is not set")
		}

		if authParams.OIDC.ProviderURL == "" {
			return errors.New("OIDC is enabled by it's provider URL is not set")
		}
	}

	return nil
}
