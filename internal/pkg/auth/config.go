// Copyright (c) 2026 Sidero Labs, Inc.
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

		res.TypedSpec().Value.Auth0.Enabled = authParams.Auth0.GetEnabled()
		res.TypedSpec().Value.Auth0.Domain = authParams.Auth0.GetDomain()
		res.TypedSpec().Value.Auth0.ClientId = authParams.Auth0.GetClientID()
		res.TypedSpec().Value.Auth0.UseFormData = authParams.Auth0.GetUseFormData()
		res.TypedSpec().Value.Saml.Enabled = authParams.Saml.GetEnabled()
		res.TypedSpec().Value.Saml.Url = authParams.Saml.GetUrl()
		res.TypedSpec().Value.Saml.Metadata = authParams.Saml.GetMetadata()
		res.TypedSpec().Value.Saml.LabelRules = authParams.Saml.LabelRules
		res.TypedSpec().Value.Saml.AttributeRules = authParams.Saml.AttributeRules
		res.TypedSpec().Value.Saml.NameIdFormat = authParams.Saml.GetNameIDFormat()

		res.TypedSpec().Value.Oidc.Enabled = authParams.Oidc.GetEnabled()
		res.TypedSpec().Value.Oidc.ClientId = authParams.Oidc.GetClientID()
		res.TypedSpec().Value.Oidc.ClientSecret = authParams.Oidc.GetClientSecret()
		res.TypedSpec().Value.Oidc.ProviderUrl = authParams.Oidc.GetProviderURL()
		res.TypedSpec().Value.Oidc.Scopes = authParams.Oidc.Scopes

		webauthnEnabled := authParams.Webauthn.GetEnabled()
		if res.TypedSpec().Value.Webauthn.Enabled && !webauthnEnabled {
			logger.Warn("webauthn is disabled in Config, but enabled in the cluster, refusing to disable it",
				zap.String("resource", confPtr.ID()),
			)
		} else {
			res.TypedSpec().Value.Webauthn.Enabled = webauthnEnabled
			res.TypedSpec().Value.Webauthn.Required = authParams.Webauthn.GetRequired()
		}

		res.TypedSpec().Value.Suspended = authParams.GetSuspended()
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
			zap.Any("webauthn", authParams.Webauthn),
			zap.Any("saml", authParams.Saml),
			zap.Any("oidc", authParams.Oidc),
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
			zap.Any("webauthn", authParams.Webauthn),
			zap.Any("saml", authParams.Saml),
			zap.Any("oidc", authParams.Oidc),
		)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update Config resource: %w", err)
	}

	return authConfig, nil
}

func validateParams(authParams config.Auth) error {
	samlEnabled := authParams.Saml.GetEnabled()
	auth0Enabled := authParams.Auth0.GetEnabled()
	webauthnEnabled := authParams.Webauthn.GetEnabled()

	oidcEnabled := authParams.Oidc.GetEnabled()
	if !samlEnabled && !auth0Enabled && !webauthnEnabled && !oidcEnabled {
		return errors.New("no authentication is enabled")
	}

	enabledAuths := make([]string, 0, 3)

	for auth, enabled := range map[string]bool{
		"auth0": auth0Enabled,
		"SAML":  samlEnabled,
		"OIDC":  oidcEnabled,
	} {
		if !enabled {
			continue
		}

		enabledAuths = append(enabledAuths, auth)
	}

	if len(enabledAuths) > 1 {
		return fmt.Errorf("several auth methods are enabled: %s, only one can be enabled at the same time", strings.Join(enabledAuths, ", "))
	}

	if samlEnabled && authParams.Saml.GetUrl() == "" && authParams.Saml.GetMetadata() == "" {
		return errors.New("SAML is enabled but neither URL nor metadata is set")
	}

	if auth0Enabled {
		if authParams.Auth0.GetDomain() == "" {
			return errors.New("auth0 is enabled but its domain is not set")
		}

		if authParams.Auth0.GetClientID() == "" {
			return errors.New("auth0 is enabled but its client id is not set")
		}
	}

	if oidcEnabled {
		if authParams.Oidc.GetClientID() == "" {
			return errors.New("OIDC is enabled by it's client id is not set")
		}

		if authParams.Oidc.GetClientSecret() == "" {
			return errors.New("OIDC is enabled by it's client secret is not set")
		}

		if authParams.Oidc.GetProviderURL() == "" {
			return errors.New("OIDC is enabled by it's provider URL is not set")
		}
	}

	return nil
}
