// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package config contains the application config loading functions.
package config_test

import (
	"context"
	_ "embed"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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
				Default: &config.StorageDefault{
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

func TestValidateStateConfig(t *testing.T) {
	state := state.WrapCore(namespaced.NewState(inmem.Build))

	cfg, err := config.FromBytes(configFull)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	// no machines

	assert.NoError(t, cfg.ValidateState(ctx, state))

	// fail with machines below 1.6, pass with above 1.6

	machine := omni.NewMachineStatus(resources.DefaultNamespace, "1")
	machine.TypedSpec().Value.TalosVersion = "v1.5.5"

	require.NoError(t, state.Create(ctx, machine))

	assert.Error(t, cfg.ValidateState(ctx, state))

	_, err = safe.StateUpdateWithConflicts(ctx, state, machine.Metadata(), func(res *omni.MachineStatus) error {
		res.TypedSpec().Value.TalosVersion = "v1.6.0"

		return nil
	})

	require.NoError(t, err)

	assert.NoError(t, cfg.ValidateState(ctx, state))
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
