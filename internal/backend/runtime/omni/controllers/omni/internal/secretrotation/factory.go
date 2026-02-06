// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secretrotation

import (
	"context"

	talosx509 "github.com/siderolabs/crypto/x509"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// RemoteGenerator provides a client for accessing trustd.
type RemoteGenerator interface {
	IdentityContext(ctx context.Context, csr *talosx509.CertificateSigningRequest) (ca, crt []byte, err error)
	Close() error
}

// RemoteGeneratorFactory is the factory for providing a client for accessing trustd.
type RemoteGeneratorFactory interface {
	NewRemoteGenerator(token string, endpoints []string, acceptedCAs []*talosx509.PEMEncodedCertificate) (RemoteGenerator, error)
}

type KubernetesClient interface {
	Clientset() *kubernetes.Clientset
	Close()
}

type KubernetesClientFactory interface {
	NewClient(config *rest.Config) (KubernetesClient, error)
}
