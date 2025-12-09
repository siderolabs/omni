// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"fmt"
	"time"
)

// HTTPService defines the interface for HTTP like services.
type HTTPService interface {
	URL() string
	GetCertFile() string
	GetKeyFile() string
	GetBindEndpoint() string
	IsSecure() bool
}

// Services configs.
//
//nolint:govet
type Services struct {
	// API is the Omni gRPC API service, gateway and the frontend.
	API *Service `yaml:"api"`
	// DevServerProxy is used in Omni development and allows proxying through Omni to the node JS dev server.
	DevServerProxy *DevServerProxyService `yaml:"devServerProxy"`
	// Metrics exposes prometheus metrics.
	Metrics *Service `yaml:"metrics"`
	// KubernetesProxy proxies the Kubernetes API to the clusters managed by Omni.
	KubernetesProxy *KubernetesProxyService `yaml:"kubernetesProxy"`
	// Siderolink manages WireGuard connections to the Talos machines connected to Omni.
	Siderolink *SiderolinkService `yaml:"siderolink"`
	// MachineAPI is the public API of Omni that helps to establish WireGuard connections.
	MachineAPI *MachineAPI `yaml:"machineAPI"`
	// LocalResourceService runs COSI API service that gives readonly access to all resources.
	LocalResourceService *LocalResourceService `yaml:"localResourceService"`
	// EmbeddedDiscoveryService runs https://discovery.talos.dev/ inside Omni.
	EmbeddedDiscoveryService *EmbeddedDiscoveryService `yaml:"embeddedDiscoveryService"`
	// LoadBalancer configures Omni Kubernetes loadbalancer runner.
	LoadBalancer *LoadBalancerService `yaml:"loadBalancer"`
	// WorkloadProxy runs the workload proxy service in Omni.
	WorkloadProxy *WorkloadProxy `yaml:"workloadProxy"`
}

// Service is the base service config.
type Service struct {
	BindEndpoint string `yaml:"endpoint"`
	// AdvertisedURL should be used when Omni runs behind an ingress.
	// This value is used in the machine join config, kernel params and schematics generation.
	AdvertisedURL string `yaml:"advertisedURL"`
	// CertFile is the TLS cert.
	CertFile string `yaml:"certFile"`
	// KeyFile is the TLS key.
	KeyFile string `yaml:"keyFile"`
}

// GetBindEndpoint implements HTTPService.
func (s *Service) GetBindEndpoint() string {
	return s.BindEndpoint
}

// GetCertFile implements HTTPService.
func (s *Service) GetCertFile() string {
	return s.CertFile
}

// GetKeyFile implements HTTPService.
func (s *Service) GetKeyFile() string {
	return s.KeyFile
}

// IsSecure returns true if both cert file and key file are present.
func (s *Service) IsSecure() bool {
	return s.CertFile != "" && s.KeyFile != ""
}

// URL gets the URL from the endpoint.
func (s *Service) URL() string {
	if s.AdvertisedURL != "" {
		return s.AdvertisedURL
	}

	schema := "http"
	if s.IsSecure() {
		schema = "https"
	}

	return fmt.Sprintf("%s://%s", schema, s.BindEndpoint)
}

// DevServerProxyService is used in Omni development and allows proxying through Omni to the node JS dev server.
type DevServerProxyService struct {
	Service `yaml:",inline"`

	ProxyTo string `yaml:"proxyTo"`
}

// KubernetesProxyService is the base service config.
type KubernetesProxyService struct {
	BindEndpoint string `yaml:"endpoint"`
	// AdvertisedURL should be used when Omni runs behind an ingress.
	// This value is used in the machine join config, kernel params and schematics generation.
	AdvertisedURL string `yaml:"advertisedURL"`
	// CertFile is the TLS cert.
	CertFile string `yaml:"certFile"`
	// KeyFile is the TLS key.
	KeyFile string `yaml:"keyFile"`
}

// GetBindEndpoint implements HTTPService.
func (ks *KubernetesProxyService) GetBindEndpoint() string {
	return ks.BindEndpoint
}

// GetCertFile implements HTTPKubernetesProxyService.
func (ks *KubernetesProxyService) GetCertFile() string {
	return ks.CertFile
}

// GetKeyFile implements HTTPKubernetesProxyService.
func (ks *KubernetesProxyService) GetKeyFile() string {
	return ks.KeyFile
}

// IsSecure returns true if both cert file and key file are present.
func (ks *KubernetesProxyService) IsSecure() bool {
	return ks.CertFile != "" && ks.KeyFile != ""
}

// URL returns kubernetes services URL.
// It is always HTTPS.
func (ks *KubernetesProxyService) URL() string {
	if ks.AdvertisedURL != "" {
		return ks.AdvertisedURL
	}

	return fmt.Sprintf("https://%s", ks.BindEndpoint)
}

// SiderolinkService manages WireGuard connections to the Talos machines connected to Omni.
type SiderolinkService struct {
	WireGuard SiderolinkWireGuard `yaml:"wireGuard"`
	// JoinTokensMode controls Talos machine join tokens operation mode.
	// - strict - only for Talos >= 1.6.x
	// - legacyAllowed - relies on the legacy join tokens mode for Talos < 1.6.x (less secure, use only if Talos upgrade is not an option)
	// - legacy - does not use node unique join tokens mode
	JoinTokensMode string `yaml:"joinTokensMode"  validate:"oneof=strict legacyAllowed legacy"`
	// DisableLastEndpoint disables populating last known peer endpoint for the WireGuard peers.
	// Using last known peer endpoints helps Omni quicker re-establish WireGuard connection to the nodes
	// after it is restarted.
	// Enable this flag if Omni runs behind the ingress and doesn't see the real node IPs.
	DisableLastEndpoint bool `yaml:"disableLastEndpoint"`
	// UsegRPCTunnel forces using WireGuard over gRPC for all machines on the account.
	UseGRPCTunnel bool `yaml:"useGRPCTunnel"`
	// EventSinkPort is the port where Talos nodes send Talos events.
	// This port is only open on the WireGuard tunnel Omni endpoint.
	EventSinkPort int `yaml:"eventSinkPort"`
	// LogServerPort is the port where Talos nodes send console logs.
	// This port is only open on the WireGuard tunnel Omni endpoint.
	LogServerPort int `yaml:"logServerPort"`
}

// SiderolinkWireGuard defines siderolink wireguard endpoint config.
type SiderolinkWireGuard struct {
	BindEndpoint       string `yaml:"endpoint"`
	AdvertisedEndpoint string `yaml:"advertisedEndpoint"`
}

// MachineAPI is the public API of Omni that helps to establish WireGuard connections.
// This API used to exchange WireGuard keys, assign IP addresses.
// If gRPC tunnel mode is used, WireGuard traffic goes over this endpoint too.
type MachineAPI Service

// URL composes URL for Talos to connect.
func (m MachineAPI) URL() string {
	if m.AdvertisedURL != "" {
		return m.AdvertisedURL
	}

	schema := "grpc"
	if m.CertFile != "" && m.KeyFile != "" {
		schema = "https"
	}

	return fmt.Sprintf("%s://%s", schema, m.BindEndpoint)
}

// LocalResourceService runs COSI API service that gives readonly access to all resources.
type LocalResourceService struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// EmbeddedDiscoveryService runs https://discovery.talos.dev/ inside Omni.
// Discovery service is only available inside the WireGuard tunnel
//
//nolint:govet
type EmbeddedDiscoveryService struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`

	// SnapshotsEnabled turns on the discovery service persistence.
	//
	// Deprecated: use SQLiteSnapshotsEnabled instead.
	SnapshotsEnabled bool `yaml:"snapshotsEnabled"`

	SQLiteSnapshotsEnabled bool `yaml:"sqliteSnapshotsEnabled"`
	// SnapshotsPath is the path on disk where to store the discovery service state.
	//
	// Deprecated: use SQLiteSnapshotsEnabled instead.
	SnapshotsPath     string        `yaml:"snapshotsPath"`
	SnapshotsInterval time.Duration `yaml:"snapshotsInterval"`
	LogLevel          string        `yaml:"logLevel"`
}

// LoadBalancerService configures Omni Kubernetes loadbalancer.
type LoadBalancerService struct {
	// MinPort is the minimum port number used for load balancer endpoints.
	MinPort int `yaml:"minPort"`
	// MaxPort is the maximum port number used for load balancer endpoints.
	MaxPort int `yaml:"maxPort"`

	DialTimeout     time.Duration `yaml:"dialTimeout"`
	KeepAlivePeriod time.Duration `yaml:"keepAlivePeriod"`
	TCPUserTimeout  time.Duration `yaml:"tcpUserTimeout"`

	HealthCheckInterval time.Duration `yaml:"healthCheckInterval"`
	HealthCheckTimeout  time.Duration `yaml:"healthCheckTimeout"`
}

// WorkloadProxy configures workload proxy.
type WorkloadProxy struct {
	Subdomain    string        `yaml:"subdomain"`
	Enabled      bool          `yaml:"enabled"`
	StopLBsAfter time.Duration `yaml:"stopLBsAfter"`
}

const (
	// JoinTokensModeLegacyOnly disables node unique token flow, uses only join token when letting the machine into the system.
	JoinTokensModeLegacyOnly = "legacy"
	// JoinTokensModeLegacyAllowed allows joining Talos nodes which do not support node unique token flow
	// uses unique token flow only for the machines which support it.
	JoinTokensModeLegacyAllowed = "legacyAllowed"
	// JoinTokensModeStrict rejects the machines that do not support node unique tokens flow.
	JoinTokensModeStrict = "strict"
)
