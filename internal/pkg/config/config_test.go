// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

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
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

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
	params := &config.Params{}
	params.Services.Api.SetEndpoint("0.0.0.0:80")
	params.Services.Api.SetCertFile("crt")
	params.Services.Api.SetKeyFile("key")
	params.Storage.Default.Etcd.SetPrivateKeySource("vault")
	params.Storage.Sqlite.SetPath("/some/path")
	params.Logs.Audit.SetSqliteTimeout(5 * time.Second)

	cfg, err := config.Init(zaptest.NewLogger(t), params)

	require.NoError(t, err)
	assert.True(t, cfg.Services.EmbeddedDiscoveryService.GetEnabled())
	assert.Equal(t, 5*time.Second, cfg.Logs.Audit.GetSqliteTimeout())

	// assert that the default value of cfg.Logs.Audit.Enabled boolean value is preserved even when the config.LogsAudit struct was partially set as an override
	assert.True(t, cfg.Logs.Audit.GetEnabled())
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

	machine := omni.NewMachineStatus("1")
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
		name             string
		configModifyFunc func(cfg *config.Params)
		validateErr      string
		loadErr          string
		config           []byte
	}{
		{
			name:        "empty",
			config:      []byte("{}"),
			validateErr: "got null, want string",
		},
		{
			name:   "full",
			config: configFull,
		},
		{
			name:   "kubernetes proxy insecure advertised url",
			config: configFull,
			configModifyFunc: func(cfg *config.Params) {
				cfg.Services.KubernetesProxy.SetAdvertisedURL("http://1.1.1.1:1111")
			},
			validateErr: `- at '/services/kubernetesProxy/advertisedURL': 'http://1.1.1.1:1111' does not match pattern '^https://'`,
		},
		{
			name:        "invalid join tokens mode",
			config:      configInvalidJoinTokenMode,
			validateErr: "at '/services/siderolink/joinTokensMode': value must be one of 'strict', 'legacyAllowed', 'legacy'",
		},
		{
			name:        "conflicting auth",
			config:      conflictingAuth,
			validateErr: "at '/auth/saml/enabled': 'not' failed",
		},
		{
			name:        "conflicting backups",
			config:      backups,
			validateErr: "at '/etcdBackup': 'not' failed",
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
		{
			name: "missing sqlite path",
			config: []byte(`storage:
  sqlite: {}`),
			validateErr: "at '/storage/sqlite/path': got null, want string",
		},
		{
			name: "empty sqlite path",
			config: []byte(`storage:
  sqlite:
    path: ""`),
			validateErr: "at '/storage/sqlite/path': minLength: got 0, want 1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.FromBytes(tt.config)
			if tt.loadErr != "" {
				require.ErrorContains(t, err, tt.loadErr)

				return
			}

			if tt.configModifyFunc != nil {
				tt.configModifyFunc(cfg)
			}

			require.NoError(t, err)

			err = cfg.Validate()

			if tt.validateErr != "" {
				var validationErr *jsonschema.ValidationError

				require.ErrorAs(t, err, &validationErr)
				require.ErrorContains(t, err, tt.validateErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestServiceURL(t *testing.T) {
	t.Parallel()

	t.Run("explicit advertised url", func(t *testing.T) {
		t.Parallel()

		conf := &config.DevServerProxyService{
			Endpoint:      pointer.To("1.1.1.1:1111"),
			AdvertisedURL: pointer.To("https://2.2.2.2:2222"),
		}

		url := conf.URL()
		assert.Equal(t, "https://2.2.2.2:2222", url)
	})

	t.Run("secure https", func(t *testing.T) {
		t.Parallel()

		conf := &config.Service{
			Endpoint: pointer.To("1.1.1.1:1111"),
			CertFile: pointer.To("/path/to/cert"),
			KeyFile:  pointer.To("/path/to/key"),
		}

		url := conf.URL()
		assert.Equal(t, "https://1.1.1.1:1111", url)
	})

	t.Run("siderolink api - insecure as grpc", func(t *testing.T) {
		t.Parallel()

		conf := &config.MachineAPI{
			Endpoint: pointer.To("1.1.1.1:1111"),
		}

		url := conf.URL()
		assert.Equal(t, "grpc://1.1.1.1:1111", url, "siderolink api should use grpc schema when no tls")
	})

	t.Run("kubernetes proxy - insecure as https", func(t *testing.T) {
		t.Parallel()

		conf := &config.KubernetesProxyService{
			Endpoint: pointer.To("1.1.1.1:1111"),
		}

		url := conf.URL()

		assert.Equal(t, "https://1.1.1.1:1111", url, "kubernetes proxy should always use https schema")
	})
}
