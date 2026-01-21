// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package certs

import (
	stdlibx509 "crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/siderolabs/crypto/x509"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

type kubeconfigTemplate struct { //nolint:govet
	APIVersion     string              `yaml:"apiVersion"`
	Kind           string              `yaml:"kind"`
	Clusters       []kubeconfigCluster `yaml:"clusters"`
	Users          []kubeconfigUser    `yaml:"users"`
	Contexts       []kubeconfigContext `yaml:"contexts"`
	CurrentContext string              `yaml:"current-context"`
}

type kubeconfigCluster struct {
	Name    string                   `yaml:"name"`
	Cluster kubeconfigClusterCluster `yaml:"cluster"`
}

type kubeconfigClusterCluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

type kubeconfigUser struct {
	Name string             `yaml:"name"`
	User kubeconfigUserUser `yaml:"user"`
}

type kubeconfigUserUser struct {
	ClientCertificateData string `yaml:"client-certificate-data"`
	ClientKeyData         string `yaml:"client-key-data"`
}

type kubeconfigContext struct {
	Name    string                   `yaml:"name"`
	Context kubeconfigContextContext `yaml:"context"`
}

type kubeconfigContextContext struct {
	Cluster   string `yaml:"cluster"`
	Namespace string `yaml:"namespace"`
	User      string `yaml:"user"`
}

const allowedTimeSkew = 10 * time.Second

// GenerateKubeconfig a kubeconfig for the cluster from the given input resources.
func GenerateKubeconfig(secrets *omni.ClusterSecrets, lbConfig *omni.LoadBalancerConfig, certificateValidity time.Duration) ([]byte, error) {
	secretBundle, err := omni.ToSecretsBundle(secrets)
	if err != nil {
		return nil, err
	}

	k8sCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(secretBundle.Certs.K8s)
	if err != nil {
		return nil, fmt.Errorf("error getting Kubernetes CA: %w", err)
	}

	clientCert, err := x509.NewKeyPair(k8sCA,
		x509.CommonName(constants.KubernetesAdminCertCommonName),
		x509.Organization(talosconstants.KubernetesAdminCertOrganization),
		x509.NotBefore(time.Now().Add(-allowedTimeSkew)),
		x509.NotAfter(time.Now().Add(certificateValidity)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("error generating Kubernetes client certificate: %w", err)
	}

	clientCertPEM := x509.NewCertificateAndKeyFromKeyPair(clientCert)
	contextName := fmt.Sprintf("%s@%s", "admin", secrets.Metadata().ID())

	kubeconfig := kubeconfigTemplate{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []kubeconfigCluster{
			{
				Name: secrets.Metadata().ID(),
				Cluster: kubeconfigClusterCluster{
					Server:                   lbConfig.TypedSpec().Value.SiderolinkEndpoint,
					CertificateAuthorityData: base64.StdEncoding.EncodeToString(secretBundle.Certs.K8s.Crt),
				},
			},
		},
		Users: []kubeconfigUser{
			{
				Name: contextName,
				User: kubeconfigUserUser{
					ClientCertificateData: base64.StdEncoding.EncodeToString(clientCertPEM.Crt),
					ClientKeyData:         base64.StdEncoding.EncodeToString(clientCertPEM.Key),
				},
			},
		},
		Contexts: []kubeconfigContext{
			{
				Name: contextName,
				Context: kubeconfigContextContext{
					Cluster:   secrets.Metadata().ID(),
					Namespace: "default",
					User:      contextName,
				},
			},
		},
		CurrentContext: contextName,
	}

	return yaml.Marshal(kubeconfig)
}
