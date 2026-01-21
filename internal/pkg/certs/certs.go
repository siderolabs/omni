// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package certs provides utilities for managing/generating certificates.
package certs

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	talosx509 "github.com/siderolabs/crypto/x509"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// IsBase64EncodedCertificateStale checks if the given base64 encoded certificate is stale.
//
// Certificate is considered stale if it's 50% into its validity period.
// As a special case, empty string is considered not stale.
func IsBase64EncodedCertificateStale(certBase64 string, expectedValidity time.Duration) (bool, error) {
	if certBase64 == "" {
		return false, nil
	}

	certPEM, err := base64.StdEncoding.DecodeString(certBase64)
	if err != nil {
		return false, fmt.Errorf("error decoding certificate: %w", err)
	}

	return IsPEMEncodedCertificateStale(certPEM, expectedValidity)
}

// IsPEMEncodedCertificateStale checks if the given PEM-encoded certificate is stale.
//
// Certificate is considered stale if it's 50% into its validity period.
// As a special case, empty string is considered not stale.
func IsPEMEncodedCertificateStale(certPEM []byte, expectedValidity time.Duration) (bool, error) {
	if len(certPEM) == 0 {
		return false, nil
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false, errors.New("error decoding PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("error parsing certificate: %w", err)
	}

	return time.Now().After(cert.NotAfter.Add(-expectedValidity / 2)), nil
}

// TalosAPIClientCertificateFromSecrets generates a Talos API client certificate from the given secrets.
func TalosAPIClientCertificateFromSecrets(secrets *omni.ClusterSecrets, certificateValidity time.Duration, roles role.Set) (*talosx509.PEMEncodedCertificateAndKey, []byte, error) {
	secretBundle, err := omni.ToSecretsBundle(secrets)
	if err != nil {
		return nil, nil, err
	}

	clientCert, err := talossecrets.NewAdminCertificateAndKey(time.Now(), secretBundle.Certs.OS, roles, certificateValidity)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", err)
	}

	return clientCert, secretBundle.Certs.OS.Crt, nil
}
