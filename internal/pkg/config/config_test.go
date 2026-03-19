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

	configSchema, parseErr := config.ParseSchema()
	require.NoError(t, parseErr)

	cfg, err := config.Init(zaptest.NewLogger(t), configSchema, params)

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
	schema, parseErr := config.ParseSchema()
	require.NoError(t, parseErr)

	for _, tt := range []struct {
		name             string
		configModifyFunc func(cfg *config.Params)
		validateErr      string
		loadErr          string
		config           []byte
	}{
		// --- Valid configs ---

		{
			name:   "full",
			config: configFull,
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

		// --- Load errors ---

		{
			name:    "unknown keys",
			config:  unknownKeys,
			loadErr: "unknown keys found",
		},

		// --- type: got null, want string (required field not set) ---

		{
			name:        "type null: empty config",
			config:      []byte("{}"),
			validateErr: `or flag "--sqlite-storage-path": is required but was not set`,
		},
		{
			name: "type null: missing sqlite path",
			config: []byte(`storage:
  sqlite: {}`),
			validateErr: `config value ".storage.sqlite.path" or flag "--sqlite-storage-path": is required but was not set`,
		},

		// --- minLength ---

		{
			name: "minLength: empty sqlite path",
			config: []byte(`storage:
  sqlite:
    path: ""`),
			validateErr: `config value ".storage.sqlite.path" or flag "--sqlite-storage-path": must not be empty`,
		},
		{
			name: "minLength: empty account id",
			config: []byte(`account:
  id: ""`),
			validateErr: `config value ".account.id" or flag "--account-id": must not be empty`,
		},
		{
			name: "minLength: empty account name",
			config: []byte(`account:
  name: ""`),
			validateErr: `config value ".account.name" or flag "--name": must not be empty`,
		},
		{
			name: "minLength: empty etcd private key source",
			config: []byte(`storage:
  default:
    etcd:
      privateKeySource: ""`),
			validateErr: `config value ".storage.default.etcd.privateKeySource" or flag "--private-key-source": must not be empty`,
		},

		// --- minimum ---

		{
			name: "minimum: sqlite cached pool size below 1",
			config: []byte(`storage:
  sqlite:
    cachedPoolSize: 0`),
			validateErr: `config value ".storage.sqlite.cachedPoolSize": must be at least 1`,
		},
		{
			name: "minimum: sqlite pool size below 1",
			config: []byte(`storage:
  sqlite:
    poolSize: 0`),
			validateErr: `config value ".storage.sqlite.poolSize": must be at least 1`,
		},

		// --- enum ---

		{
			name:        "enum: invalid join tokens mode",
			config:      configInvalidJoinTokenMode,
			validateErr: `config value ".services.siderolink.joinTokensMode" or flag "--join-tokens-mode": must be one of 'strict', 'legacyAllowed', 'legacy'`,
		},
		{
			name: "enum: invalid storage kind",
			config: []byte(`storage:
  default:
    kind: invalid`),
			validateErr: `config value ".storage.default.kind" or flag "--storage-kind": must be one of 'etcd', 'boltdb'`,
		},

		// --- const ---

		{
			name:   "const: webauthn enabled must be false",
			config: configFull,
			configModifyFunc: func(cfg *config.Params) {
				cfg.Auth.Webauthn.SetEnabled(true)
			},
			validateErr: `config value ".auth.webauthn.enabled" or flag "--auth-webauthn-enabled": must be false`,
		},
		{
			name:   "const: webauthn required must be false",
			config: configFull,
			configModifyFunc: func(cfg *config.Params) {
				cfg.Auth.Webauthn.SetRequired(true)
			},
			validateErr: `config value ".auth.webauthn.required" or flag "--auth-webauthn-required": must be false`,
		},

		// --- pattern ---

		{
			name:   "pattern: kubernetes proxy advertised URL must be https",
			config: configFull,
			configModifyFunc: func(cfg *config.Params) {
				cfg.Services.KubernetesProxy.SetAdvertisedURL("http://1.1.1.1:1111")
			},
			validateErr: `config value ".services.kubernetesProxy.advertisedURL" or flag "--advertised-kubernetes-proxy-url": must start with 'https://'`,
		},
		{
			name: "pattern: workload proxy subdomain invalid",
			config: []byte(`services:
  workloadProxy:
    subdomain: "INVALID!!"`),
			validateErr: `config value ".services.workloadProxy.subdomain" or flag "--workload-proxying-subdomain": must be a valid DNS subdomain (lowercase alphanumeric, dots, hyphens)`,
		},
		{
			name: "pattern: sqlite experimental base params starts with ?",
			config: []byte(`storage:
  sqlite:
    experimentalBaseParams: "?bad"`),
			validateErr: `config value ".storage.sqlite.experimentalBaseParams" or flag "--sqlite-storage-experimental-base-params": must not start with '?'`,
		},
		{
			name: "pattern: sqlite extra params starts with &",
			config: []byte(`storage:
  sqlite:
    extraParams: "&bad"`),
			validateErr: `config value ".storage.sqlite.extraParams" or flag "--sqlite-storage-extra-params": must not start with '&'`,
		},

		// --- if/then/else + not (kept as-is for now, TODO: move to Go validation) ---

		{
			name:        "if/then: conflicting auth providers",
			config:      conflictingAuth,
			validateErr: "'not' failed",
		},
		{
			name:        "if/then: conflicting backup storage",
			config:      backups,
			validateErr: "'not' failed",
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

			err = cfg.Validate(schema)

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
			Endpoint:      new("1.1.1.1:1111"),
			AdvertisedURL: new("https://2.2.2.2:2222"),
		}

		url := conf.URL()
		assert.Equal(t, "https://2.2.2.2:2222", url)
	})

	t.Run("secure https", func(t *testing.T) {
		t.Parallel()

		conf := &config.Service{
			Endpoint: new("1.1.1.1:1111"),
			CertFile: new("/path/to/cert"),
			KeyFile:  new("/path/to/key"),
		}

		url := conf.URL()
		assert.Equal(t, "https://1.1.1.1:1111", url)
	})

	t.Run("siderolink api - insecure as grpc", func(t *testing.T) {
		t.Parallel()

		conf := &config.MachineAPI{
			Endpoint: new("1.1.1.1:1111"),
		}

		url := conf.URL()
		assert.Equal(t, "grpc://1.1.1.1:1111", url, "siderolink api should use grpc schema when no tls")
	})

	t.Run("kubernetes proxy - insecure as https", func(t *testing.T) {
		t.Parallel()

		conf := &config.KubernetesProxyService{
			Endpoint: new("1.1.1.1:1111"),
		}

		url := conf.URL()

		assert.Equal(t, "https://1.1.1.1:1111", url, "kubernetes proxy should always use https schema")
	})
}
