// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"fmt"
)

// HTTPService defines the interface for HTTP like services.
type HTTPService interface {
	URL() string
	GetCertFile() string
	GetKeyFile() string
	GetBindEndpoint() string
	IsSecure() bool
}

// Service is the base service config.
type service struct {
	bindEndpoint  *string
	advertisedURL *string
	certFile      *string
	keyFile       *string
}

func wrapService(bindEndpoint, advertisedURL, certFile, keyFile *string) service {
	return service{
		bindEndpoint:  bindEndpoint,
		advertisedURL: advertisedURL,
		certFile:      certFile,
		keyFile:       keyFile,
	}
}

// getBindEndpoint implements HTTPService.
func (s service) getBindEndpoint() string {
	if s.bindEndpoint == nil {
		return ""
	}

	return *s.bindEndpoint
}

// isSecure returns true if both cert file and key file are present.
func (s service) isSecure() bool {
	return s.certFile != nil && *s.certFile != "" && s.keyFile != nil && *s.keyFile != ""
}

func (s service) url(schemeOverrides map[string]string) string {
	var advertisedURL string
	if s.advertisedURL != nil {
		advertisedURL = *s.advertisedURL
	}

	if advertisedURL != "" {
		return advertisedURL
	}

	schema := "http"
	if s.isSecure() {
		schema = "https"
	}

	if override, ok := schemeOverrides[schema]; ok {
		schema = override
	}

	bindEndpoint := ""
	if s.bindEndpoint != nil {
		bindEndpoint = *s.bindEndpoint
	}

	return fmt.Sprintf("%s://%s", schema, bindEndpoint)
}

// GetBindEndpoint implements HTTPService.
func (s *Service) GetBindEndpoint() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).getBindEndpoint()
}

// IsSecure returns true if both cert file and key file are present.
func (s *Service) IsSecure() bool {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).isSecure()
}

// URL gets the URL from the endpoint.
func (s *Service) URL() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).url(nil)
}

// GetBindEndpoint implements HTTPService.
func (s *KubernetesProxyService) GetBindEndpoint() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).getBindEndpoint()
}

// IsSecure returns true if both cert file and key file are present.
func (s *KubernetesProxyService) IsSecure() bool {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).isSecure()
}

// URL returns kubernetes services URL.
//
// It is always HTTPS.
func (s *KubernetesProxyService) URL() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).url(map[string]string{"http": "https"})
}

// MachineAPI is the public API of Omni that helps to establish WireGuard connections.
// This API used to exchange WireGuard keys, assign IP addresses.
// If gRPC tunnel mode is used, WireGuard traffic goes over this endpoint too.
type MachineAPI Service

// URL composes URL for Talos to connect.
//
// If the URL schema is "http", it is replaced with "grpc" to meet the SideroLink API library's protocol requirement.
func (m MachineAPI) URL() string {
	url := wrapService(m.Endpoint, m.AdvertisedURL, m.CertFile, m.KeyFile).url(map[string]string{"http": "grpc"})

	return url
}

func (s *DevServerProxyService) URL() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).url(nil)
}

func (s *DevServerProxyService) GetBindEndpoint() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).getBindEndpoint()
}

func (s *DevServerProxyService) IsSecure() bool {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).isSecure()
}
