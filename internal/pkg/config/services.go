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

// GetBindEndpoint implements HTTPService.
func (s service) GetBindEndpoint() string {
	if s.bindEndpoint == nil {
		return ""
	}

	return *s.bindEndpoint
}

// GetCertFile implements HTTPService.
func (s service) GetCertFile() string {
	if s.certFile == nil {
		return ""
	}

	return *s.certFile
}

// GetKeyFile implements HTTPService.
func (s service) GetKeyFile() string {
	if s.keyFile == nil {
		return ""
	}

	return *s.keyFile
}

// IsSecure returns true if both cert file and key file are present.
func (s service) IsSecure() bool {
	return s.certFile != nil && *s.certFile != "" && s.keyFile != nil && *s.keyFile != ""
}

// URL gets the URL from the endpoint.
func (s service) URL() string {
	var advertisedURL string
	if s.advertisedURL != nil {
		advertisedURL = *s.advertisedURL
	}

	if advertisedURL != "" {
		return advertisedURL
	}

	schema := "http"
	if s.IsSecure() {
		schema = "https"
	}

	bindEndpoint := ""
	if s.bindEndpoint != nil {
		bindEndpoint = *s.bindEndpoint
	}

	return fmt.Sprintf("%s://%s", schema, bindEndpoint)
}

// GetBindEndpoint implements HTTPService.
func (s *Service) GetBindEndpoint() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).GetBindEndpoint()
}

// IsSecure returns true if both cert file and key file are present.
func (s *Service) IsSecure() bool {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).IsSecure()
}

// URL gets the URL from the endpoint.
func (s *Service) URL() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).URL()
}

// GetBindEndpoint implements HTTPService.
func (s *KubernetesProxyService) GetBindEndpoint() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).GetBindEndpoint()
}

// IsSecure returns true if both cert file and key file are present.
func (s *KubernetesProxyService) IsSecure() bool {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).IsSecure()
}

// URL returns kubernetes services URL.
// It is always HTTPS.
func (s *KubernetesProxyService) URL() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).URL()
}

// MachineAPI is the public API of Omni that helps to establish WireGuard connections.
// This API used to exchange WireGuard keys, assign IP addresses.
// If gRPC tunnel mode is used, WireGuard traffic goes over this endpoint too.
type MachineAPI Service

// URL composes URL for Talos to connect.
func (m MachineAPI) URL() string {
	return wrapService(m.Endpoint, m.AdvertisedURL, m.CertFile, m.KeyFile).URL()
}

func (s *DevServerProxyService) URL() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).URL()
}

func (s *DevServerProxyService) GetBindEndpoint() string {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).GetBindEndpoint()
}

func (s *DevServerProxyService) IsSecure() bool {
	return wrapService(s.Endpoint, s.AdvertisedURL, s.CertFile, s.KeyFile).IsSecure()
}
