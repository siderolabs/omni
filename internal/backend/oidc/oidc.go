// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package oidc implements OIDC server.
package oidc

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/sirupsen/logrus"
	oidc_logging "github.com/zitadel/logging"
	"github.com/zitadel/oidc/pkg/op"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func init() {
	oidc_logging.SetFormatter(&logrus.JSONFormatter{})
}

// NewStorage creates OIDC internal storage.
func NewStorage(st state.State, logger *zap.Logger) *storage.Storage {
	return storage.NewStorage(st, logger)
}

// Provider combines OIDC provider and storage.
type Provider struct {
	op.OpenIDProvider

	storage *storage.Storage
}

// NewProvider creates new OIDC provider.
func NewProvider(ctx context.Context, store *storage.Storage) (*Provider, error) {
	issuerEndpoint, err := config.Config.GetOIDCIssuerEndpoint()
	if err != nil {
		return nil, err
	}

	// generate fresh crypto key time, as all auth requests are ephemeral in the storage
	var cryptoKey [32]byte

	_, err = rand.Read(cryptoKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to generate crypto key: %w", err)
	}

	config := &op.Config{
		Issuer:                   issuerEndpoint,
		CryptoKey:                cryptoKey,
		DefaultLogoutRedirectURI: "/logout",
		CodeMethodS256:           true,
		AuthMethodPost:           true,
		AuthMethodPrivateKeyJWT:  true,
		GrantTypeRefreshToken:    false,
		RequestObjectSupported:   true,
	}

	if constants.IsDebugBuild {
		// allow HTTP in OIDC issuer endpoint
		os.Setenv(op.OidcDevMode, "true") //nolint:errcheck
	}

	h, err := op.NewOpenIDProvider(ctx, config, store)
	if err != nil {
		return nil, err
	}

	return &Provider{
		OpenIDProvider: h,
		storage:        store,
	}, nil
}

// AuthenticateRequest authenticates OIDC request.
func (p *Provider) AuthenticateRequest(requestID, identity string) error {
	return p.storage.AuthenticateRequest(requestID, identity)
}
