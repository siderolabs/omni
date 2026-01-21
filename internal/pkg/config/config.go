// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package config contains the application config loading functions.
package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/go-playground/validator/v10"
	"github.com/siderolabs/gen/xyaml"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/compression"
	consts "github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config/validations"
)

const (
	wireguardDefaultPort  = "50180"
	SQLiteStoragePathFlag = "sqlite-storage-path"
)

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
func FromBytes(data []byte, opts ...ParseOption) (*Params, error) {
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
	return &Params{
		Account: Account{
			ID:   "edd2822a-7834-4fe0-8172-cc5581f13a8d",
			Name: "default",
		},
		Services: Services{
			API: &Service{
				BindEndpoint:  net.JoinHostPort("0.0.0.0", "8080"),
				AdvertisedURL: "http://localhost:8080",
			},
			KubernetesProxy: &KubernetesProxyService{
				BindEndpoint:  net.JoinHostPort("0.0.0.0", "8095"),
				AdvertisedURL: "https://localhost:8095",
			},
			Metrics: &Service{
				BindEndpoint: net.JoinHostPort("0.0.0.0", "2122"),
			},
			Siderolink: &SiderolinkService{
				WireGuard: SiderolinkWireGuard{
					BindEndpoint:       net.JoinHostPort("0.0.0.0", wireguardDefaultPort),
					AdvertisedEndpoint: net.JoinHostPort(localIP, wireguardDefaultPort),
				},
				EventSinkPort:  8090,
				LogServerPort:  8092,
				JoinTokensMode: JoinTokensModeLegacyOnly,
			},
			MachineAPI: &MachineAPI{
				BindEndpoint: net.JoinHostPort(localIP, "8090"),
			},
			LoadBalancer: &LoadBalancerService{
				MinPort: 10000,
				MaxPort: 35000,

				DialTimeout:     15 * time.Second,
				KeepAlivePeriod: 30 * time.Second,
				TCPUserTimeout:  30 * time.Second,

				HealthCheckInterval: 20 * time.Second,
				HealthCheckTimeout:  15 * time.Second,
			},
			DevServerProxy: &DevServerProxyService{},
			LocalResourceService: &LocalResourceService{
				Enabled: true,
				Port:    8081,
			},
			EmbeddedDiscoveryService: &EmbeddedDiscoveryService{
				Enabled:           true,
				Port:              8093,
				SnapshotsEnabled:  true,
				SnapshotsPath:     "_out/secondary-storage/discovery-service-state.binpb", // todo: Keeping this enabled to get it migrated to SQLite.
				SnapshotsInterval: 10 * time.Minute,
				LogLevel:          zapcore.WarnLevel.String(),
				SQLiteTimeout:     30 * time.Second,
			},
			WorkloadProxy: &WorkloadProxy{
				Subdomain:    "proxy-us",
				Enabled:      true,
				StopLBsAfter: 5 * time.Minute,
			},
		},
		Auth: Auth{
			KeyPruner: KeyPrunerConfig{
				Interval: 10 * time.Minute,
			},
			InitialServiceAccount: InitialServiceAccount{
				Enabled:  false,
				Role:     string(role.Admin),
				KeyPath:  "_out/initial-service-account-key",
				Name:     "automation",
				Lifetime: time.Hour,
			},
		},
		Registries: Registries{
			Talos:               consts.TalosRegistry,
			Kubernetes:          consts.KubernetesRegistry,
			ImageFactoryBaseURL: consts.ImageFactoryBaseURL,
		},
		Logs: Logs{
			Audit: LogsAudit{
				Enabled:       pointer.To(true),
				Path:          "_out/audit",
				SQLiteTimeout: 30 * time.Second,
			},
			ResourceLogger: ResourceLoggerConfig{
				LogLevel: zapcore.InfoLevel.String(),
				Types:    common.UserManagedResourceTypes,
			},
			Machine: LogsMachine{
				BufferInitialCapacity: 16384,
				BufferMaxCapacity:     131072,
				BufferSafetyGap:       256,
				Storage: LogsMachineStorage{
					Enabled:             true,
					Path:                "_out/logs",
					FlushPeriod:         10 * time.Minute,
					FlushJitter:         0.1,
					NumCompressedChunks: 5,

					SQLiteTimeout:      30 * time.Second,
					CleanupInterval:    30 * time.Minute,
					CleanupOlderThan:   24 * 30 * time.Hour,
					MaxLinesPerMachine: 5000,
					CleanupProbability: 0.01,
				},
			},
		},
		Storage: Storage{
			Secondary: BoltDB{
				Path: "_out/secondary-storage/bolt.db",
			},
			SQLite: SQLite{
				ExperimentalBaseParams: "_txlock=immediate&_pragma=busy_timeout(50000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)",
			},
			Default: &StorageDefault{
				Kind: "etcd",
				Boltdb: BoltDB{
					Path: "_out/omni.db",
				},
				Etcd: EtcdParams{
					Endpoints:            []string{"http://localhost:2379"},
					DialKeepAliveTime:    30 * time.Second,
					DialKeepAliveTimeout: 5 * time.Second,
					CAFile:               "etcd/ca.crt",
					CertFile:             "etcd/client.crt",
					KeyFile:              "etcd/client.key",

					Embedded:       true,
					RunElections:   false,
					EmbeddedDBPath: "_out/etcd/",
				},
			},
		},
		Features: Features{
			EnableConfigDataCompression: true,
			EnableClusterImport:         true,
		},
		EtcdBackup: EtcdBackup{
			TickInterval: time.Minute,
			MinInterval:  time.Hour,
			MaxInterval:  24 * time.Hour,
			Jitter:       10 * time.Minute,
		},
		Debug: Debug{
			Server: DebugServer{
				Endpoint: ":9988",
			},
		},
	}
}

// Init the config using defaults, merge with overrides, populate fallbacks and validate.
func Init(logger *zap.Logger, params ...*Params) (*Params, error) {
	config := Default()

	for _, override := range params {
		if err := merge.Merge(config, override); err != nil {
			return nil, err
		}
	}

	config.PopulateFallbacks()

	if err := config.Validate(); err != nil {
		return nil, err
	}

	if err := compression.InitConfig(config.Features.EnableConfigDataCompression); err != nil {
		return nil, err
	}

	logger.Info("initialized resource compression config", zap.Bool("enabled", config.Features.EnableConfigDataCompression))

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

// Params defines application configs.
//
//nolint:govet
type Params struct {
	Account    Account    `yaml:"account" validate:"required"`
	Services   Services   `yaml:"services" validate:"required"`
	Auth       Auth       `yaml:"auth" validate:"required"`
	Logs       Logs       `yaml:"logs" validate:"required"`
	Storage    Storage    `yaml:"storage" validate:"required"`
	EtcdBackup EtcdBackup `yaml:"etcdBackup"`
	Registries Registries `yaml:"registries" validate:"required"`
	Debug      Debug      `yaml:"debug"`
	Features   Features   `yaml:"features"`
}

// ValidateState validate Omni params against the current state of Omni instance.
// Add any hooks that would need to validate the config against the state here.
func (p *Params) ValidateState(ctx context.Context, st state.State) error {
	if p.Services.Siderolink.JoinTokensMode == JoinTokensModeStrict {
		if err := validations.EnsureAllMachinesSupportStrictTokens(ctx, st); err != nil {
			return err
		}
	}

	return nil
}

// Validate Omni params.
func (p *Params) Validate() error {
	validate := validator.New(validator.WithRequiredStructEnabled())

	if err := validate.Struct(p); err != nil {
		return p.handleValidationErrors(err)
	}

	if p.Auth.Auth0.Enabled && p.Auth.SAML.Enabled {
		return fmt.Errorf("SAML and Auth0 are mutually exclusive")
	}

	return nil
}

// handleValidationErrors customizes validation error messages returned by the validator.
//
// It is mainly used to provide more user-friendly error messages for required fields, e.g., by
// suggesting the corresponding CLI flag to set the missing value.
//
// TODO: we should do this in a more generic, unified way in the future.
func (p *Params) handleValidationErrors(err error) error {
	var validationErrs validator.ValidationErrors

	if !errors.As(err, &validationErrs) {
		return err
	}

	for i, validationErr := range validationErrs {
		if validationErr.Tag() != "required" {
			continue
		}

		ns := validationErr.Namespace()
		switch ns { //nolint:gocritic
		case "Params.Storage.SQLite.Path":
			validationErrs[i] = &fieldError{
				FieldError: validationErr,
				customErr:  fmt.Sprintf("missing required config value: %v, can be specified using --%s flag", ns, SQLiteStoragePathFlag),
			}
		}
	}

	return validationErrs
}

type fieldError struct {
	validator.FieldError
	customErr string
}

func (f fieldError) Error() string {
	return f.customErr
}

// Account defines Omni account settings.
type Account struct {
	// ID is the stable identifier of the instance.
	//
	// Omni will use that to build paths to etcd storage, etc.
	ID string `yaml:"id" validate:"required"`
	// Name is the user-facing name of the instance.
	//
	// Omni will use to present some information to the user.
	// Name can be changed at any time.
	Name string `yaml:"name" validate:"required"`

	// UserPilot configuration.
	UserPilot UserPilot `yaml:"userPilot"`
}

// UserPilot describes user pilot credentials.
// If not set it is disabled.
type UserPilot struct {
	// AppToken is the token used to report metrics to the userpilot service.
	AppToken string `yaml:"appToken"`
}

// Registries configures docker registries to be used for the Talos and Kubernetes images.
// Also it has URLs for the image factory.
type Registries struct {
	Talos      string `yaml:"talos" validate:"required"`
	Kubernetes string `yaml:"kubernetes" validate:"required"`

	ImageFactoryBaseURL    string `yaml:"imageFactoryBaseURL" validate:"required"`
	ImageFactoryPXEBaseURL string `yaml:"imageFactoryPXEBaseURL"`

	// Mirrors enables registry mirrors for all Talos machines connected to Omni.
	Mirrors []string `yaml:"mirrors"`
}

var (
	localIP = getLocalIPOrEmpty()

	// Config holds the application config and provides the default values for it.
	Config = Default()
)

// GetImageFactoryPXEBaseURL reads image factory PXE address from the args.
func (p *Params) GetImageFactoryPXEBaseURL() (*url.URL, error) {
	if p.Registries.ImageFactoryPXEBaseURL != "" {
		return url.Parse(p.Registries.ImageFactoryPXEBaseURL)
	}

	url, err := url.Parse(p.Registries.ImageFactoryBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL specified for the image factory: %w", err)
	}

	url.Host = fmt.Sprintf("pxe.%s", url.Host)

	return url, nil
}

// GetOIDCIssuerEndpoint returns the OIDC issuer endpoint.
func (p *Params) GetOIDCIssuerEndpoint() (string, error) {
	u, err := url.Parse(p.Services.API.URL())
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
		p.Services.KubernetesProxy.CertFile = p.Services.API.CertFile
		p.Services.KubernetesProxy.KeyFile = p.Services.API.KeyFile
	}

	// copy the keys from the main API server if dev server proxy doesn't have certs defined explicitly.
	if p.Services.DevServerProxy != nil && !p.Services.DevServerProxy.IsSecure() {
		p.Services.DevServerProxy.CertFile = p.Services.API.CertFile
		p.Services.DevServerProxy.KeyFile = p.Services.API.KeyFile
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
