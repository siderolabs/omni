// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package config contains the application config loading functions.
package config

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xyaml"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/compression"
	consts "github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config/validations"
	"github.com/siderolabs/omni/internal/pkg/jsonschema"
)

const (
	wireguardDefaultPort  = "50180"
	SQLiteStoragePathFlag = "sqlite-storage-path"
)

//go:embed schema.json
var schemaData string

// ParseSchema parses the embedded JSON schema for the Omni config.
func ParseSchema() (*jsonschema.Schema, error) {
	return jsonschema.Parse("omni", schemaData)
}

// ParseOption describes an additional optional arg to the parseConfig function.
type ParseOption func(*ParseOptions)

// ParseOptions describes additional options for parsing the Omni config.
type ParseOptions struct {
	ignoreUnknownFields bool
}

// WithIgnoreUnknownFields ignores the unknown fields present in the config file.
func WithIgnoreUnknownFields() ParseOption {
	return func(po *ParseOptions) {
		po.ignoreUnknownFields = true
	}
}

// FromBytes loads the config from bytes.
func FromBytes(data []byte) (*Params, error) {
	return parseConfig(bytes.NewBuffer(data))
}

// LoadFromFile loads the config from the file.
func LoadFromFile(path string, opts ...ParseOption) (*Params, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck

	return parseConfig(f, opts...)
}

// Default creates the new default configuration.
func Default() *Params {
	p := &Params{}

	p.Account.SetId("edd2822a-7834-4fe0-8172-cc5581f13a8d")
	p.Account.SetName("default")

	p.Services.Api.SetEndpoint(net.JoinHostPort("0.0.0.0", "8080"))
	p.Services.Api.SetAdvertisedURL("http://localhost:8080")

	p.Services.KubernetesProxy.SetEndpoint(net.JoinHostPort("0.0.0.0", "8095"))
	p.Services.KubernetesProxy.SetAdvertisedURL("https://localhost:8095")

	p.Services.Metrics.SetEndpoint(net.JoinHostPort("0.0.0.0", "2122"))

	p.Services.Siderolink.WireGuard.SetEndpoint(net.JoinHostPort("0.0.0.0", wireguardDefaultPort))
	p.Services.Siderolink.WireGuard.SetAdvertisedEndpoint(net.JoinHostPort(localIP, wireguardDefaultPort))
	p.Services.Siderolink.SetEventSinkPort(8090)
	p.Services.Siderolink.SetLogServerPort(8092)
	p.Services.Siderolink.SetJoinTokensMode(SiderolinkServiceJoinTokensModeLegacy)

	p.Services.MachineAPI.SetEndpoint(net.JoinHostPort(localIP, "8090"))

	p.Services.LoadBalancer.SetMinPort(10000)
	p.Services.LoadBalancer.SetMaxPort(35000)
	p.Services.LoadBalancer.SetDialTimeout(15 * time.Second)
	p.Services.LoadBalancer.SetKeepAlivePeriod(30 * time.Second)
	p.Services.LoadBalancer.SetTcpUserTimeout(30 * time.Second)
	p.Services.LoadBalancer.SetHealthCheckInterval(20 * time.Second)
	p.Services.LoadBalancer.SetHealthCheckTimeout(15 * time.Second)

	p.Services.LocalResourceService.SetEnabled(true)
	p.Services.LocalResourceService.SetPort(8081)

	p.Services.EmbeddedDiscoveryService.SetEnabled(true)
	p.Services.EmbeddedDiscoveryService.SetPort(8093)
	p.Services.EmbeddedDiscoveryService.SetSnapshotsEnabled(true)
	p.Services.EmbeddedDiscoveryService.SetSnapshotsPath("_out/secondary-storage/discovery-service-state.binpb") // todo: Keeping this enabled to get it migrated to SQLite.
	p.Services.EmbeddedDiscoveryService.SetSnapshotsInterval(10 * time.Minute)
	p.Services.EmbeddedDiscoveryService.SetLogLevel(zapcore.WarnLevel.String())
	p.Services.EmbeddedDiscoveryService.SetSqliteTimeout(30 * time.Second)

	p.Services.WorkloadProxy.SetSubdomain("proxy-us")
	p.Services.WorkloadProxy.SetEnabled(true)
	p.Services.WorkloadProxy.SetStopLBsAfter(5 * time.Minute)

	p.Auth.KeyPruner.SetInterval(10 * time.Minute)

	p.Auth.InitialServiceAccount.SetEnabled(false)
	p.Auth.InitialServiceAccount.SetRole(string(role.Admin))
	p.Auth.InitialServiceAccount.SetKeyPath("_out/initial-service-account-key")
	p.Auth.InitialServiceAccount.SetName("automation")
	p.Auth.InitialServiceAccount.SetLifetime(time.Hour)

	p.Registries.SetTalos(consts.TalosRegistry)
	p.Registries.SetKubernetes(consts.KubernetesRegistry)
	p.Registries.SetImageFactoryBaseURL(consts.ImageFactoryBaseURL)

	p.Logs.Audit.SetEnabled(true)
	p.Logs.Audit.SetPath("_out/audit")
	p.Logs.Audit.SetSqliteTimeout(30 * time.Second)

	p.Logs.ResourceLogger.SetLogLevel(zapcore.InfoLevel.String())
	p.Logs.ResourceLogger.Types = common.UserManagedResourceTypes

	p.Logs.Machine.SetBufferInitialCapacity(16384)
	p.Logs.Machine.SetBufferMaxCapacity(131072)
	p.Logs.Machine.SetBufferSafetyGap(256)

	p.Logs.Machine.Storage.SetEnabled(true)
	p.Logs.Machine.Storage.SetPath("_out/logs")
	p.Logs.Machine.Storage.SetFlushPeriod(10 * time.Minute)
	p.Logs.Machine.Storage.SetFlushJitter(0.1)
	p.Logs.Machine.Storage.SetNumCompressedChunks(5)
	p.Logs.Machine.Storage.SetSqliteTimeout(30 * time.Second)
	p.Logs.Machine.Storage.SetCleanupInterval(30 * time.Minute)
	p.Logs.Machine.Storage.SetCleanupOlderThan(24 * 30 * time.Hour)
	p.Logs.Machine.Storage.SetMaxLinesPerMachine(5000)
	p.Logs.Machine.Storage.SetCleanupProbability(0.01)

	p.Storage.Secondary.SetPath("_out/secondary-storage/bolt.db")

	p.Storage.Sqlite.SetExperimentalBaseParams("_txlock=immediate&_pragma=busy_timeout(50000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")

	p.Storage.Default.SetKind(StorageDefaultKindEtcd)
	p.Storage.Default.Boltdb.SetPath("_out/omni.db")
	p.Storage.Default.Etcd.Endpoints = []string{"http://localhost:2379"}
	p.Storage.Default.Etcd.SetDialKeepAliveTime(30 * time.Second)
	p.Storage.Default.Etcd.SetDialKeepAliveTimeout(5 * time.Second)
	p.Storage.Default.Etcd.SetCaFile("etcd/ca.crt")
	p.Storage.Default.Etcd.SetCertFile("etcd/client.crt")
	p.Storage.Default.Etcd.SetKeyFile("etcd/client.key")
	p.Storage.Default.Etcd.SetEmbedded(true)
	p.Storage.Default.Etcd.SetRunElections(false)
	p.Storage.Default.Etcd.SetEmbeddedDBPath("_out/etcd/")

	p.Features.SetEnableConfigDataCompression(true)
	p.Features.SetEnableClusterImport(true)

	p.EtcdBackup.SetTickInterval(time.Minute)
	p.EtcdBackup.SetMinInterval(time.Hour)
	p.EtcdBackup.SetMaxInterval(24 * time.Hour)
	p.EtcdBackup.SetJitter(10 * time.Minute)

	p.Debug.Server.SetEndpoint(":9988")

	return p
}

// Init the config using defaults, merge with overrides, populate fallbacks and validate.
func Init(logger *zap.Logger, schema *jsonschema.Schema, params ...*Params) (*Params, error) {
	config := Default()

	for _, override := range params {
		if err := merge.Merge(config, override); err != nil {
			return nil, err
		}
	}

	config.PopulateFallbacks()

	if err := config.Validate(schema); err != nil {
		return nil, err
	}

	enableCompression := config.Features.GetEnableConfigDataCompression()
	if err := compression.InitConfig(enableCompression); err != nil {
		return nil, err
	}

	logger.Info("initialized resource compression config", zap.Bool("enabled", enableCompression))

	return config, nil
}

func parseConfig(r io.Reader, opts ...ParseOption) (*Params, error) {
	var options ParseOptions

	for _, o := range opts {
		o(&options)
	}

	if options.ignoreUnknownFields {
		var config Params

		return &config, yaml.NewDecoder(r).Decode(&config)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var config Params

	if err := xyaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ValidateState validate Omni params against the current state of Omni instance.
// Add any hooks that would need to validate the config against the state here.
func (p *Params) ValidateState(ctx context.Context, st state.State) error {
	if p.Services.Siderolink.GetJoinTokensMode() == SiderolinkServiceJoinTokensModeStrict {
		if err := validations.EnsureAllMachinesSupportStrictTokens(ctx, st); err != nil {
			return err
		}
	}

	return nil
}

// Validate Omni params.
func (p *Params) Validate(schema *jsonschema.Schema) error {
	if schema == nil {
		return errors.New("schema is nil")
	}

	var sb strings.Builder

	encoder := yaml.NewEncoder(&sb)
	encoder.SetIndent(2)

	if err := encoder.Encode(p); err != nil {
		return fmt.Errorf("failed to encode config to YAML for validation: %w", err)
	}

	configYAML := sb.String()

	if err := schema.Validate(configYAML); err != nil {
		return fmt.Errorf("failed to validate config against JSON schema: %w", err)
	}

	return nil
}

var (
	localIP = getLocalIPOrEmpty()

	// Config holds the application config and provides the default values for it.
	Config = Default()
)

// GetImageFactoryPXEBaseURL reads image factory PXE address from the args.
func (p *Params) GetImageFactoryPXEBaseURL() (*url.URL, error) {
	pxeBaseURL := p.Registries.GetImageFactoryPXEBaseURL()
	if pxeBaseURL != "" {
		return url.Parse(pxeBaseURL)
	}

	factoryBaseURL := p.Registries.GetImageFactoryBaseURL()

	url, err := url.Parse(factoryBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL specified for the image factory: %w", err)
	}

	url.Host = fmt.Sprintf("pxe.%s", url.Host)

	return url, nil
}

// GetOIDCIssuerEndpoint returns the OIDC issuer endpoint.
func (p *Params) GetOIDCIssuerEndpoint() (string, error) {
	u, err := url.Parse(p.Services.Api.URL())
	if err != nil {
		return "", err
	}

	u.Path, err = url.JoinPath(u.Path, "/oidc")
	if err != nil {
		return "", err
	}

	return u.String(), nil
}

// PopulateFallbacks in the config file.
func (p *Params) PopulateFallbacks() {
	// copy the keys from the main API server if kubernetes proxy doesn't have certs defined explicitly.
	if !p.Services.KubernetesProxy.IsSecure() {
		p.Services.KubernetesProxy.SetCertFile(p.Services.Api.GetCertFile())
		p.Services.KubernetesProxy.SetKeyFile(p.Services.Api.GetKeyFile())
	}

	// copy the keys from the main API server if dev server proxy doesn't have certs defined explicitly.
	if !p.Services.DevServerProxy.IsSecure() {
		p.Services.DevServerProxy.SetCertFile(p.Services.Api.GetCertFile())
		p.Services.DevServerProxy.SetKeyFile(p.Services.Api.GetKeyFile())
	}

	if p.Auth.Auth0.InitialUsers != nil && p.Auth.InitialUsers == nil { //nolint:staticcheck
		p.Auth.InitialUsers = p.Auth.Auth0.InitialUsers //nolint:staticcheck
	}
}

func getLocalIPOrEmpty() string {
	ip, _ := getLocalIP() //nolint:errcheck

	return ip
}

// getLocalIP returns the non-loopback local IP of the host, preferring IPv4 over IPv6.
func getLocalIP() (string, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	var firstIPV6 string

	for _, address := range addresses {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}

			if firstIPV6 == "" {
				firstIPV6 = ipnet.IP.String()
			}
		}
	}

	if firstIPV6 != "" {
		return firstIPV6, nil
	}

	return "", errors.New("could not determine local IP address")
}

// EtcdBackupStorage defines etcd backup storage type.
type EtcdBackupStorage string

const (
	// EtcdBackupTypeNone is the no backup storage type.
	EtcdBackupTypeNone EtcdBackupStorage = "none"
	// EtcdBackupTypeS3 is the S3 backup storage type.
	EtcdBackupTypeS3 EtcdBackupStorage = "s3"
	// EtcdBackupTypeFS is the filesystem backup storage type.
	EtcdBackupTypeFS EtcdBackupStorage = "local"
)
