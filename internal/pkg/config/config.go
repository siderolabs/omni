// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package config contains the application config loading functions.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"go.uber.org/zap/zapcore"

	consts "github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
)

const (
	wireguardDefaultPort = "50180"
)

// Params defines application configs.
//
//nolint:govet
type Params struct {
	// AccountID is the stable identifier of the instance.
	//
	// Omni will use that to build paths to etcd storage, etc.
	AccountID string `yaml:"account_id"`
	// Name is the user-facing name of the instance.
	//
	// Omni will use to present some information to the user.
	// Name can be changed at any time.
	Name string `yaml:"name"`

	APIURL                string `yaml:"apiURL"`
	MachineAPIBindAddress string `yaml:"apiBindAddress"`
	MachineAPICertFile    string `yaml:"apiCertFile"`
	MachineAPIKeyFile     string `yaml:"apiKeyFile"`

	KubernetesProxyURL                   string `yaml:"kubernetesProxyURL"`
	SiderolinkEnabled                    bool   `yaml:"siderolinkEnabled"`
	SiderolinkWireguardBindAddress       string `yaml:"siderolinkWireguardBindAddress"`
	SiderolinkWireguardAdvertisedAddress string `yaml:"siderolinkWireguardAdvertisedAddress"`
	SiderolinkDisableLastEndpoint        bool   `yaml:"siderolinkDisableLastEndpoint"`

	EventSinkPort    int                `yaml:"eventSinkPort"`
	SideroLinkAPIURL string             `yaml:"siderolinkAPIURL"`
	LoadBalancer     LoadBalancerParams `yaml:"loadbalancer"`
	LogServerPort    int                `yaml:"logServerPort"`

	LogStorage LogStorageParams `yaml:"logStorage"`

	Auth AuthParams `yaml:"auth"`

	InitialUsers []string `yaml:"initialUsers"`

	TalosRegistry string `yaml:"talosRegistry"`

	KubernetesRegistry string `yaml:"kubernetesRegistry"`

	ImageFactoryBaseURL    string `yaml:"imageFactoryAddress"`
	ImageFactoryPXEBaseURL string `yaml:"imageFactoryProxyAddress"`

	Storage StorageParams `yaml:"storage"`

	SecondaryStorage BoltDBParams `yaml:"secondaryStorage"`

	DefaultConfigGenOptions []generate.Option `yaml:"-" json:"-"`

	KeyPruner KeyPrunerParams `yaml:"keyPruner"`

	EnableTalosPreReleaseVersions bool `yaml:"enableTalosPreReleaseVersions"`

	WorkloadProxying WorkloadProxyingParams `yaml:"workloadProxying"`

	LocalResourceServerPort int `yaml:"localResourceServerPort"`

	EtcdBackup EtcdBackupParams `yaml:"etcdBackup"`

	DisableControllerRuntimeCache bool `yaml:"disableControllerRuntimeCache"`

	LogResourceUpdatesTypes    []string
	LogResourceUpdatesLogLevel string
}

// EtcdBackupParams defines etcd backup configs.
type EtcdBackupParams struct {
	LocalPath    string        `yaml:"localPath"`
	S3Enabled    bool          `yaml:"s3Enabled"`
	TickInterval time.Duration `yaml:"tickInterval"`
	MinInterval  time.Duration `yaml:"minInterval"`
	MaxInterval  time.Duration `yaml:"maxInterval"`
}

// GetStorageType returns the storage type.
func (ebp EtcdBackupParams) GetStorageType() (EtcdBackupStorage, error) {
	if ebp.LocalPath != "" && ebp.S3Enabled {
		return "", errors.New("both localPath and s3 are set")
	}

	switch {
	case ebp.LocalPath == "" && !ebp.S3Enabled:
		return EtcdBackupTypeS3, nil
	case ebp.LocalPath != "":
		return EtcdBackupTypeFS, nil
	case ebp.S3Enabled:
		return EtcdBackupTypeS3, nil
	default:
		return "", errors.New("unknown backup storage type")
	}
}

// WorkloadProxyingParams defines workload proxying configs.
type WorkloadProxyingParams struct {
	Enabled bool `yaml:"enabled"`
}

// LoadBalancerParams defines load balancer configs.
type LoadBalancerParams struct {
	MinPort int `yaml:"minPort"`
	MaxPort int `yaml:"maxPort"`

	DialTimeout     time.Duration `yaml:"dialTimeout"`
	KeepAlivePeriod time.Duration `yaml:"keepAlivePeriod"`
	TCPUserTimeout  time.Duration `yaml:"tcpUserTimeout"`

	HealthCheckInterval time.Duration `yaml:"healthCheckInterval"`
	HealthCheckTimeout  time.Duration `yaml:"healthCheckTimeout"`
}

// StorageParams defines storage configs.
type StorageParams struct {
	// Kind can be either 'boltdb' or 'etcd'.
	Kind   string       `yaml:"kind"`
	Boltdb BoltDBParams `yaml:"boltdb"`
	Etcd   EtcdParams   `yaml:"etcd"`
}

// BoltDBParams defines boltdb storage configs.
type BoltDBParams struct {
	Path string `yaml:"path"`
}

// EtcdParams defines etcd storage configs.
type EtcdParams struct { ///nolint:govet
	// External etcd: list of endpoints, as host:port pairs.
	Endpoints []string `yaml:"endpoints"`
	CAPath    string   `yaml:"caPath"`
	CertPath  string   `yaml:"certPath"`
	KeyPath   string   `yaml:"keyPath"`

	// Use embedded etcd server (no clustering).
	Embedded            bool   `yaml:"embedded"`
	EmbeddedDBPath      string `yaml:"embeddedDBPath"`
	EmbeddedUnsafeFsync bool   `yaml:"embeddedUnsafeFsync"`

	PrivateKeySource string   `yaml:"privateKeySource"`
	PublicKeyFiles   []string `yaml:"publicKeysFiles"`
}

// KeyPrunerParams defines key pruner configs.
type KeyPrunerParams struct {
	Interval time.Duration `yaml:"interval"`
}

// LogStorageParams defines log storage configuration.
type LogStorageParams struct {
	Path        string        `yaml:"directory"`
	FlushPeriod time.Duration `yaml:"flushPeriod"`
	FlushJitter time.Duration `yaml:"flushJitter"`
	Enabled     bool          `yaml:"enabled"`
	Compress    bool          `yaml:"compress"`
}

var (
	localIP = getLocalIPOrEmpty()

	// Config holds the application config and provides the default values for it.
	Config = &Params{
		AccountID:                            "edd2822a-7834-4fe0-8172-cc5581f13a8d",
		Name:                                 "default",
		APIURL:                               fmt.Sprintf("http://%s", net.JoinHostPort("localhost", "8080")),
		KubernetesProxyURL:                   fmt.Sprintf("https://%s", net.JoinHostPort("localhost", "8095")),
		SiderolinkEnabled:                    true,
		SiderolinkWireguardBindAddress:       net.JoinHostPort("0.0.0.0", wireguardDefaultPort),
		SiderolinkWireguardAdvertisedAddress: net.JoinHostPort(localIP, wireguardDefaultPort),
		MachineAPIBindAddress:                net.JoinHostPort(localIP, "8090"),
		EventSinkPort:                        8090,
		SideroLinkAPIURL:                     fmt.Sprintf("grpc://%s", net.JoinHostPort(localIP, "8090")),
		LoadBalancer: LoadBalancerParams{
			MinPort: 10000,
			MaxPort: 35000,

			DialTimeout:     15 * time.Second,
			KeepAlivePeriod: 30 * time.Second,
			TCPUserTimeout:  30 * time.Second,

			HealthCheckInterval: 20 * time.Second,
			HealthCheckTimeout:  15 * time.Second,
		},
		KeyPruner: KeyPrunerParams{
			Interval: 10 * time.Minute,
		},
		LogServerPort: 8092,
		LogStorage: LogStorageParams{
			Enabled:     true,
			Path:        "_out/logs",
			FlushPeriod: 10 * time.Minute,
			FlushJitter: 2 * time.Minute,
			Compress:    true,
		},
		TalosRegistry:       consts.TalosRegistry,
		KubernetesRegistry:  consts.KubernetesRegistry,
		ImageFactoryBaseURL: consts.ImageFactoryBaseURL,
		Storage: StorageParams{
			Kind: "etcd",
			Boltdb: BoltDBParams{
				Path: "_out/omni.db",
			},
			Etcd: EtcdParams{
				Endpoints: []string{"http://localhost:2379"},
				CAPath:    "etcd/ca.crt",
				CertPath:  "etcd/client.crt",
				KeyPath:   "etcd/client.key",

				Embedded:       true,
				EmbeddedDBPath: "_out/etcd/",
			},
		},

		SecondaryStorage: BoltDBParams{
			Path: "_out/secondary-storage/bolt.db",
		},

		WorkloadProxying: WorkloadProxyingParams{
			Enabled: true,
		},

		LocalResourceServerPort: 8081,

		EtcdBackup: EtcdBackupParams{
			TickInterval: time.Minute,
			MinInterval:  time.Hour,
			MaxInterval:  24 * time.Hour,
		},

		LogResourceUpdatesLogLevel: zapcore.InfoLevel.String(),
		LogResourceUpdatesTypes:    common.UserManagedResourceTypes,
	}
)

// GetImageFactoryPXEBaseURL reads image factory PXE address from the args.
func (p *Params) GetImageFactoryPXEBaseURL() (*url.URL, error) {
	if p.ImageFactoryPXEBaseURL != "" {
		return url.Parse(p.ImageFactoryPXEBaseURL)
	}

	url, err := url.Parse(p.ImageFactoryBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL specified for the image factory: %w", err)
	}

	url.Host = fmt.Sprintf("pxe.%s", url.Host)

	return url, nil
}

// GetAdvertisedAPIHost returns the advertised host (IP or domain) of the API without the port.
func (p *Params) GetAdvertisedAPIHost() (string, error) {
	apiURL, err := url.Parse(p.SideroLinkAPIURL)
	if err != nil {
		return "", err
	}

	apiHost, _, err := net.SplitHostPort(apiURL.Host)
	if err != nil {
		apiHost = apiURL.Host
	}

	return apiHost, nil
}

// GetOIDCIssuerEndpoint returns the OIDC issuer endpoint.
func (p *Params) GetOIDCIssuerEndpoint() (string, error) {
	u, err := url.Parse(p.APIURL)
	if err != nil {
		return "", err
	}

	u.Path, err = url.JoinPath(u.Path, "/oidc")
	if err != nil {
		return "", err
	}

	return u.String(), nil
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
