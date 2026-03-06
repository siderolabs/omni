// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package certs

import (
	"bytes"
	stdlibx509 "crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"time"

	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
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
func GenerateKubeconfig(clientCert *talosx509.PEMEncodedCertificateAndKey, ca []byte, lbConfig *omni.LoadBalancerConfig) ([]byte, error) {
	// The SiderolinkEndpoint points to the SideroLink WireGuard address of this Omni instance,
	// as it is meant for Talos nodes reaching the load balancer over the WireGuard tunnel.
	// Since this kubeconfig is used internally and the load balancer runs in the same process,
	// rewrite the server to localhost instead.
	//
	// The primary reason for doing this is that leaving the host as-is does not work consistently across platforms.
	// macOS does not route traffic to a local WireGuard interface address back to the host itself, causing Kubernetes proxy connections to hang.
	server, err := localhostEndpoint(lbConfig.TypedSpec().Value.SiderolinkEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error rewriting kubeconfig server to localhost: %w", err)
	}

	contextName := fmt.Sprintf("%s@%s", "admin", lbConfig.Metadata().ID())

	kubeconfig := kubeconfigTemplate{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []kubeconfigCluster{
			{
				Name: lbConfig.Metadata().ID(),
				Cluster: kubeconfigClusterCluster{
					Server:                   server,
					CertificateAuthorityData: base64.StdEncoding.EncodeToString(ca),
				},
			},
		},
		Users: []kubeconfigUser{
			{
				Name: contextName,
				User: kubeconfigUserUser{
					ClientCertificateData: base64.StdEncoding.EncodeToString(clientCert.Crt),
					ClientKeyData:         base64.StdEncoding.EncodeToString(clientCert.Key),
				},
			},
		},
		Contexts: []kubeconfigContext{
			{
				Name: contextName,
				Context: kubeconfigContextContext{
					Cluster:   lbConfig.Metadata().ID(),
					Namespace: "default",
					User:      contextName,
				},
			},
		},
		CurrentContext: contextName,
	}

	out, err := yaml.Marshal(kubeconfig)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// KubernetesAPIClientCertificateFromSecrets generates a Kubernetes API client certificate from the given secrets.
func KubernetesAPIClientCertificateFromSecrets(secrets *omni.ClusterSecrets, certificateValidity time.Duration) (*talosx509.PEMEncodedCertificateAndKey, []byte, error) {
	secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return nil, nil, err
	}

	clientCert, err := NewKubernetesCertificateAndKey(secretsBundle.Certs.K8s, certificateValidity)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating Kubernetes API certificate: %w", err)
	}

	acceptedCAs := []*talosx509.PEMEncodedCertificate{{Crt: secretsBundle.Certs.K8s.Crt}}

	if secrets.TypedSpec().Value.GetExtraCerts().GetK8S() != nil {
		acceptedCAs = append(acceptedCAs, &talosx509.PEMEncodedCertificate{Crt: secrets.TypedSpec().Value.ExtraCerts.K8S.Crt})
	}

	return clientCert, bytes.Join(
		xslices.Map(
			acceptedCAs,
			func(cert *talosx509.PEMEncodedCertificate) []byte {
				return cert.Crt
			},
		),
		nil,
	), nil
}

// NewKubernetesCertificateAndKey generates a Kubernetes client certificate and key signed by the given CA.
func NewKubernetesCertificateAndKey(ca *talosx509.PEMEncodedCertificateAndKey, certificateValidity time.Duration) (*talosx509.PEMEncodedCertificateAndKey, error) {
	k8sCA, err := talosx509.NewCertificateAuthorityFromCertificateAndKey(ca)
	if err != nil {
		return nil, fmt.Errorf("error getting Kubernetes CA: %w", err)
	}

	clientCert, err := talosx509.NewKeyPair(k8sCA,
		talosx509.CommonName(constants.KubernetesAdminCertCommonName),
		talosx509.Organization(talosconstants.KubernetesAdminCertOrganization),
		talosx509.NotBefore(time.Now().Add(-allowedTimeSkew)),
		talosx509.NotAfter(time.Now().Add(certificateValidity)),
		talosx509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		talosx509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("error generating Kubernetes client certificate: %w", err)
	}

	return talosx509.NewCertificateAndKeyFromKeyPair(clientCert), nil
}

// localhostEndpoint rewrites a URL to use localhost while preserving the port.
func localhostEndpoint(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
	}

	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", fmt.Errorf("failed to split host:port %q: %w", u.Host, err)
	}

	u.Host = net.JoinHostPort("localhost", port)

	return u.String(), nil
}
