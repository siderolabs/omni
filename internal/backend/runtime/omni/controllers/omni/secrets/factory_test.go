// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets_test

import (
	"bytes"
	"context"
	stdx509 "crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/secretrotation"
	"github.com/siderolabs/omni/internal/pkg/siderolink/trustd"
)

type fakeRemoteGeneratorFactory struct {
	omniState state.State
}

type remoteGenerator struct {
	omniState state.State
}

func (r remoteGenerator) IdentityContext(ctx context.Context, csr *x509.CertificateSigningRequest) (ca, crt []byte, err error) {
	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r.omniState, csr.X509CertificateRequest.Subject.CommonName)
	if err != nil {
		return nil, nil, err
	}

	clusterID, _ := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)

	clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, r.omniState, clusterID)
	if err != nil {
		return nil, nil, err
	}

	secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
	if err != nil {
		return nil, nil, err
	}

	issuingCA, acceptedCAs, err := trustd.GetIssuingAndAcceptedCAs(ctx, r.omniState, secretsBundle, clusterSecrets.Metadata().ID())
	if err != nil {
		return nil, nil, err
	}

	csrPemBlock, _ := pem.Decode(csr.X509CertificateRequestPEM)
	if csrPemBlock == nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "failed to decode CSR")
	}

	x509Opts := []x509.Option{
		x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
	}

	signed, err := x509.NewCertificateFromCSRBytes(
		issuingCA.Crt,
		issuingCA.Key,
		csr.X509CertificateRequestPEM,
		x509Opts...,
	)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed to sign CSR: %s", err)
	}

	return bytes.Join(
		xslices.Map(
			acceptedCAs,
			func(cert *x509.PEMEncodedCertificate) []byte {
				return cert.Crt
			},
		),
		nil,
	), signed.X509CertificatePEM, nil
}

func (r remoteGenerator) Close() error {
	return nil
}

func (f *fakeRemoteGeneratorFactory) NewRemoteGenerator(string, []string, []*x509.PEMEncodedCertificate) (secretrotation.RemoteGenerator, error) {
	return remoteGenerator{omniState: f.omniState}, nil
}

type fakeKubernetesClientFactory struct{}

func (f fakeKubernetesClientFactory) NewClient(config *rest.Config) (secretrotation.KubernetesClient, error) {
	// Create a test server that returns ready nodes
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a list of ready nodes for the /api/v1/nodes endpoint
		nodeList := &corev1.NodeList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "NodeList",
				APIVersion: "v1",
			},
			Items: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(w).Encode(nodeList)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	// Configure the REST client to use the test server
	config.Host = server.URL
	config.TLSClientConfig = rest.TLSClientConfig{
		Insecure: true,
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		server.Close()

		return nil, err
	}

	return kubernetesClient{clientSet: clientSet, server: server}, nil
}

type kubernetesClient struct {
	clientSet *kubernetes.Clientset
	server    *httptest.Server
}

func (k kubernetesClient) Clientset() *kubernetes.Clientset {
	return k.clientSet
}

func (k kubernetesClient) Close() {
	if k.server != nil {
		k.server.Close()
	}
}
