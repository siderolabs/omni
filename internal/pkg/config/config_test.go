// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config_test

import (
	"context"
	_ "embed"
	"fmt"
	"reflect"
	"strings"
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

// TestConfigStructNillability verifies that all leaf fields in the generated config structs
// are nillable (pointer, slice, or map), and all intermediate struct fields are non-pointer.
// This prevents regressions when extending the config schema.
//
// Leaf fields must be nillable so that we can distinguish between "unset" and "zero value".
// This is critical for reliably merging multiple config layers (schema defaults, config files,
// CLI flags): without nillability, a zero-valued field (e.g., false for bool) in a higher
// layer would silently overwrite an explicitly set value in a lower layer, which would be wrong.
//
// Intermediate struct fields, on the other hand, do not need to be nillable — they are purely
// structural containers, and keeping them non-pointer allows safe navigation to leaf fields
// without nil checks at every level.
func TestConfigStructNillability(t *testing.T) {
	var violations []string

	checkStructNillability(reflect.TypeFor[config.Params](), "Params", &violations)

	if len(violations) > 0 {
		t.Errorf("config struct nillability violations:\n%s", strings.Join(violations, "\n"))
	}
}

func checkStructNillability(typ reflect.Type, path string, violations *[]string) {
	for field := range typ.Fields() {
		if !field.IsExported() {
			continue
		}

		fieldPath := path + "." + field.Name
		fieldType := field.Type

		underlying := fieldType
		if fieldType.Kind() == reflect.Ptr {
			underlying = fieldType.Elem()
		}

		if isConfigStruct(underlying) {
			if fieldType.Kind() == reflect.Ptr {
				*violations = append(*violations, fmt.Sprintf(
					"%s: intermediate struct field must not be a pointer — mark it as required in its parent in schema.json to make it non-nillable",
					fieldPath,
				))
			}

			checkStructNillability(underlying, fieldPath, violations)
		} else if !isNillableKind(fieldType.Kind()) {
			*violations = append(*violations, fmt.Sprintf(
				"%s: leaf field must be nillable — add goJSONSchema.pointer=true in schema.json",
				fieldPath,
			))
		}
	}
}

// isConfigStruct returns true if the type is a struct with exported fields,
// i.e., an intermediate config object that contains other fields.
func isConfigStruct(typ reflect.Type) bool {
	if typ.Kind() != reflect.Struct {
		return false
	}

	for field := range typ.Fields() {
		if field.IsExported() {
			return true
		}
	}

	return false
}

func isNillableKind(kind reflect.Kind) bool {
	switch kind { //nolint:exhaustive
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		return true
	default:
		return false
	}
}

// TestSchemaDefaults verifies that defaults from schema.json are correctly
// read into the Params struct by LoadDefault.
func TestSchemaDefaults(t *testing.T) {
	p, err := config.LoadDefault()
	require.NoError(t, err)

	// account
	assert.Equal(t, "edd2822a-7834-4fe0-8172-cc5581f13a8d", p.Account.GetId())
	assert.Equal(t, "default", p.Account.GetName())

	// registries
	assert.Equal(t, "ghcr.io/siderolabs/installer", p.Registries.GetTalos())
	assert.Equal(t, "ghcr.io/siderolabs/kubelet", p.Registries.GetKubernetes())
	assert.Equal(t, "https://factory.talos.dev", p.Registries.GetImageFactoryBaseURL())

	// services.api (inline overrides on $ref)
	assert.Equal(t, "0.0.0.0:8080", p.Services.Api.GetEndpoint())
	assert.Equal(t, "http://localhost:8080", p.Services.Api.GetAdvertisedURL())

	// services.metrics
	assert.Equal(t, "0.0.0.0:2122", p.Services.Metrics.GetEndpoint())

	// services.kubernetesProxy
	assert.Equal(t, "0.0.0.0:8095", p.Services.KubernetesProxy.GetEndpoint())
	assert.Equal(t, "https://localhost:8095", p.Services.KubernetesProxy.GetAdvertisedURL())

	// services.siderolink
	assert.Equal(t, config.SiderolinkServiceJoinTokensModeLegacyAllowed, p.Services.Siderolink.GetJoinTokensMode())
	assert.Equal(t, 8090, p.Services.Siderolink.GetEventSinkPort())
	assert.Equal(t, 8092, p.Services.Siderolink.GetLogServerPort())

	// services.siderolink.wireGuard
	assert.Equal(t, "0.0.0.0:50180", p.Services.Siderolink.WireGuard.GetEndpoint())

	// services.localResourceService
	assert.True(t, p.Services.LocalResourceService.GetEnabled())
	assert.Equal(t, 8081, p.Services.LocalResourceService.GetPort())

	// services.embeddedDiscoveryService
	assert.True(t, p.Services.EmbeddedDiscoveryService.GetEnabled())
	assert.Equal(t, 8093, p.Services.EmbeddedDiscoveryService.GetPort())
	assert.True(t, p.Services.EmbeddedDiscoveryService.GetSnapshotsEnabled())
	assert.Equal(t, 10*time.Minute, p.Services.EmbeddedDiscoveryService.GetSnapshotsInterval())
	assert.Equal(t, "warn", p.Services.EmbeddedDiscoveryService.GetLogLevel())
	assert.Equal(t, 30*time.Second, p.Services.EmbeddedDiscoveryService.GetSqliteTimeout())

	// services.loadBalancer
	assert.Equal(t, 10000, p.Services.LoadBalancer.GetMinPort())
	assert.Equal(t, 35000, p.Services.LoadBalancer.GetMaxPort())
	assert.Equal(t, 15*time.Second, p.Services.LoadBalancer.GetDialTimeout())
	assert.Equal(t, 30*time.Second, p.Services.LoadBalancer.GetKeepAlivePeriod())
	assert.Equal(t, 30*time.Second, p.Services.LoadBalancer.GetTcpUserTimeout())
	assert.Equal(t, 20*time.Second, p.Services.LoadBalancer.GetHealthCheckInterval())
	assert.Equal(t, 15*time.Second, p.Services.LoadBalancer.GetHealthCheckTimeout())

	// services.workloadProxy
	assert.True(t, p.Services.WorkloadProxy.GetEnabled())
	assert.Equal(t, "proxy-us", p.Services.WorkloadProxy.GetSubdomain())
	assert.Equal(t, 5*time.Minute, p.Services.WorkloadProxy.GetStopLBsAfter())

	// auth.keyPruner
	assert.Equal(t, 10*time.Minute, p.Auth.KeyPruner.GetInterval())

	// auth.initialServiceAccount
	assert.False(t, p.Auth.InitialServiceAccount.GetEnabled())
	assert.Equal(t, "Admin", p.Auth.InitialServiceAccount.GetRole())
	assert.Equal(t, "_out/initial-service-account-key", p.Auth.InitialServiceAccount.GetKeyPath())
	assert.Equal(t, "automation", p.Auth.InitialServiceAccount.GetName())
	assert.Equal(t, time.Hour, p.Auth.InitialServiceAccount.GetLifetime())

	// logs.machine.storage
	assert.Equal(t, 30*time.Second, p.Logs.Machine.Storage.GetSqliteTimeout())
	assert.Equal(t, 30*time.Minute, p.Logs.Machine.Storage.GetCleanupInterval())
	assert.Equal(t, 720*time.Hour, p.Logs.Machine.Storage.GetCleanupOlderThan())
	assert.Equal(t, 5000, p.Logs.Machine.Storage.GetMaxLinesPerMachine())
	assert.Equal(t, uint64(0), p.Logs.Machine.Storage.GetMaxSize())
	assert.InDelta(t, 0.01, p.Logs.Machine.Storage.GetCleanupProbability(), 0.001)

	// logs.audit
	assert.True(t, p.Logs.Audit.GetEnabled())
	assert.Equal(t, 30*time.Second, p.Logs.Audit.GetSqliteTimeout())
	assert.Equal(t, 720*time.Hour, p.Logs.Audit.GetRetentionPeriod())
	assert.Equal(t, uint64(0), p.Logs.Audit.GetMaxSize())
	assert.InDelta(t, 0.01, p.Logs.Audit.GetCleanupProbability(), 0.001)

	// logs.resourceLogger
	assert.Equal(t, "info", p.Logs.ResourceLogger.GetLogLevel())

	// storage.default
	assert.Equal(t, config.StorageDefaultKindEtcd, p.Storage.Default.GetKind())

	// storage.default.etcd
	assert.True(t, p.Storage.Default.Etcd.GetEmbedded())
	assert.Equal(t, "_out/etcd/", p.Storage.Default.Etcd.GetEmbeddedDBPath())
	assert.False(t, p.Storage.Default.Etcd.GetRunElections())
	assert.Equal(t, "etcd/ca.crt", p.Storage.Default.Etcd.GetCaFile())
	assert.Equal(t, "etcd/client.crt", p.Storage.Default.Etcd.GetCertFile())
	assert.Equal(t, "etcd/client.key", p.Storage.Default.Etcd.GetKeyFile())
	assert.Equal(t, 30*time.Second, p.Storage.Default.Etcd.GetDialKeepAliveTime())
	assert.Equal(t, 5*time.Second, p.Storage.Default.Etcd.GetDialKeepAliveTimeout())
	assert.Equal(t, []string{"http://localhost:2379"}, p.Storage.Default.Etcd.Endpoints)

	// storage.default.boltdb
	assert.Equal(t, "_out/omni.db", p.Storage.Default.Boltdb.GetPath())

	// storage.sqlite
	assert.Equal(t, "_txlock=immediate&_pragma=busy_timeout(50000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", p.Storage.Sqlite.GetExperimentalBaseParams())
	assert.Equal(t, 4, p.Storage.Sqlite.GetCachedPoolSize())
	assert.Equal(t, 64, p.Storage.Sqlite.GetPoolSize())
	assert.Equal(t, 2*time.Minute, p.Storage.Sqlite.Metrics.GetRefreshInterval())
	assert.Equal(t, time.Minute, p.Storage.Sqlite.Metrics.GetRefreshTimeout())

	// etcdBackup
	assert.Equal(t, time.Minute, p.EtcdBackup.GetTickInterval())
	assert.Equal(t, time.Hour, p.EtcdBackup.GetMinInterval())
	assert.Equal(t, 24*time.Hour, p.EtcdBackup.GetMaxInterval())
	assert.Equal(t, 10*time.Minute, p.EtcdBackup.GetJitter())

	// debug.server
	assert.Equal(t, ":9988", p.Debug.Server.GetEndpoint())

	// features
	assert.True(t, p.Features.GetEnableConfigDataCompression())
	assert.True(t, p.Features.GetEnableClusterImport())

	// notifications.nonImageFactoryDeprecation
	assert.False(t, p.Notifications.NonImageFactoryDeprecation.GetEnabled())
	assert.Equal(t, "Non-ImageFactory Machines Detected", p.Notifications.NonImageFactoryDeprecation.GetTitle())
	assert.Equal(t, "%d machine(s) were provisioned without ImageFactory. Support for these machines will end after a future release. Please re-provision them using ImageFactory.",
		p.Notifications.NonImageFactoryDeprecation.GetBody())
}
