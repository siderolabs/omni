// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package config contains the application config loading functions.
package config_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/pkg/config"
)

//go:embed testdata/config-full.yaml
var configFull []byte

//go:embed testdata/invalid-join-token-mode.yaml
var configInvalidJoinTokenMode []byte

//go:embed testdata/conflicting-auth.yaml
var conflictingAuth []byte

//go:embed testdata/backups.yaml
var backups []byte

//go:embed testdata/unknown-keys.yaml
var unknownKeys []byte

//go:embed testdata/config-no-tls-certs.yaml
var configNoTLSCerts []byte

//go:embed testdata/enable-saml.yaml
var enableSAML []byte

func TestMergeConfig(t *testing.T) {
	cfg, err := config.Init(zaptest.NewLogger(t),
		&config.Params{
			Services: config.Services{
				API: &config.Service{
					BindEndpoint: "0.0.0.0:80",
					CertFile:     "crt",
					KeyFile:      "key",
				},
			},
			Storage: config.Storage{
				Default: config.StorageDefault{
					Etcd: config.EtcdParams{
						PrivateKeySource: "vault",
					},
				},
			},
		},
	)

	require.NoError(t, err)
	require.True(t, cfg.Services.EmbeddedDiscoveryService.Enabled)
}

func TestValidateConfig(t *testing.T) {
	for _, tt := range []struct {
		name        string
		validateErr string
		loadErr     string
		config      []byte
	}{
		{
			name:        "empty",
			config:      []byte("{}"),
			validateErr: "required",
		},
		{
			name:   "full",
			config: configFull,
		},
		{
			name:        "invalid join tokens mode",
			config:      configInvalidJoinTokenMode,
			validateErr: "JoinTokensMode",
		},
		{
			name:        "conflicting auth",
			config:      conflictingAuth,
			validateErr: "mutually exclusive",
		},
		{
			name:        "conflicting backups",
			config:      backups,
			validateErr: "Field validation for 'LocalPath' failed",
		},
		{
			name:    "unknown keys",
			config:  unknownKeys,
			loadErr: "unknown keys found",
		},
		{
			// Having no TLS cert/key neither for the API nor for Kubernetes Proxy Server is NOT an error,
			// as Omni might be running behind a reverse proxy that handles the TLS termination.
			name:   "no tls certs",
			config: configNoTLSCerts,
		},
		{
			name:   "SAML with initial users",
			config: enableSAML,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.FromBytes(tt.config)
			if tt.loadErr != "" {
				require.ErrorContains(t, err, tt.loadErr)

				return
			}

			require.NoError(t, err)

			err = cfg.Validate()
			if tt.validateErr != "" {
				require.ErrorContains(t, err, tt.validateErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
