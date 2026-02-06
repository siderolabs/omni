// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secretrotation

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/pkg/certs"
)

func (c Candidate) Validate(ctx context.Context, lbConfig *omni.LoadBalancerConfig, status *omni.ClusterMachineStatus, secrets *omni.ClusterMachineSecrets) (bool, error) {
	switch secrets.TypedSpec().Value.Rotation.Component {
	case specs.SecretRotationSpec_TALOS_CA:
		return c.validateTalosCARotation(ctx, status, secrets)
	case specs.SecretRotationSpec_KUBERNETES_CA:
		return c.validateKubernetesCARotation(ctx, lbConfig, secrets, status)
	case specs.SecretRotationSpec_NONE:
		return false, fmt.Errorf("unexpected rotation component: %s", secrets.TypedSpec().Value.Rotation.Component.String())
	}

	return false, nil
}

func (c Candidate) validateTalosCARotation(ctx context.Context, status *omni.ClusterMachineStatus, cmSecrets *omni.ClusterMachineSecrets) (bool, error) {
	talosClient, err := c.getTalosClient(ctx, status, cmSecrets)
	if err != nil {
		return false, err
	}
	defer talosClient.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = talosClient.Version(ctx)
	if err != nil {
		return false, err
	}

	if c.ControlPlane {
		return c.checkTrustdGeneratedCerts(ctx, talosClient, status, cmSecrets)
	}

	return true, nil
}

func (c Candidate) validateKubernetesCARotation(ctx context.Context, lbConfig *omni.LoadBalancerConfig, secrets *omni.ClusterMachineSecrets, status *omni.ClusterMachineStatus) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	talosClient, err := c.getTalosClient(ctx, status, secrets)
	if err != nil {
		return false, err
	}
	defer talosClient.Close() //nolint:errcheck

	if err = c.checkKubeletApiserverClientCert(ctx, talosClient, secrets); err != nil {
		return false, err
	}

	if c.ControlPlane {
		if err = c.checkApiserverCerts(ctx, talosClient, secrets); err != nil {
			return false, err
		}
	}

	k8sClient, err := c.getKubernetesClient(secrets, lbConfig)
	if err != nil {
		return false, err
	}
	defer k8sClient.Close() //nolint:errcheck

	clientset := k8sClient.Clientset()

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	var notReadyNodes []string

	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == v1.NodeReady {
				if cond.Status != v1.ConditionTrue {
					notReadyNodes = append(notReadyNodes, node.Name)

					break
				}
			}
		}
	}

	if len(notReadyNodes) > 0 {
		return false, fmt.Errorf("nodes not ready: %q", notReadyNodes)
	}

	return true, nil
}

func (c Candidate) getTalosClient(ctx context.Context, status *omni.ClusterMachineStatus, secrets *omni.ClusterMachineSecrets) (*client.Client, error) {
	address := status.TypedSpec().Value.ManagementAddress
	opts := talos.GetSocketOptions(address)

	var endpoints []string

	if opts == nil {
		endpoints = []string{address}
	}

	ca, clientCert, err := c.talosAPIClientCertificateFromSecrets(secrets, constants.CertificateValidityTime, role.MakeSet(role.Admin))
	if err != nil {
		return nil, err
	}

	config := &clientconfig.Config{
		Context: status.Metadata().ID(),
		Contexts: map[string]*clientconfig.Context{
			status.Metadata().ID(): {
				Endpoints: endpoints,
				CA:        base64.StdEncoding.EncodeToString(ca),
				Crt:       base64.StdEncoding.EncodeToString(clientCert.Crt),
				Key:       base64.StdEncoding.EncodeToString(clientCert.Key),
			},
		},
	}

	opts = append(opts, client.WithConfig(config))

	result, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client to machine '%s': %w", status.Metadata().ID(), err)
	}

	return result, nil
}

func (c Candidate) getKubernetesClient(secrets *omni.ClusterMachineSecrets, lbConfig *omni.LoadBalancerConfig) (KubernetesClient, error) {
	clientCerts, ca, err := c.kubernetesAPIClientCertificateFromSecrets(secrets, constants.CertificateValidityTime)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Kubernetes API client certificate: %w", err)
	}

	kubeconfig, err := certs.GenerateKubeconfig(clientCerts, ca, lbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Kubernetes API config: %w", err)
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Kubernetes API config: %w", err)
	}

	result, err := c.KubernetesClientFactory.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return result, nil
}

func (c Candidate) talosAPIClientCertificateFromSecrets(secrets *omni.ClusterMachineSecrets, validity time.Duration, roles role.Set) ([]byte, *talosx509.PEMEncodedCertificateAndKey, error) {
	secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return nil, nil, err
	}

	if secrets.TypedSpec().Value.Rotation.Component != specs.SecretRotationSpec_TALOS_CA {
		clientCert, certErr := talossecrets.NewAdminCertificateAndKey(time.Now(), secretsBundle.Certs.OS, roles, validity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", certErr)
		}

		return secretsBundle.Certs.OS.Crt, clientCert, nil
	}

	switch secrets.TypedSpec().Value.Rotation.Phase {
	case specs.SecretRotationSpec_PRE_ROTATE:
		newCA := &talosx509.PEMEncodedCertificateAndKey{
			Crt: secrets.TypedSpec().Value.Rotation.ExtraCerts.Os.Crt,
			Key: secrets.TypedSpec().Value.Rotation.ExtraCerts.Os.Key,
		}

		clientCert, certErr := talossecrets.NewAdminCertificateAndKey(time.Now(), newCA, roles, validity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", certErr)
		}

		return secretsBundle.Certs.OS.Crt, clientCert, nil

	case specs.SecretRotationSpec_OK, specs.SecretRotationSpec_ROTATE, specs.SecretRotationSpec_POST_ROTATE:
		clientCert, certErr := talossecrets.NewAdminCertificateAndKey(time.Now(), secretsBundle.Certs.OS, roles, validity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Talos API certificate: %w", certErr)
		}

		return secretsBundle.Certs.OS.Crt, clientCert, nil
	}

	return nil, nil, fmt.Errorf("unknown rotation phase: %s", secrets.TypedSpec().Value.Rotation.Phase)
}

func (c Candidate) getTrustdClient(status *omni.ClusterMachineStatus, secrets *omni.ClusterMachineSecrets) (RemoteGenerator, []*talosx509.PEMEncodedCertificate, error) {
	endpoint := status.TypedSpec().Value.ManagementAddress

	secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse secrets bundle: %w", err)
	}

	acceptedCAs := []*talosx509.PEMEncodedCertificate{{Crt: secretsBundle.Certs.OS.Crt}}

	if secrets.TypedSpec().Value.Rotation.ExtraCerts.GetOs() != nil {
		acceptedCAs = append(acceptedCAs, &talosx509.PEMEncodedCertificate{Crt: secrets.TypedSpec().Value.Rotation.ExtraCerts.GetOs().Crt})
	}

	remoteGen, err := c.RemoteGeneratorFactory.NewRemoteGenerator(secretsBundle.TrustdInfo.Token, []string{endpoint}, acceptedCAs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating trustd client: %w", err)
	}

	return remoteGen, acceptedCAs, nil
}

func (c Candidate) checkTrustdGeneratedCerts(ctx context.Context, talosClient *client.Client, status *omni.ClusterMachineStatus, cmSecrets *omni.ClusterMachineSecrets) (bool, error) {
	trustdClient, acceptedCAs, err := c.getTrustdClient(status, cmSecrets)
	if err != nil {
		return false, err
	}
	defer trustdClient.Close() //nolint:errcheck

	certSAN, err := safe.ReaderGetByID[*secrets.CertSAN](ctx, talosClient.COSI, secrets.CertSANAPIID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, fmt.Errorf("certSAN resource not found: %w", err)
		}

		return false, fmt.Errorf("error getting certSANs: %w", err)
	}

	certSANs := certSAN.TypedSpec()

	acceptedCA, err := acceptedCAs[len(acceptedCAs)-1].GetCert()
	if err != nil {
		return false, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	serverCSR, serverCert, err := talosx509.NewCSRAndIdentityFromCA(
		acceptedCA,
		talosx509.IPAddresses(certSANs.StdIPs()),
		talosx509.DNSNames(certSANs.DNSNames),
		talosx509.CommonName(certSANs.FQDN),
	)
	if err != nil {
		return false, fmt.Errorf("failed to generate API server CSR: %w", err)
	}

	var serverCA []byte

	serverCA, serverCert.Crt, err = trustdClient.IdentityContext(ctx, serverCSR)
	if err != nil {
		return false, err
	}

	clientCA, clientCert, err := c.talosAPIClientCertificateFromSecrets(cmSecrets, constants.CertificateValidityTime, role.MakeSet(role.Admin))
	if err != nil {
		return false, err
	}

	verifyErr := c.verifyCert(serverCert.Crt, clientCA, x509.ExtKeyUsageServerAuth)
	if verifyErr != nil {
		return false, fmt.Errorf("trustd: failed to verify server cert: %w", verifyErr)
	}

	verifyErr = c.verifyCert(clientCert.Crt, serverCA, x509.ExtKeyUsageClientAuth)
	if verifyErr != nil {
		return false, fmt.Errorf("trustd: failed to verify client cert: %w", verifyErr)
	}

	return true, nil
}

func (c Candidate) checkKubeletApiserverClientCert(ctx context.Context, talosClient *client.Client, secrets *omni.ClusterMachineSecrets) error {
	kubeletApiserverClient, err := talosClient.Read(ctx, filepath.Join(talosconstants.KubeletPKIDir, "kubelet-client-current.pem"))
	if err != nil {
		return fmt.Errorf("failed to read kubelet-client-current.pem: %w", err)
	}

	cert, err := c.getCertBytes(kubeletApiserverClient)
	if err != nil {
		return fmt.Errorf("failed to read certificate from kubelet-client-current.pem: %w", err)
	}

	acceptedCAs, err := c.kubernetesAcceptedCAsFromSecrets(secrets)
	if err != nil {
		return fmt.Errorf("failed to get accepted CAs: %w", err)
	}

	verifyErr := c.verifyCert(cert, acceptedCAs, x509.ExtKeyUsageClientAuth)
	if verifyErr != nil {
		return fmt.Errorf("kubelet: failed to verify client cert: %w", verifyErr)
	}

	return nil
}

func (c Candidate) checkApiserverCerts(ctx context.Context, talosClient *client.Client, secrets *omni.ClusterMachineSecrets) error {
	apiserverServerCert, err := talosClient.Read(ctx, filepath.Join(talosconstants.KubernetesAPIServerSecretsDir, "apiserver.crt"))
	if err != nil {
		return fmt.Errorf("failed to read apiserver.crt: %w", err)
	}

	serverCert, err := c.getCertBytes(apiserverServerCert)
	if err != nil {
		return fmt.Errorf("failed to read certificate from apiserver.crt: %w", err)
	}

	clientCert, clientCA, err := c.kubernetesAPIClientCertificateFromSecrets(secrets, constants.CertificateValidityTime)
	if err != nil {
		return fmt.Errorf("failed to generate Kubernetes API client certificate: %w", err)
	}

	acceptedCAs, err := c.kubernetesAcceptedCAsFromSecrets(secrets)
	if err != nil {
		return fmt.Errorf("failed to get accepted CAs: %w", err)
	}

	verifyErr := c.verifyCert(serverCert, clientCA, x509.ExtKeyUsageServerAuth)
	if verifyErr != nil {
		return fmt.Errorf("apiserver: failed to verify server cert: %w", verifyErr)
	}

	verifyErr = c.verifyCert(clientCert.Crt, acceptedCAs, x509.ExtKeyUsageClientAuth)
	if verifyErr != nil {
		return fmt.Errorf("apiserver: failed to verify client cert: %w", verifyErr)
	}

	return nil
}

func (c Candidate) verifyCert(certPEM, caPEM []byte, extKeyUsage x509.ExtKeyUsage) error {
	block, _ := pem.Decode(certPEM)

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse cert: %w", err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caPEM); !ok {
		return fmt.Errorf("failed to append CA to pool")
	}

	opts := x509.VerifyOptions{
		Roots:         caPool,
		DNSName:       "",
		Intermediates: x509.NewCertPool(),
		KeyUsages:     []x509.ExtKeyUsage{extKeyUsage},
	}

	_, err = cert.Verify(opts)

	return err
}

func (c Candidate) kubernetesAPIClientCertificateFromSecrets(secrets *omni.ClusterMachineSecrets, certificateValidity time.Duration) (*talosx509.PEMEncodedCertificateAndKey, []byte, error) {
	secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return nil, nil, err
	}

	switch secrets.TypedSpec().Value.Rotation.Phase {
	case specs.SecretRotationSpec_PRE_ROTATE:
		newCA := &talosx509.PEMEncodedCertificateAndKey{
			Crt: secrets.TypedSpec().Value.Rotation.ExtraCerts.K8S.Crt,
			Key: secrets.TypedSpec().Value.Rotation.ExtraCerts.K8S.Key,
		}

		clientCert, certErr := certs.NewKubernetesCertificateAndKey(newCA, certificateValidity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Kubernetes API certificate: %w", certErr)
		}

		return clientCert, secretsBundle.Certs.K8s.Crt, nil

	case specs.SecretRotationSpec_OK, specs.SecretRotationSpec_ROTATE, specs.SecretRotationSpec_POST_ROTATE:
		clientCert, certErr := certs.NewKubernetesCertificateAndKey(secretsBundle.Certs.K8s, certificateValidity)
		if certErr != nil {
			return nil, nil, fmt.Errorf("error generating Kubernetes API certificate: %w", certErr)
		}

		return clientCert, secretsBundle.Certs.K8s.Crt, nil
	}

	return nil, nil, fmt.Errorf("unknown rotation phase: %s", secrets.TypedSpec().Value.Rotation.Phase.String())
}

func (c Candidate) kubernetesAcceptedCAsFromSecrets(secrets *omni.ClusterMachineSecrets) ([]byte, error) {
	secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.GetData())
	if err != nil {
		return nil, err
	}

	acceptedCAs := []*talosx509.PEMEncodedCertificate{{Crt: secretsBundle.Certs.K8s.Crt}}

	if secrets.TypedSpec().Value.GetRotation() != nil && secrets.TypedSpec().Value.Rotation.ExtraCerts.GetK8S() != nil {
		acceptedCAs = append(acceptedCAs, &talosx509.PEMEncodedCertificate{Crt: secrets.TypedSpec().Value.Rotation.ExtraCerts.GetK8S().Crt})
	}

	return bytes.Join(
		xslices.Map(
			acceptedCAs,
			func(cert *talosx509.PEMEncodedCertificate) []byte {
				return cert.Crt
			},
		),
		nil,
	), nil
}

func (c Candidate) getCertBytes(r io.ReadCloser) ([]byte, error) {
	defer r.Close() //nolint:errcheck

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	var certBytes []byte

	rest := data

	for {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type == "CERTIFICATE" {
			// Append the block (re-encoded to PEM format)
			certBytes = append(certBytes, pem.EncodeToMemory(block)...)
		}
	}

	if len(certBytes) == 0 {
		return nil, errors.New("no certificate block found in input")
	}

	return certBytes, nil
}
